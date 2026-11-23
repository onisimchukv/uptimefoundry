// SPDX-License-Identifier: MIT

package correlator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
	"github.com/onisimchukv/uptimefoundry/internal/telemetry"
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

type mockMetricsProvider struct {
	val float64
}

func (m *mockMetricsProvider) QueryInstant(ctx context.Context, query string, t time.Time) (*telemetry.MetricResult, error) {
	return &telemetry.MetricResult{
		Series: []telemetry.MetricSeries{
			{
				Metric: map[string]string{"app": "payment-api"},
				Points: []telemetry.MetricPoint{
					{Timestamp: t, Value: m.val},
				},
			},
		},
	}, nil
}

func (m *mockMetricsProvider) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*telemetry.MetricResult, error) {
	return &telemetry.MetricResult{}, nil
}

func (m *mockMetricsProvider) HealthCheck(ctx context.Context) error {
	return nil
}

type mockLogsProvider struct {
	entries []telemetry.LogEntry
}

func (m *mockLogsProvider) QueryLogs(ctx context.Context, q telemetry.LogQuery) (*telemetry.LogResult, error) {
	return &telemetry.LogResult{
		Entries: m.entries,
	}, nil
}

func (m *mockLogsProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func TestCorrelator_CollectEvidence_FullSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	nodes := []*models.Resource{
		{ID: "res-01", Name: "payment-api", Kind: "pod"},
		{ID: "res-02", Name: "postgres-db", Kind: "database"},
	}
	rels := []*models.Relationship{
		{ID: "rel-01", SourceResourceID: "res-01", TargetResourceID: "res-02", RelationshipType: "depends_on"},
	}

	resRepo := &mockResourceRepo{nodes: nodes, rels: rels}
	metricsProv := &mockMetricsProvider{val: 0.25} // > 0.05 anomaly
	logsProv := &mockLogsProvider{
		entries: []telemetry.LogEntry{
			{Timestamp: time.Now(), Line: "ERROR: db connection timeout"},
			{Timestamp: time.Now(), Line: "FATAL: transaction aborted"},
		},
	}

	c := NewCorrelator(resRepo, metricsProv, logsProv, nil, logger)

	inc := &models.Incident{
		ID:                "inc-001",
		Title:             "Payment API High 5xx Rates",
		Description:       "Spike in payment transaction errors",
		PrimaryResourceID: "res-01",
		DetectedAt:        time.Now().Add(-10 * time.Minute),
	}

	bundle, err := c.CollectEvidence(context.Background(), inc)
	if err != nil {
		t.Fatalf("unexpected error collecting evidence: %v", err)
	}

	if len(bundle.TopologyNodes) != 2 {
		t.Errorf("expected 2 topology nodes, got %d", len(bundle.TopologyNodes))
	}
	if len(bundle.TopologyRels) != 1 {
		t.Errorf("expected 1 topology rel, got %d", len(bundle.TopologyRels))
	}

	// Verify evidence records
	evidenceTypes := make(map[string]bool)
	for _, ev := range bundle.Evidence {
		evidenceTypes[ev.EvidenceType] = true
		if ev.EvidenceType == "metric_timeseries" && !ev.IsAnomaly {
			t.Errorf("expected metric timeseries with val=0.25 to be flagged as anomaly")
		}
		if ev.EvidenceType == "log_stream" && !ev.IsAnomaly {
			t.Errorf("expected log stream to be flagged as anomaly")
		}
	}

	if !evidenceTypes["topology_subgraph"] {
		t.Error("missing topology_subgraph evidence record")
	}
	if !evidenceTypes["metric_timeseries"] {
		t.Error("missing metric_timeseries evidence record")
	}
	if !evidenceTypes["log_stream"] {
		t.Error("missing log_stream evidence record")
	}
}

func TestCorrelator_CollectEvidence_GracefulDegradation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// All providers nil
	c := NewCorrelator(nil, nil, nil, nil, logger)

	inc := &models.Incident{
		ID:         "inc-nil",
		Title:      "Generic Incident",
		DetectedAt: time.Now(),
	}

	bundle, err := c.CollectEvidence(context.Background(), inc)
	if err != nil {
		t.Fatalf("expected no error with nil providers, got: %v", err)
	}
	if bundle == nil {
		t.Fatal("expected non-nil bundle")
	}
	if len(bundle.Evidence) != 0 {
		t.Errorf("expected 0 evidence records with nil providers, got %d", len(bundle.Evidence))
	}
}
