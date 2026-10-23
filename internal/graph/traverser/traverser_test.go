package traverser

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/forgeops/forgeops/internal/core/models"
)

type mockGraphEngine struct {
	resources []*models.Resource
	rels      []*models.Relationship
}

func (m *mockGraphEngine) UpsertNode(ctx context.Context, res *models.Resource) error { return nil }
func (m *mockGraphEngine) UpsertEdge(ctx context.Context, rel *models.Relationship) error { return nil }
func (m *mockGraphEngine) GetSubgraph(ctx context.Context, rootID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	return m.resources, m.rels, nil
}
func (m *mockGraphEngine) QueryCypher(ctx context.Context, cypher string) ([]map[string]any, error) {
	return nil, nil
}
func (m *mockGraphEngine) Name() string { return "mock" }

func TestComputeBlastRadius(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mock := &mockGraphEngine{
		resources: []*models.Resource{
			{ID: "node-1", Kind: models.KindK8sNode, Name: "worker-node-1"},
			{ID: "pod-1", Kind: models.KindK8sPod, Name: "billing-pod-1"},
			{ID: "pod-2", Kind: models.KindK8sPod, Name: "billing-pod-2"},
			{ID: "svc-1", Kind: models.KindK8sService, Name: "billing-service"},
			{ID: "deploy-1", Kind: models.KindK8sDeployment, Name: "billing-deployment"},
		},
		rels: []*models.Relationship{
			{SourceResourceID: "pod-1", TargetResourceID: "node-1", RelationshipType: models.RelRunsOn},
			{SourceResourceID: "pod-2", TargetResourceID: "node-1", RelationshipType: models.RelRunsOn},
			{SourceResourceID: "svc-1", TargetResourceID: "pod-1", RelationshipType: models.RelConnectsTo},
		},
	}

	traverser := New(mock, logger)
	report, err := traverser.ComputeBlastRadius(context.Background(), "node-1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalImpacted != 5 {
		t.Fatalf("expected 5 total impacted resources, got %d", report.TotalImpacted)
	}
	if len(report.ImpactedServices) != 1 || report.ImpactedServices[0] != "billing-service" {
		t.Fatalf("expected 1 impacted service (billing-service), got %v", report.ImpactedServices)
	}
	if len(report.ImpactedWorkloads) != 1 || report.ImpactedWorkloads[0] != "billing-deployment" {
		t.Fatalf("expected 1 impacted workload (billing-deployment), got %v", report.ImpactedWorkloads)
	}
	// Expected score: 2 pods (4) + 1 svc (15) + 1 deploy (10) = 29
	if report.SeverityScore != 29.0 {
		t.Fatalf("expected severity score 29.0, got %f", report.SeverityScore)
	}
}
