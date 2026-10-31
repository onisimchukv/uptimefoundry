package ai

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/config"
	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/investigation/correlator"
	"github.com/onisimchukv/uptimefoundry/internal/rag/embeddings"
	"github.com/onisimchukv/uptimefoundry/internal/rag/graphrag"
	"github.com/onisimchukv/uptimefoundry/internal/telemetry"
)

type mockResourceRepo struct{}

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
	return []*models.Resource{
		{ID: "res-payment-01", Name: "payment-api", Kind: "k8s_deployment"},
		{ID: "res-db-01", Name: "postgres-core", Kind: "database"},
	}, []*models.Relationship{
		{SourceResourceID: "res-payment-01", TargetResourceID: "res-db-01", RelationshipType: "CONNECTS_TO"},
	}, nil
}
func (m *mockResourceRepo) ExecuteCypher(ctx context.Context, query string) (any, error) {
	return nil, nil
}

type mockIncidentRepo struct {
	inc *models.Incident
}

func (m *mockIncidentRepo) CreateIncident(ctx context.Context, inc *models.Incident) error {
	m.inc = inc
	return nil
}
func (m *mockIncidentRepo) GetIncidentByID(ctx context.Context, id string) (*models.Incident, error) {
	return m.inc, nil
}
func (m *mockIncidentRepo) ListIncidents(ctx context.Context, envID string, status models.IncidentStatus, limit, offset int) ([]*models.Incident, error) {
	return []*models.Incident{m.inc}, nil
}
func (m *mockIncidentRepo) UpdateIncidentStatus(ctx context.Context, id string, status models.IncidentStatus) error {
	if m.inc != nil {
		m.inc.Status = status
	}
	return nil
}

type mockInvestigationRepo struct {
	inv *models.Investigation
}

func (m *mockInvestigationRepo) CreateInvestigation(ctx context.Context, inv *models.Investigation) error {
	inv.ID = "inv-100"
	m.inv = inv
	return nil
}
func (m *mockInvestigationRepo) GetInvestigationByID(ctx context.Context, id string) (*models.Investigation, error) {
	return m.inv, nil
}
func (m *mockInvestigationRepo) SaveInvestigationResults(ctx context.Context, inv *models.Investigation) error {
	m.inv = inv
	return nil
}

type mockRemediationRepo struct {
	proposal *models.Remediation
}

func (m *mockRemediationRepo) CreateProposal(ctx context.Context, rem *models.Remediation) error {
	rem.ID = "rem-200"
	m.proposal = rem
	return nil
}
func (m *mockRemediationRepo) GetProposalByID(ctx context.Context, id string) (*models.Remediation, error) {
	return m.proposal, nil
}
func (m *mockRemediationRepo) ApproveProposal(ctx context.Context, id, approver string) error {
	return nil
}
func (m *mockRemediationRepo) RecordExecution(ctx context.Context, id string, result map[string]any, err error) error {
	return nil
}

type mockKnowledgeRepo struct{}

func (m *mockKnowledgeRepo) IndexDocument(ctx context.Context, doc *models.KnowledgeDocument) error {
	return nil
}
func (m *mockKnowledgeRepo) SearchHybrid(ctx context.Context, query string, embedding []float32, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	return nil, nil
}
func (m *mockKnowledgeRepo) SearchWithTopologyFilter(ctx context.Context, embedding []float32, resourceKinds []string, topK int, similarityThreshold float64) ([]*models.KnowledgeSearchResult, error) {
	return []*models.KnowledgeSearchResult{
		{
			Document: models.KnowledgeDocument{
				ID:        "doc-pay",
				Title:     "Payment DB Runbook > Mitigation",
				PathOrURL: "runbooks/payment.md#mitigation",
				Content:   "Pool restart guide",
			},
			Similarity: 0.91,
		},
	}, nil
}

type mockMetricsProvider struct{}

