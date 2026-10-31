package graphrag

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/core/ports"
	"github.com/onisimchukv/uptimefoundry/internal/rag/embeddings"
)

// GroundedContext represents the structured context provided to AI investigators.
type GroundedContext struct {
	PrimaryResourceID string                          `json:"primary_resource_id"`
	TopologyEntities  []string                        `json:"topology_entities"`
	TopologySummary   string                          `json:"topology_summary"`
	Citations         []string                        `json:"citations"`
	MatchedDocuments  []*models.KnowledgeSearchResult `json:"matched_documents"`
	PromptContext     string                          `json:"prompt_context"`
}

// Engine implements Graph-guided Retrieval-Augmented Generation (GraphRAG).
type Engine struct {
	resourceRepo ports.ResourceRepository
	knowledgeRepo ports.KnowledgeRepository
	embedder     embeddings.Provider
	logger       *slog.Logger
}

// NewEngine creates a new GraphRAG engine instance.
func NewEngine(
	resourceRepo ports.ResourceRepository,
	knowledgeRepo ports.KnowledgeRepository,
	embedder embeddings.Provider,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		resourceRepo: resourceRepo,
		knowledgeRepo: knowledgeRepo,
		embedder:     embedder,
		logger:       logger,
	}
}

// AssembleContext constructs grounded operational knowledge constrained by topology subgraph.
func (e *Engine) AssembleContext(ctx context.Context, primaryResourceID string, queryOrProblem string, hops int) (*GroundedContext, error) {
	if hops <= 0 {
		hops = 2
	}

	entitySet := make(map[string]bool)
	var topologyLines []string

	// 1. Extract Topology Subgraph if primary resource is specified
	if primaryResourceID != "" && e.resourceRepo != nil {
		nodes, rels, err := e.resourceRepo.GetTopologySubgraph(ctx, primaryResourceID, hops)
		if err == nil && len(nodes) > 0 {
			topologyLines = append(topologyLines, fmt.Sprintf("Topology Subgraph (%d nodes, %d relationships, %d hops):", len(nodes), len(rels), hops))
			for _, n := range nodes {
				entitySet[string(n.Kind)] = true
				entitySet[n.Name] = true
				if n.Namespace != "" {
					entitySet[n.Namespace] = true
				}
				topologyLines = append(topologyLines, fmt.Sprintf(" - [%s] %s (id: %s, env: %s)", n.Kind, n.Name, n.ID, n.EnvironmentID))
			}
			for _, r := range rels {
				topologyLines = append(topologyLines, fmt.Sprintf("   * %s --(%s)--> %s", r.SourceResourceID, r.RelationshipType, r.TargetResourceID))
			}
		} else {
			e.logger.Debug("subgraph extraction returned no neighbors or failed", slog.String("resource_id", primaryResourceID), slog.Any("error", err))
		}
	}

	// 2. Extract Entity Tags
	var entityTags []string
	for ent := range entitySet {
		entityTags = append(entityTags, ent)
	}

	// 3. Generate Embedding for Query
	var queryEmbedding []float32
	if e.embedder != nil && queryOrProblem != "" {
		vec, err := e.embedder.Embed(ctx, queryOrProblem)
		if err != nil {
			e.logger.Warn("failed to embed query for graphrag", slog.Any("error", err))
		} else {
			queryEmbedding = vec
		}
	}

	// 4. Retrieve Runbooks with Topological Filter
	var searchResults []*models.KnowledgeSearchResult
	if len(queryEmbedding) > 0 {
		filtered, err := e.knowledgeRepo.SearchWithTopologyFilter(ctx, queryEmbedding, entityTags, 4, 0.60)
		if err == nil && len(filtered) > 0 {
			searchResults = filtered
		} else {
			// Fall back to hybrid search if topological filter returned empty
			hybrid, err2 := e.knowledgeRepo.SearchHybrid(ctx, queryOrProblem, queryEmbedding, 4, 0.50)
			if err2 == nil {
				searchResults = hybrid
			}
		}
	}

	// 5. Build Formatted Citations and Prompt Excerpt
	var citations []string
	var docSections []string

	for idx, res := range searchResults {
		citationRef := fmt.Sprintf("[Runbook: %s]", res.Document.PathOrURL)
		citations = append(citations, citationRef)

		docSections = append(docSections, fmt.Sprintf(
			"### Document %d: %s (Similarity: %.2f)\nCitation: %s\nSource Type: %s\n\n%s\n",
			idx+1,
			res.Document.Title,
			res.Similarity,
			citationRef,
			res.Document.SourceType,
			res.Document.Content,
		))
	}

	var sb strings.Builder
	sb.WriteString("=== TOPOLOGICAL GROUNDING ===\n")
	if len(topologyLines) > 0 {
		sb.WriteString(strings.Join(topologyLines, "\n"))
	} else {
		sb.WriteString("No active topology subgraph extracted for this query.\n")
	}
	sb.WriteString("\n\n=== RELEVANT RUNBOOKS & OPERATIONAL POST-MORTEMS ===\n")
	if len(docSections) > 0 {
		sb.WriteString(strings.Join(docSections, "\n---\n"))
	} else {
		sb.WriteString("No matching runbooks found above similarity threshold.\n")
	}

	return &GroundedContext{
		PrimaryResourceID: primaryResourceID,
		TopologyEntities:  entityTags,
		TopologySummary:   strings.Join(topologyLines, "\n"),
		Citations:         citations,
		MatchedDocuments:  searchResults,
		PromptContext:     sb.String(),
	}, nil
}
