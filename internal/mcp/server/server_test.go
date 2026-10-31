package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/mcp/protocol"
	"github.com/forgeops/forgeops/internal/telemetry"
)

type mockResourceRepo struct{}

func (m *mockResourceRepo) UpsertResource(ctx context.Context, res *models.Resource) error { return nil }
func (m *mockResourceRepo) GetResourceByID(ctx context.Context, id string) (*models.Resource, error) {
	return nil, nil
}
func (m *mockResourceRepo) ListResources(ctx context.Context, envID string, kind models.ResourceKind, limit, offset int) ([]*models.Resource, error) {
	return []*models.Resource{{ID: "res-1", Name: "payment-api"}}, nil
}
func (m *mockResourceRepo) UpsertRelationship(ctx context.Context, rel *models.Relationship) error {
	return nil
}
func (m *mockResourceRepo) GetRelationships(ctx context.Context, resourceID string) ([]*models.Relationship, error) {
	return nil, nil
}
func (m *mockResourceRepo) GetTopologySubgraph(ctx context.Context, rootResourceID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	return []*models.Resource{{ID: "node-1", Name: "srv-prod-1"}}, nil, nil
}
func (m *mockResourceRepo) ExecuteCypher(ctx context.Context, query string) (any, error) {
	return nil, nil
}

type mockRemediationRepo struct {
	created *models.Remediation
}

func (m *mockRemediationRepo) CreateProposal(ctx context.Context, rem *models.Remediation) error {
	rem.ID = "rem-uuid-1234"
	m.created = rem
	return nil
}
func (m *mockRemediationRepo) GetProposalByID(ctx context.Context, id string) (*models.Remediation, error) {
	return m.created, nil
}
func (m *mockRemediationRepo) ApproveProposal(ctx context.Context, id, approver string) error {
	return nil
}
func (m *mockRemediationRepo) RecordExecution(ctx context.Context, id string, result map[string]any, err error) error {
	return nil
}

type mockMetricsProvider struct{}

func (m *mockMetricsProvider) QueryInstant(ctx context.Context, query string, ts time.Time) (*telemetry.MetricResult, error) {
	return &telemetry.MetricResult{
		ResultType: "vector",
		Series: []telemetry.MetricSeries{
			{
				Metric: map[string]string{"app": "payment-api"},
				Points: []telemetry.MetricPoint{{Timestamp: ts, Value: 99.4}},
			},
		},
	}, nil
}
func (m *mockMetricsProvider) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*telemetry.MetricResult, error) {
	return nil, nil
}
func (m *mockMetricsProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func TestMCPServer_InitializeAndListTools(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	srv := NewServer(Config{}, &mockResourceRepo{}, nil, nil, nil, &mockMetricsProvider{}, nil, nil, logger)

	// Test Initialize
	initReq := protocol.Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
	}
	resp := srv.HandleRequest(context.Background(), initReq)
	if resp.Error != nil {
		t.Fatalf("unexpected error on initialize: %v", resp.Error)
	}

	// Test tools/list
	toolsReq := protocol.Request{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}
	toolsResp := srv.HandleRequest(context.Background(), toolsReq)
	if toolsResp.Error != nil {
		t.Fatalf("unexpected error on tools/list: %v", toolsResp.Error)
	}
	toolsList, ok := toolsResp.Result.(protocol.ToolListResult)
	if !ok || len(toolsList.Tools) != 6 {
		t.Fatalf("expected 6 tools, got %d", len(toolsList.Tools))
	}
}

func TestMCPServer_CallTool_MetricsAndTopology(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	srv := NewServer(Config{}, &mockResourceRepo{}, nil, nil, nil, &mockMetricsProvider{}, nil, nil, logger)

	// Call forgeops_query_metrics
	metricArgs, _ := json.Marshal(protocol.CallToolParams{
		Name:      "forgeops_query_metrics",
		Arguments: map[string]any{"query": "up{job=\"payment\"}"},
	})
	callReq := protocol.Request{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params:  metricArgs,
	}
	resp := srv.HandleRequest(context.Background(), callReq)
	callRes, ok := resp.Result.(protocol.CallToolResult)
	if !ok || callRes.IsError || len(callRes.Content) == 0 {
		t.Fatalf("expected successful metrics tool execution: %+v", resp)
	}
	if !strings.Contains(callRes.Content[0].Text, "payment-api") {
		t.Fatalf("expected response to contain series data, got: %s", callRes.Content[0].Text)
	}

	// Call forgeops_get_topology
	topoArgs, _ := json.Marshal(protocol.CallToolParams{
		Name:      "forgeops_get_topology",
		Arguments: map[string]any{"resource_id": "root-1", "hops": 2.0},
	})
	callReq.Params = topoArgs
	respTopo := srv.HandleRequest(context.Background(), callReq)
	topoRes, ok := respTopo.Result.(protocol.CallToolResult)
	if !ok || topoRes.IsError {
		t.Fatalf("expected successful topology tool execution: %+v", respTopo)
	}
	if !strings.Contains(topoRes.Content[0].Text, "srv-prod-1") {
		t.Fatalf("expected response to contain node-1, got: %s", topoRes.Content[0].Text)
	}
}

func TestMCPServer_ProposeRemediation_DraftOnly(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	mockRem := &mockRemediationRepo{}
	srv := NewServer(Config{}, nil, nil, mockRem, nil, nil, nil, nil, logger)

	remArgs, _ := json.Marshal(protocol.CallToolParams{
		Name: "forgeops_propose_remediation",
		Arguments: map[string]any{
			"investigation_id": "inv-99",
			"title":            "Restart Payment Pods",
			"action_type":      "restart_pod",
			"parameters":       map[string]any{"target": "payment-api-prod"},
		},
	})
	req := protocol.Request{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "tools/call",
		Params:  remArgs,
	}
	resp := srv.HandleRequest(context.Background(), req)
	callRes, ok := resp.Result.(protocol.CallToolResult)
	if !ok || callRes.IsError {
		t.Fatalf("expected successful remediation proposal: %+v", resp)
	}
	if mockRem.created == nil || mockRem.created.Status != models.RemediationPendingApproval {
		t.Fatalf("expected proposal created with pending_approval status, got %+v", mockRem.created)
	}
}

func TestMCPServer_StdioTransport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	srv := NewServer(Config{}, nil, nil, nil, nil, nil, nil, nil, logger)

	inBuf := bytes.NewBufferString(`{"jsonrpc":"2.0","id":10,"method":"ping"}` + "\n")
	var outBuf bytes.Buffer

	err := srv.RunStdio(context.Background(), inBuf, &outBuf)
	if err != nil {
		t.Fatalf("stdio transport failed: %v", err)
	}

	var resp protocol.Response
	if err := json.Unmarshal(outBuf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v, raw: %s", err, outBuf.String())
	}
	if resp.Error != nil {
		t.Fatalf("unexpected ping error: %v", resp.Error)
	}
}
