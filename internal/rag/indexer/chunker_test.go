package indexer

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/rag/embeddings"
)

type mockKnowledgeRepo struct {
	docs []*models.KnowledgeDocument
}

func (m *mockKnowledgeRepo) IndexDocument(ctx context.Context, doc *models.KnowledgeDocument) error {
	m.docs = append(m.docs, doc)
	return nil
}

func (m *mockKnowledgeRepo) SearchHybrid(ctx context.Context, query string, embedding []float32, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	return nil, nil
}

func (m *mockKnowledgeRepo) SearchWithTopologyFilter(ctx context.Context, embedding []float32, resourceKinds []string, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	return nil, nil
}

const sampleRunbook = `---
title: Payment API Database Connection Timeout Runbook
services:
  - payment-api
  - postgres-core
kinds:
  - k8s_deployment
  - database
---

# Payment API Database Connection Timeout

## Symptoms
- HTTP 504 Gateway Timeout errors spike on '/api/v1/checkout'.
- Pod logs display 'pq: connection pool exhausted'.
- TCP retransmissions between payment-api pods and postgresql host.

## Immediate Mitigation
1. Check VictoriaMetrics pool utilization: 'sum(pg_stat_activity_count) by (state)'
2. Scale connection pooler replica count if queue is saturated.
3. Restart stuck payment-api pods using safe rolling restart.

## Root Cause Analysis
Inspect postgres pg_locks table to verify if an unindexed table scan is holding an exclusive row lock.
`

func TestMarkdownChunker_ParseSectionsAndFrontmatter(t *testing.T) {
	chunker := NewMarkdownChunker(1000, 100)
	docs, err := chunker.Parse("runbooks/payments/db_timeout.md", models.SourceRunbook, sampleRunbook)
	if err != nil {
		t.Fatalf("unexpected chunker error: %v", err)
	}

	if len(docs) != 3 {
		t.Fatalf("expected 3 section chunks (Symptoms, Mitigation, RCA), got %d", len(docs))
	}

	// Verify frontmatter propagation
	for _, doc := range docs {
		if doc.SourceType != models.SourceRunbook {
			t.Errorf("expected SourceRunbook, got %v", doc.SourceType)
		}
		kinds, ok := doc.Metadata["kinds"].([]any)
		if !ok || len(kinds) != 2 {
			t.Errorf("expected kinds frontmatter metadata to be extracted")
		}
		if !strings.Contains(doc.PathOrURL, "runbooks/payments/db_timeout.md#") {
			t.Errorf("expected anchor in path_or_url: %s", doc.PathOrURL)
		}
	}

	// Verify section titles
	if !strings.Contains(docs[0].Title, "Symptoms") {
		t.Errorf("expected Symptoms section title, got: %s", docs[0].Title)
	}
	if !strings.Contains(docs[1].Title, "Immediate Mitigation") {
		t.Errorf("expected Immediate Mitigation section title, got: %s", docs[1].Title)
	}
}

func TestIndexer_IndexPipeline(t *testing.T) {
	// Create a temporary runbook file
	tmpDir := t.TempDir()
	filePath := tmpDir + "/payment_timeout.md"
	if err := os.WriteFile(filePath, []byte(sampleRunbook), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	mockRepo := &mockKnowledgeRepo{}
	embedder := embeddings.NewMockProvider(768)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	idx := NewIndexer(embedder, mockRepo, logger)
	count, err := idx.IndexDirectory(context.Background(), tmpDir, models.SourceRunbook)
	if err != nil {
		t.Fatalf("indexer failed: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 chunks indexed, got %d", count)
	}
	if len(mockRepo.docs) != 3 {
		t.Fatalf("expected 3 docs in repo, got %d", len(mockRepo.docs))
	}
	for _, doc := range mockRepo.docs {
		if len(doc.Embedding) != 768 {
			t.Fatalf("expected 768-dim embedding, got %d", len(doc.Embedding))
		}
	}
}
