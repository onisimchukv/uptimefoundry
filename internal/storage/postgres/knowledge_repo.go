package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// KnowledgeRepo provides persistence and vector search for operational documentation in PostgreSQL.
type KnowledgeRepo struct {
	db     *DB
	logger *slog.Logger
}

// NewKnowledgeRepo creates a new knowledge repository instance.
func NewKnowledgeRepo(db *DB, logger *slog.Logger) *KnowledgeRepo {
	return &KnowledgeRepo{
		db:     db,
		logger: logger,
	}
}

// IndexDocument saves or replaces an operational knowledge document chunk with its dense embedding.
func (r *KnowledgeRepo) IndexDocument(ctx context.Context, doc *models.KnowledgeDocument) error {
	metaJSON, err := json.Marshal(doc.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	var vecVal any
	if len(doc.Embedding) > 0 {
		vecVal = pgvector.NewVector(doc.Embedding)
	}

	query := `
		INSERT INTO knowledge_documents (
			source_type, title, path_or_url, content, metadata, embedding, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		)
		RETURNING id
	`

	err = r.db.Pool.QueryRow(ctx, query,
		string(doc.SourceType),
		doc.Title,
		doc.PathOrURL,
		doc.Content,
		metaJSON,
		vecVal,
		doc.CreatedAt,
		doc.UpdatedAt,
	).Scan(&doc.ID)

	if err != nil {
		return fmt.Errorf("insert knowledge document: %w", err)
	}

	return nil
}

// SearchHybrid executes hybrid retrieval combining pgvector cosine similarity and full-text search.
func (r *KnowledgeRepo) SearchHybrid(ctx context.Context, query string, embedding []float32, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	if len(embedding) > 0 && r.db.HasVector {
		vec := pgvector.NewVector(embedding)
		sql := `
			SELECT 
				id, source_type, title, path_or_url, content, metadata, created_at, updated_at,
				(1 - (embedding <=> $1)) AS similarity,
				ts_rank_cd(to_tsvector('english', title || ' ' || content), plainto_tsquery('english', $2)) AS rank_bm25
			FROM knowledge_documents
			WHERE embedding IS NOT NULL
			  AND (1 - (embedding <=> $1)) >= $3
			ORDER BY ((0.7 * (1 - (embedding <=> $1))) + (0.3 * ts_rank_cd(to_tsvector('english', title || ' ' || content), plainto_tsquery('english', $2)))) DESC
			LIMIT $4;
		`

		rows, err := r.db.Pool.Query(ctx, sql, vec, query, similarityThreshold, topK)
		if err != nil {
			return nil, fmt.Errorf("hybrid vector search failed: %w", err)
		}
		defer rows.Close()

		return r.scanSearchResults(rows)
	}

	// Lexical fallback when embeddings or pgvector extension is absent
	fallbackSQL := `
		SELECT 
			id, source_type, title, path_or_url, content, metadata, created_at, updated_at,
			0.5 AS similarity,
			ts_rank_cd(to_tsvector('english', title || ' ' || content), plainto_tsquery('english', $1)) AS rank_bm25
		FROM knowledge_documents
		WHERE to_tsvector('english', title || ' ' || content) @@ plainto_tsquery('english', $1)
		ORDER BY rank_bm25 DESC
		LIMIT $2;
	`
	rows, err := r.db.Pool.Query(ctx, fallbackSQL, query, topK)
	if err != nil {
		return nil, fmt.Errorf("lexical fallback search failed: %w", err)
	}
	defer rows.Close()

	return r.scanSearchResults(rows)
}

// SearchWithTopologyFilter performs vector retrieval pre-filtered by entity kinds and tags.
func (r *KnowledgeRepo) SearchWithTopologyFilter(ctx context.Context, embedding []float32, resourceKinds []string, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	if len(embedding) == 0 || !r.db.HasVector {
		return nil, fmt.Errorf("vector embedding or pgvector extension required for topology search")
	}

	vec := pgvector.NewVector(embedding)

	// If no resourceKinds specified, standard vector cosine search
	if len(resourceKinds) == 0 {
		sql := `
			SELECT 
				id, source_type, title, path_or_url, content, metadata, created_at, updated_at,
				(1 - (embedding <=> $1)) AS similarity,
				0.0 AS rank_bm25
			FROM knowledge_documents
			WHERE embedding IS NOT NULL
			  AND (1 - (embedding <=> $1)) >= $2
			ORDER BY similarity DESC
			LIMIT $3;
		`
		rows, err := r.db.Pool.Query(ctx, sql, vec, similarityThreshold, topK)
		if err != nil {
			return nil, fmt.Errorf("vector topology search: %w", err)
		}
		defer rows.Close()
		return r.scanSearchResults(rows)
	}

	// Filter where metadata's 'kinds' array overlaps with resourceKinds
	sql := `
		SELECT 
			id, source_type, title, path_or_url, content, metadata, created_at, updated_at,
			(1 - (embedding <=> $1)) AS similarity,
			0.0 AS rank_bm25
		FROM knowledge_documents
		WHERE embedding IS NOT NULL
		  AND (1 - (embedding <=> $1)) >= $2
		  AND (
		      metadata->'kinds' ?| $3
		      OR metadata->'services' ?| $3
		      OR metadata->'target_kinds' ?| $3
		  )
		ORDER BY similarity DESC
		LIMIT $4;
	`

	rows, err := r.db.Pool.Query(ctx, sql, vec, similarityThreshold, resourceKinds, topK)
	if err != nil {
		return nil, fmt.Errorf("topology filtered vector search failed: %w", err)
	}
	defer rows.Close()

	return r.scanSearchResults(rows)
}

func (r *KnowledgeRepo) scanSearchResults(rows pgx.Rows) ([]*models.KnowledgeSearchResult, error) {
	var results []*models.KnowledgeSearchResult

	for rows.Next() {
		var doc models.KnowledgeDocument
		var srcType string
		var metaRaw []byte
		var similarity float64
		var bm25Rank float64

		err := rows.Scan(
			&doc.ID,
			&srcType,
			&doc.Title,
			&doc.PathOrURL,
			&doc.Content,
			&metaRaw,
			&doc.CreatedAt,
			&doc.UpdatedAt,
			&similarity,
			&bm25Rank,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result row: %w", err)
		}

		doc.SourceType = models.KnowledgeSourceType(srcType)
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &doc.Metadata)
		}

		results = append(results, &models.KnowledgeSearchResult{
			Document:   doc,
			Similarity: similarity,
			ScoreBM25:  bm25Rank,
		})
	}

	return results, rows.Err()
}
