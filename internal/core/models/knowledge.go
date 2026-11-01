// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// KnowledgeSourceType defines the origin and category of operational documentation.
type KnowledgeSourceType string

const (
	SourceRunbook        KnowledgeSourceType = "runbook"
	SourcePostMortem     KnowledgeSourceType = "postmortem"
	SourceAlertDoc       KnowledgeSourceType = "alert_doc"
	SourceGitOpsManifest KnowledgeSourceType = "gitops_manifest"
)

// KnowledgeDocument represents an operational document indexed into the pgvector store.
type KnowledgeDocument struct {
	ID         string              `json:"id"`
	SourceType KnowledgeSourceType `json:"source_type"`
	Title      string              `json:"title"`
	PathOrURL  string              `json:"path_or_url"`
	Content    string              `json:"content"`
	Metadata   map[string]any      `json:"metadata"` // contains targets like kinds, services, namespaces
	Embedding  []float32           `json:"embedding,omitempty"`
	CreatedAt  time.Time           `json:"created_at"`
	UpdatedAt  time.Time           `json:"updated_at"`
}

// KnowledgeSearchResult represents a match returned from hybrid/vector retrieval.
type KnowledgeSearchResult struct {
	Document   KnowledgeDocument `json:"document"`
	Similarity float64           `json:"similarity"` // 0.0 to 1.0 cosine similarity
	ScoreBM25  float64           `json:"score_bm25,omitempty"`
}
