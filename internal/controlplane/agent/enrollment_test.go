// SPDX-License-Identifier: MIT

package agent

import (
	"io"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/agent/identity"
	"github.com/onisimchukv/uptimefoundry/internal/agent/protocol"
	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

func TestCAEnrollmentAndCloneDetection(t *testing.T) {
	ca, err := NewCA()
	if err != nil {
		t.Fatalf("failed to create ca: %v", err)
	}

	token := ca.CreateBootstrapToken("env-prod-1", 1, 1*time.Hour)

	// Generate client key and CSR
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to gen key: %v", err)
	}

	csrPEM, err := identity.CreateCSR(clientKey, "pending-agent")
	if err != nil {
		t.Fatalf("failed to create csr: %v", err)
	}

	// 1. Successful Enrollment
	fingerprint1 := "fp-machine-original-12345"
	agentID, certPEM, expiresAt, err := ca.Enroll(token, fingerprint1, csrPEM)
	if err != nil {
		t.Fatalf("enrollment failed: %v", err)
	}

	if agentID == "" || len(certPEM) == 0 {
		t.Fatal("expected non-empty agentID and certPEM")
	}
	if time.Until(expiresAt) <= 0 {
		t.Fatal("expected cert to expire in the future")
	}

	// 2. Second enrollment with same single-use token must be refused
	_, _, _, err = ca.Enroll(token, "fp-machine-clone", csrPEM)
	if err == nil {
		t.Fatal("expected single-use token reuse to be rejected")
	}

	// 3. Clone Detection on Renewal
	newCSRPEM, _ := identity.CreateCSR(clientKey, agentID)
	// Same fingerprint -> Success
	_, _, err = ca.RenewCertificate(agentID, fingerprint1, newCSRPEM)
	if err != nil {
		t.Fatalf("renewal with matching fingerprint should succeed: %v", err)
	}

	// Different fingerprint -> Clone Detected!
	_, _, err = ca.RenewCertificate(agentID, "fp-machine-cloned-bad-actor", newCSRPEM)
	if err == nil {
		t.Fatal("expected clone with mismatched fingerprint to be rejected")
	}
}

type mockResourceRepo struct {
	upsertedRes []*models.Resource
	upsertedRel []*models.Relationship
}

func (m *mockResourceRepo) UpsertResource(ctx context.Context, res *models.Resource) error {
	m.upsertedRes = append(m.upsertedRes, res)
	return nil
}
func (m *mockResourceRepo) GetResourceByID(ctx context.Context, id string) (*models.Resource, error) {
	return nil, nil
}
func (m *mockResourceRepo) ListResources(ctx context.Context, envID string, kind models.ResourceKind, limit, offset int) ([]*models.Resource, error) {
	return m.upsertedRes, nil
}
func (m *mockResourceRepo) UpsertRelationship(ctx context.Context, rel *models.Relationship) error {
	m.upsertedRel = append(m.upsertedRel, rel)
	return nil
}
func (m *mockResourceRepo) GetRelationships(ctx context.Context, resourceID string) ([]*models.Relationship, error) {
	return m.upsertedRel, nil
}
func (m *mockResourceRepo) GetTopologySubgraph(ctx context.Context, rootResourceID string, hops int) ([]*models.Resource, []*models.Relationship, error) {
	return nil, nil, nil
}
func (m *mockResourceRepo) ExecuteCypher(ctx context.Context, query string) (any, error) {
	return nil, nil
}

func TestIngestionService(t *testing.T) {
	ca, _ := NewCA()
	mockRepo := &mockResourceRepo{}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	svc := NewIngestionService(ca, mockRepo, logger)

	// 1. Test Register
	regMsg := &protocol.AgentMessage{
		AgentID:     "agent-01",
		Class:       protocol.ClassRegister,
		Sequence:    1,
		Register: &protocol.RegisterPayload{
			AgentVersion:    "1.3.0",
			HostFingerprint: "fp-test",
			Capabilities:    map[string]any{"tier": 1},
		},
	}
	resp, err := svc.HandleMessage(context.Background(), regMsg)
	if err != nil {
		t.Fatalf("unexpected register error: %v", err)
	}
	if resp.Policy == nil {
		t.Fatal("expected server policy returned on register")
	}

	// 2. Test Inventory Delta
	invMsg := &protocol.AgentMessage{
		AgentID:  "agent-01",
		Class:    protocol.ClassInventory,
		Sequence: 2,
		Inventory: &protocol.InventoryDeltaPayload{
			UpsertedResources: []*models.Resource{
				{ID: "res-host-1", Kind: models.KindHost, Name: "worker-01"},
			},
			Relationships: []*models.Relationship{
				{SourceResourceID: "res-host-1", TargetResourceID: "res-net-1", RelationshipType: models.RelContains},
			},
		},
	}
	respInv, err := svc.HandleMessage(context.Background(), invMsg)
	if err != nil {
		t.Fatalf("unexpected inventory error: %v", err)
	}
	if respInv.AckSequence != 2 {
		t.Fatalf("expected ack sequence 2, got %d", respInv.AckSequence)
	}
	if len(mockRepo.upsertedRes) != 1 {
		t.Fatalf("expected 1 resource persisted in repo, got %d", len(mockRepo.upsertedRes))
	}
	if len(mockRepo.upsertedRel) != 1 {
		t.Fatalf("expected 1 relationship persisted in repo, got %d", len(mockRepo.upsertedRel))
	}

	// 3. Test Flow Delta
	flowMsg := &protocol.AgentMessage{
		AgentID:  "agent-01",
		Class:    protocol.ClassFlow,
		Sequence: 3,
		Flow: &protocol.FlowDeltaPayload{
			Edges: []protocol.FlowEdge{
				{
					SourceWorkload: "pod-billing",
					DestWorkload:   "db-postgres",
					Protocol:       "postgres",
					MeanRTTMs:      1.45,
					BytesSent:      1024,
					BytesRecv:      4096,
				},
			},
		},
	}
	respFlow, err := svc.HandleMessage(context.Background(), flowMsg)
	if err != nil {
		t.Fatalf("unexpected flow error: %v", err)
	}
	if respFlow.AckSequence != 3 {
		t.Fatalf("expected ack sequence 3, got %d", respFlow.AckSequence)
	}
	if len(mockRepo.upsertedRel) != 2 {
		t.Fatalf("expected flow edge persisted as relationship, got total %d", len(mockRepo.upsertedRel))
	}
	lastRel := mockRepo.upsertedRel[1]
	if lastRel.RelationshipType != models.RelConnectsTo {
		t.Fatalf("expected relationship CONNECTS_TO, got %s", lastRel.RelationshipType)
	}
	if lastRel.Weight != 1.45 {
		t.Fatalf("expected weight 1.45 ms, got %f", lastRel.Weight)
	}
}

