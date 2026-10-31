package graphrag

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/rag/embeddings"
)

type mockResourceRepo struct {
	nodes []*models.Resource
	rels  []*models.Relationship
}

func (m *mockResourceRepo) UpsertResource(ctx context.Context, res *models.Resource) error { return nil }
func (m *mockResourceRepo) GetResourceByID(ctx context.Context, id string) (*models.Resource, error) {
	return nil, nil
}
func (m *mockResourceRepo) ListResources(ctx context.Context, envID string, kind models.ResourceKind, limit, offset int) ([]*models.Resource, error) {
	return nil, nil
}
func (m *mockResourceRepo) UpsertRelationship(ctx context.Context, rel *models.Relationship) error {
	return nil
}
func (m *mockResourceRepo) GetRelationships(ctx context.Context, resourceID string) ([]*models.Relationship, error) {
	return nil, nil
}
func (m *mockResourceRepo) GetTopologySubgraph(ctx context.Context, rootResourceID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	return m.nodes, m.rels, nil
}
func (m *mockResourceRepo) ExecuteCypher(ctx context.Context, query string) (any, error) {
	return nil, nil
}

type mockKnowledgeRepo struct {
	topologyResults []*models.KnowledgeSearchResult
	hybridResults   []*models.KnowledgeSearchResult
}

func (m *mockKnowledgeRepo) IndexDocument(ctx context.Context, doc *models.KnowledgeDocument) error {
	return nil
}

func (m *mockKnowledgeRepo) SearchHybrid(ctx context.Context, query string, embedding []float32, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	return m.hybridResults, nil
}

func (m *mockKnowledgeRepo) SearchWithTopologyFilter(ctx context.Context, embedding []float32, resourceKinds []string, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	return m.topologyResults, nil
}

func TestGraphRAG_AssembleContextWithTopologyAndCitations(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	embedder := embeddings.NewMockProvider(768)

	resRepo := &mockResourceRepo{
		nodes: []*models.Resource{
			{ID: "res-payment-01", Name: "payment-api", Kind: "k8s_deployment", Namespace: "production"},
			{ID: "res-pg-01", Name: "postgres-core", Kind: "database", Namespace: "production"},
		},
		rels: []*models.Relationship{
			{SourceResourceID: "res-payment-01", TargetResourceID: "res-pg-01", RelationshipType: "CONNECTS_TO"},
		},
	}

	docResult := &models.KnowledgeSearchResult{
		Document: models.KnowledgeDocument{
			ID:         "doc-01",
			SourceType: models.SourceRunbook,
			Title:      "Payment Database Runbook > Mitigation",
			PathOrURL:  "runbooks/payments/db.md#mitigation",
			Content:    "Restart payment pods or scale pooler.",
		},
		Similarity: 0.89,
	}

	knowRepo := &mockKnowledgeRepo{
		topologyResults: []*models.KnowledgeSearchResult{docResult},
	}

	engine := NewEngine(resRepo, knowRepo, embedder, logger)

	grounded, err := engine.AssembleContext(context.Background(), "res-payment-01", "HTTP 504 on payment checkout", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(grounded.TopologyEntities) == 0 {
		t.Fatal("expected topology entities to be extracted")
	}

	if len(grounded.Citations) != 1 {
		t.Fatalf("expected 1 citation, got %d", len(grounded.Citations))
	}

	expectedCitation := "[Runbook: runbooks/payments/db.md#mitigation]"
	if grounded.Citations[0] != expectedCitation {
		t.Fatalf("expected citation %s, got %s", expectedCitation, grounded.Citations[0])
	}

	if !strings.Contains(grounded.PromptContext, "=== TOPOLOGICAL GROUNDING ===") {
		t.Fatal("missing topological grounding section in prompt context")
	}
	if !strings.Contains(grounded.PromptContext, "CONNECTS_TO") {
		t.Fatal("missing relationship in prompt context")
	}
}
