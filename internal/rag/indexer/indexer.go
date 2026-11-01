// SPDX-License-Identifier: MIT

package indexer

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/core/ports"
	"github.com/onisimchukv/uptimefoundry/internal/rag/embeddings"
)

// Indexer coordinates document ingestion, chunking, embedding, and storage.
type Indexer struct {
	chunker  *MarkdownChunker
	embedder embeddings.Provider
	repo     ports.KnowledgeRepository
	logger   *slog.Logger
}

// NewIndexer creates a new document indexer.
func NewIndexer(embedder embeddings.Provider, repo ports.KnowledgeRepository, logger *slog.Logger) *Indexer {
	return &Indexer{
		chunker:  NewMarkdownChunker(1500, 200),
		embedder: embedder,
		repo:     repo,
		logger:   logger,
	}
}

// IndexFile parses a single markdown document, generates embeddings, and indexes chunks.
func (idx *Indexer) IndexFile(ctx context.Context, filePath string, sourceType models.KnowledgeSourceType) (int, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("read file %q: %w", filePath, err)
	}

	chunks, err := idx.chunker.Parse(filePath, sourceType, string(data))
	if err != nil {
		return 0, fmt.Errorf("chunk markdown %q: %w", filePath, err)
	}

	count := 0
	for _, chunk := range chunks {
		vec, err := idx.embedder.Embed(ctx, chunk.Content)
		if err != nil {
			idx.logger.Warn("failed to embed chunk", slog.String("title", chunk.Title), slog.Any("error", err))
			continue
		}
		chunk.Embedding = vec

		if err := idx.repo.IndexDocument(ctx, chunk); err != nil {
			return count, fmt.Errorf("index chunk %q: %w", chunk.Title, err)
		}
		count++
	}

	idx.logger.Info("indexed operational document",
		slog.String("path", filePath),
		slog.String("source_type", string(sourceType)),
		slog.Int("chunks_indexed", count),
	)

	return count, nil
}

// IndexDirectory traverses a directory, finding all markdown files (.md, .markdown) and indexing them.
func (idx *Indexer) IndexDirectory(ctx context.Context, dirPath string, sourceType models.KnowledgeSourceType) (int, error) {
	totalIndexed := 0

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".md" || ext == ".markdown" {
			count, err := idx.IndexFile(ctx, path, sourceType)
			if err != nil {
				idx.logger.Error("failed indexing document file", slog.String("path", path), slog.Any("error", err))
			} else {
				totalIndexed += count
			}
		}
		return nil
	})

	if err != nil {
		return totalIndexed, fmt.Errorf("walk directory %q: %w", dirPath, err)
	}

	return totalIndexed, nil
}