func (m *mockMetricsProvider) QueryInstant(ctx context.Context, query string, ts time.Time) (*telemetry.MetricResult, error) {
	return &telemetry.MetricResult{
		ResultType: "vector",
		Series: []telemetry.MetricSeries{
			{
				Metric: map[string]string{"app": "payment-api"},
				Points: []telemetry.MetricPoint{{Timestamp: ts, Value: 0.15}},
			},
		},
	}, nil
}
func (m *mockMetricsProvider) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*telemetry.MetricResult, error) {
	return nil, nil
}
func (m *mockMetricsProvider) HealthCheck(ctx context.Context) error { return nil }

type mockLogsProvider struct{}

func (m *mockLogsProvider) QueryLogs(ctx context.Context, query telemetry.LogQuery) (*telemetry.LogResult, error) {
	return &telemetry.LogResult{
		Entries: []telemetry.LogEntry{
			{Timestamp: time.Now(), Line: "pq: connection pool exhausted to postgres-core"},
		},
		Total: 1,
	}, nil
}
func (m *mockLogsProvider) HealthCheck(ctx context.Context) error { return nil }

func TestInvestigator_FullDeterministicInvestigationPipeline(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	embedder := embeddings.NewMockProvider(768)

	resRepo := &mockResourceRepo{}
	knowRepo := &mockKnowledgeRepo{}
	metrics := &mockMetricsProvider{}
	logs := &mockLogsProvider{}
	incRepo := &mockIncidentRepo{
		inc: &models.Incident{
			ID:                "inc-prod-01",
			EnvironmentID:     "env-1",
			Title:             "Elevated 504 on payment checkout",
			Description:       "Payment gateway latency surge",
			Severity:          models.SeverityCritical,
			Status:            models.StatusOpen,
			Source:            "ebpf_anomaly",
			PrimaryResourceID: "res-payment-01",
			DetectedAt:        time.Now().Add(-5 * time.Minute),
		},
	}
	invRepo := &mockInvestigationRepo{}
	remRepo := &mockRemediationRepo{}

	grag := graphrag.NewEngine(resRepo, knowRepo, embedder, logger)
	corr := correlator.NewCorrelator(resRepo, metrics, logs, grag, logger)

	aiCfg := config.AIConfig{
		Enabled: false, // tests the deterministic fallback with citations
	}

	investigator := NewInvestigator(aiCfg, corr, invRepo, remRepo, incRepo, logger)

	result, err := investigator.Investigate(context.Background(), "inc-prod-01")
	if err != nil {
		t.Fatalf("investigation failed: %v", err)
	}

	if result.Status != models.InvestigationCompleted {
		t.Fatalf("expected completed status, got %s", result.Status)
	}

	if len(result.EvidenceBundle) < 3 {
		t.Fatalf("expected at least 3 evidence records (topo, metrics, logs, rag), got %d", len(result.EvidenceBundle))
	}

	if len(result.Hypotheses) == 0 {
		t.Fatal("expected at least 1 hypothesis")
	}

	hypo := result.Hypotheses[0]
	if len(hypo.RunbookCitations) == 0 {
		t.Fatal("expected hypotheses to be grounded with runbook citations")
	}
	if !strings.Contains(hypo.RunbookCitations[0], "runbooks/payment.md#mitigation") {
		t.Fatalf("expected citation runbooks/payment.md#mitigation, got: %v", hypo.RunbookCitations)
	}

	// Verify remediation proposal was created with pending approval
	if remRepo.proposal == nil {
		t.Fatal("expected remediation proposal to be drafted")
	}
	if remRepo.proposal.Status != models.RemediationPendingApproval {
		t.Fatalf("expected pending_approval status, got %s", remRepo.proposal.Status)
	}

	// Verify incident transitioned to investigating
	if incRepo.inc.Status != models.StatusInvestigating {
		t.Fatalf("expected incident status investigating, got %s", incRepo.inc.Status)
	}
}
