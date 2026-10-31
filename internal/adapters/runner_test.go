package adapters

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

type mockAdapter struct {
	name   string
	result *DiscoveryResult
	err    error
}

func (m *mockAdapter) Name() string { return m.name }
func (m *mockAdapter) Discover(ctx context.Context, envID string) (*DiscoveryResult, error) {
	return m.result, m.err
}

type mockResourceRepo struct {
	upsertedResources     []*models.Resource
	upsertedRelationships []*models.Relationship
}

func (m *mockResourceRepo) UpsertResource(ctx context.Context, res *models.Resource) error {
	m.upsertedResources = append(m.upsertedResources, res)
	return nil
}
func (m *mockResourceRepo) GetResourceByID(ctx context.Context, id string) (*models.Resource, error) {
	return nil, nil
}
func (m *mockResourceRepo) ListResources(ctx context.Context, envID string, kind models.ResourceKind, limit, offset int) ([]*models.Resource, error) {
	return m.upsertedResources, nil
}
func (m *mockResourceRepo) UpsertRelationship(ctx context.Context, rel *models.Relationship) error {
	m.upsertedRelationships = append(m.upsertedRelationships, rel)
	return nil
}
func (m *mockResourceRepo) GetRelationships(ctx context.Context, resourceID string) ([]*models.Relationship, error) {
	return m.upsertedRelationships, nil
}
func (m *mockResourceRepo) GetTopologySubgraph(ctx context.Context, rootResourceID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	return nil, nil, nil
}
func (m *mockResourceRepo) ExecuteCypher(ctx context.Context, query string) (any, error) {
	return nil, nil
}

type mockDiscoveryRepo struct {
	createdRuns []*models.DiscoveryRun
	updatedRuns []*models.DiscoveryRun
}

func (m *mockDiscoveryRepo) CreateRun(ctx context.Context, run *models.DiscoveryRun) error {
	m.createdRuns = append(m.createdRuns, run)
	return nil
}
func (m *mockDiscoveryRepo) UpdateRun(ctx context.Context, run *models.DiscoveryRun) error {
	m.updatedRuns = append(m.updatedRuns, run)
	return nil
}
func (m *mockDiscoveryRepo) GetRunByID(ctx context.Context, id string) (*models.DiscoveryRun, error) {
	return nil, nil
}
func (m *mockDiscoveryRepo) ListRuns(ctx context.Context, envID string, limit, offset int) ([]*models.DiscoveryRun, error) {
	return m.createdRuns, nil
}

func TestRunnerExecution(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	mockAdapter1 := &mockAdapter{
		name: "test_adapter",
		result: &DiscoveryResult{
			Resources: []*models.Resource{
				{ID: "res-1", Name: "server-01", Kind: models.KindHost},
				{ID: "res-2", Name: "eth0", Kind: models.KindNetworkInterface},
			},
			Relationships: []*models.Relationship{
				{SourceResourceID: "res-1", TargetResourceID: "res-2", RelationshipType: models.RelContains},
			},
		},
	}

	resRepo := &mockResourceRepo{}
	discRepo := &mockDiscoveryRepo{}

	runner := NewRunner([]DiscoveryAdapter{mockAdapter1}, resRepo, discRepo, logger)
	run, err := runner.RunAll(context.Background(), "env-prod")
	if err != nil {
		t.Fatalf("unexpected runner error: %v", err)
	}

	if run.Status != models.DiscoveryStatusCompleted {
		t.Fatalf("expected run status completed, got %s", run.Status)
	}
	if run.ResourcesDiscovered != 2 {
		t.Fatalf("expected 2 discovered resources, got %d", run.ResourcesDiscovered)
	}
	if run.RelationshipsDiscovered != 1 {
		t.Fatalf("expected 1 discovered relationship, got %d", run.RelationshipsDiscovered)
	}

	if len(resRepo.upsertedResources) != 2 {
		t.Fatalf("expected 2 resources persisted, got %d", len(resRepo.upsertedResources))
	}
	if len(resRepo.upsertedRelationships) != 1 {
		t.Fatalf("expected 1 relationship persisted, got %d", len(resRepo.upsertedRelationships))
	}
}