type testMockRemediationRepo struct {
	recordedID     string
	recordedResult map[string]any
	recordedErr    error
}

func (m *testMockRemediationRepo) CreateProposal(ctx context.Context, rem *models.Remediation) error {
	return nil
}

func (m *testMockRemediationRepo) GetProposalByID(ctx context.Context, id string) (*models.Remediation, error) {
	return nil, nil
}

func (m *testMockRemediationRepo) ApproveProposal(ctx context.Context, id, approver string) error {
	return nil
}

func (m *testMockRemediationRepo) RecordExecution(ctx context.Context, id string, result map[string]any, execErr error) error {
	m.recordedID = id
	m.recordedResult = result
	m.recordedErr = execErr
	return nil
}

func TestIngestionService_TaskDispatchAndExecution(t *testing.T) {
	ca, _ := NewCA()
	mockRepo := &mockResourceRepo{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := NewIngestionService(ca, mockRepo, logger)
	remRepo := &testMockRemediationRepo{}
	svc.SetRemediationRepo(remRepo)

	// 1. Queue a task for agent-01
	task := &protocol.TaskPayload{
		TaskID:     "rem-100",
		TaskType:   "systemd_restart",
		Target:     "nginx.service",
		ApprovedBy: "alice",
		Signature:  "test-sig-123",
	}
	target := svc.QueueTask("agent-01", task)
	if target != "agent-01" {
		t.Fatalf("expected target agent-01, got %s", target)
	}

	// 2. Agent sends heartbeat -> receives the queued task
	hbMsg := &protocol.AgentMessage{
		AgentID:   "agent-01",
		Class:     protocol.ClassHeartbeat,
		Sequence:  10,
		Heartbeat: &protocol.HeartbeatPayload{HealthStatus: "healthy"},
	}
	resp, err := svc.HandleMessage(context.Background(), hbMsg)
	if err != nil {
		t.Fatalf("unexpected heartbeat error: %v", err)
	}
	if resp.Task == nil {
		t.Fatal("expected pending task to be dispatched in server response")
	}
	if resp.Task.TaskID != "rem-100" || resp.Task.TaskType != "systemd_restart" {
		t.Fatalf("unexpected task in response: %+v", resp.Task)
	}

	// 3. Second heartbeat -> queue is empty, no task returned
	resp2, _ := svc.HandleMessage(context.Background(), hbMsg)
	if resp2.Task != nil {
		t.Fatal("expected no task in second response since task queue was drained")
	}

	// 4. Agent sends task execution result
	taskResMsg := &protocol.AgentMessage{
		AgentID:  "agent-01",
		Class:    protocol.ClassTask,
		Sequence: 11,
		TaskResult: &protocol.TaskResultPayload{
			TaskID:       "rem-100",
			Success:      true,
			Status:       "success",
			Output:       "nginx.service successfully restarted",
			DurationMs:   120,
			CompletedAt:  time.Now().UTC(),
		},
	}
	_, err = svc.HandleMessage(context.Background(), taskResMsg)
	if err != nil {
		t.Fatalf("unexpected task result error: %v", err)
	}

	// Verify remediation repo recorded the execution
	if remRepo.recordedID != "rem-100" {
		t.Fatalf("expected recorded task ID 'rem-100', got %q", remRepo.recordedID)
	}
	if remRepo.recordedResult["output"] != "nginx.service successfully restarted" {
		t.Fatalf("expected recorded output to match, got %+v", remRepo.recordedResult)
	}
	if remRepo.recordedErr != nil {
		t.Fatalf("expected nil recorded error for successful task, got %v", remRepo.recordedErr)
	}
}
