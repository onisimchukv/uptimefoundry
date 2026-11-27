// SPDX-License-Identifier: MIT

package supervisor

import (
	"github.com/onisimchukv/uptimefoundry/internal/agent/actuator"
	"net"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/agent/policy"
	"github.com/onisimchukv/uptimefoundry/internal/agent/protocol"
	cpagent "github.com/onisimchukv/uptimefoundry/internal/controlplane/agent"
)

func TestSupervisor_LifecycleAndWALDrain(t *testing.T) {
	tempDir := t.TempDir()

	ca, err := cpagent.NewCA()
	if err != nil {
		t.Fatalf("failed to create test CA: %v", err)
	}

	token := ca.CreateBootstrapToken("00000000-0000-0000-0000-000000000001", 10, time.Hour)

	var receivedMessages []*protocol.AgentMessage
	var enrolled bool

	// Mock Control Plane Server using real CA logic
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/agent/enroll":
			enrolled = true
			var req struct {
				Token           string `json:"token"`
				HostFingerprint string `json:"host_fingerprint"`
				CSRPEM          string `json:"csr_pem"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			agentID, certPEM, exp, err := ca.Enroll(req.Token, req.HostFingerprint, []byte(req.CSRPEM))
			if err != nil {
				http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusUnauthorized)
				return
			}

			_ = json.NewEncoder(w).Encode(map[string]any{
				"agent_id":        agentID,
				"certificate_pem": string(certPEM),
				"ca_cert_pem":     string(ca.CACertificatePEM()),
				"expires_at":      exp,
			})

		case "/api/v1/agent/message":
			var msg protocol.AgentMessage
			if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			receivedMessages = append(receivedMessages, &msg)

			// Return ACK response
			_ = json.NewEncoder(w).Encode(protocol.ServerMessage{
				AckSequence: msg.Sequence,
				AckClass:    msg.Class,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := Config{
		ControlPlaneURL: server.URL,
		StateDir:        tempDir,
		SocketPath:      filepath.Join(tempDir, "agent.sock"),
		BootstrapToken:  token,
		EnvironmentID:   "00000000-0000-0000-0000-000000000001",
		WALMaxMB:        10,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sup, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test Start (which performs enrollment and starts loops)
	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer sup.Stop()

	if !enrolled {
		t.Error("expected supervisor to enroll with control plane")
	}

	// Push a test message into the WAL
	testMsg := &protocol.AgentMessage{
		Class: protocol.ClassHeartbeat,
		Heartbeat: &protocol.HeartbeatPayload{
			HealthStatus: "healthy",
			CurrentTier:  1,
		},
	}
	if err := sup.wal.Push(testMsg); err != nil {
		t.Fatalf("failed to push message to wal: %v", err)
	}

	// Drain WAL to mock control plane
	endpoint := server.URL + "/api/v1/agent/message"
	sup.drainWAL(ctx, endpoint)

	// Verify message was received by server
	if len(receivedMessages) == 0 {
		t.Fatal("expected server to receive messages")
	}

	// Verify supervisor marked as connected
	status := sup.Status()
	if connected, ok := status["connected"].(bool); !ok || !connected {
		t.Errorf("expected supervisor status to show connected=true")
	}
	if status["agent_id"] == "" {
		t.Errorf("expected supervisor status to have non-empty agent_id")
	}

	// Test Status and Diagnostics
	_, filename, err := sup.GenerateDiagnosticsBundle()
	if err != nil {
		t.Fatalf("failed to generate diagnostics: %v", err)
	}
	if filename == "" {
		t.Error("expected non-empty diagnostics filename")
	}

	_ = os.RemoveAll(tempDir)
}

func TestSupervisor_PolicyReload(t *testing.T) {
	tempDir := t.TempDir()
	policyFile := filepath.Join(tempDir, "policy.yaml")

	initialYAML := `
version: 1
modules:
  discovery:
    allowed: true
  logs:
    allowed: false
`
	if err := os.WriteFile(policyFile, []byte(initialYAML), 0600); err != nil {
		t.Fatalf("failed to write initial policy: %v", err)
	}

	cfg := Config{
		StateDir:        tempDir,
		SocketPath:      filepath.Join(tempDir, "agent.sock"),
		LocalPolicyPath: policyFile,
		WALMaxMB:        5,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sup, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	eff := sup.EffectivePolicy()
	if eff == nil {
		t.Fatal("expected non-nil effective policy")
	}
	if eff.Modules["logs"].Allowed {
		t.Fatal("expected logs module to be disallowed initially")
	}

	// Update local policy file
	updatedYAML := `
version: 2
modules:
  discovery:
    allowed: true
  logs:
    allowed: true
`
	if err := os.WriteFile(policyFile, []byte(updatedYAML), 0600); err != nil {
		t.Fatalf("failed to write updated policy: %v", err)
	}

	// Trigger policy reload
	if err := sup.ReloadPolicy(); err != nil {
		t.Fatalf("failed to reload policy: %v", err)
	}

	effAfter := sup.EffectivePolicy()
	if !effAfter.Modules["logs"].Allowed {
		t.Fatal("expected logs module to be allowed after reload")
	}

	// Test remote policy narrowing
	remote := &policy.RemotePolicy{
		Version: 3,
		Modules: map[string]policy.ModuleRule{
			"logs": {Allowed: false},
		},
	}
	sup.ApplyRemotePolicy(remote)

	effNarrowed := sup.EffectivePolicy()
	if effNarrowed.Modules["logs"].Allowed {
		t.Fatal("expected logs module to be narrowed (disallowed) by remote policy")
	}
}

func TestSupervisor_CertAutoRenewal(t *testing.T) {
	tempDir := t.TempDir()

	ca, err := cpagent.NewCA()
	if err != nil {
		t.Fatalf("failed to create CA: %v", err)
	}

	token := ca.CreateBootstrapToken("env-1", 5, time.Hour)

	var renewedCertCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v1/agent/enroll":
			var req struct {
				Token           string `json:"token"`
				HostFingerprint string `json:"host_fingerprint"`
				CSRPEM          string `json:"csr_pem"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			agentID, certPEM, exp, err := ca.Enroll(req.Token, req.HostFingerprint, []byte(req.CSRPEM))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"agent_id":        agentID,
				"certificate_pem": string(certPEM),
				"expires_at":      exp,
			})

		case "/api/v1/agent/message":
			var msg protocol.AgentMessage
			_ = json.NewDecoder(r.Body).Decode(&msg)

			if msg.Class == protocol.ClassCertRenew && msg.CertRenew != nil {
				renewedCertCount++
				certPEM, exp, err := ca.RenewCertificate(msg.AgentID, msg.CertRenew.HostFingerprint, msg.CertRenew.CSR)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}

				_ = json.NewEncoder(w).Encode(protocol.ServerMessage{
					AckSequence: msg.Sequence,
					AckClass:    msg.Class,
					CertRenewResp: &protocol.CertRenewResponse{
						CertificateChain: [][]byte{certPEM},
						ExpiresAt:        exp,
					},
				})
				return
			}

			_ = json.NewEncoder(w).Encode(protocol.ServerMessage{
				AckSequence: msg.Sequence,
				AckClass:    msg.Class,
			})

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cfg := Config{
		ControlPlaneURL: server.URL,
		StateDir:        tempDir,
		SocketPath:      filepath.Join(tempDir, "agent.sock"),
		BootstrapToken:  token,
		EnvironmentID:   "env-1",
		WALMaxMB:        5,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sup, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer sup.Stop()

	// 1. Explicit Renewal test
	if err := sup.RenewCertificate(ctx); err != nil {
		t.Fatalf("failed to renew certificate: %v", err)
	}

	if renewedCertCount != 1 {
		t.Fatalf("expected renewedCertCount=1, got %d", renewedCertCount)
	}

	// 2. Simulated Auto-Renewal Trigger when approaching expiration
	sup.mu.Lock()
	sup.id.ExpiresAt = time.Now().Add(2 * 24 * time.Hour) // 2 days left (within 7-day window)
	sup.mu.Unlock()

	sup.checkAndRenewCertificate(ctx)

	if renewedCertCount != 2 {
		t.Fatalf("expected renewedCertCount=2 after checkAndRenewCertificate, got %d", renewedCertCount)
	}

	// Verify expiration was extended beyond 2 days
	sup.mu.RLock()
	newExpiry := sup.id.ExpiresAt
	sup.mu.RUnlock()

	if time.Until(newExpiry) < 20*24*time.Hour {
		t.Errorf("expected renewed cert expiry far in future, got %v", newExpiry)
	}
}

func TestSupervisor_TaskForwardingAndExecution(t *testing.T) {
	tempDir := t.TempDir()

	socketPath := filepath.Join(tempDir, "actuator.sock")
	cfg := Config{
		StateDir:           tempDir,
		SocketPath:         filepath.Join(tempDir, "agent.sock"),
		ActuatorSocketPath: socketPath,
		WALMaxMB:           5,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sup, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer sup.Stop()

	testTask := &protocol.TaskPayload{
		TaskID:     "task-test-01",
		TaskType:   "systemd_restart",
		Target:     "nginx.service",
		ApprovedBy: "operator-bob",
		IssuedAt:   time.Now().UTC(),
		ExpiresAt:  time.Now().UTC().Add(5 * time.Minute),
		Signature:  "mock-sig",
	}

	// 1. When actuator is unreachable, ExecuteTask returns graceful rejection
	rejResult, err := sup.ExecuteTask(ctx, testTask)
	if err != nil {
		t.Fatalf("unexpected error on unreachable actuator: %v", err)
	}
	if rejResult == nil || rejResult.Success || rejResult.Status != "rejected" {
		t.Fatalf("expected rejected status for unreachable actuator, got %+v", rejResult)
	}

	// 2. Set up mock actuator Unix domain socket if supported
	listener, err := net.Listen("unix", socketPath)
	if err == nil {
		defer listener.Close()
		go func() {
			mux := http.NewServeMux()
			mux.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				var req actuator.SignedActionTask
				_ = json.NewDecoder(r.Body).Decode(&req)
				_ = json.NewEncoder(w).Encode(actuator.ExecutionResult{
					TaskID:       req.TaskID,
					Success:      true,
					Output:       "restarted " + req.Target,
					ExecutedAt:   time.Now().UTC(),
					DurationMs:   35,
				})
			})
			_ = http.Serve(listener, mux)
		}()

		// Allow server to accept
		time.Sleep(50 * time.Millisecond)

		successResult, err := sup.ExecuteTask(ctx, testTask)
		if err != nil {
			t.Fatalf("unexpected error executing task against mock actuator: %v", err)
		}
		if !successResult.Success || successResult.Status != "success" {
			t.Fatalf("expected successful task execution, got %+v", successResult)
		}
		if successResult.Output != "restarted nginx.service" {
			t.Fatalf("unexpected output: %s", successResult.Output)
		}
	} else {
		t.Logf("unix domain sockets not supported on this platform (%v); verified rejection path", err)
	}

	// 3. Test handleAssignedTask pushes ClassTask result to WAL
	sup.handleAssignedTask(ctx, testTask)

	unacked := sup.wal.Peek(10)

	var foundTaskResult bool
	for _, msg := range unacked {
		if msg.Class == protocol.ClassTask && msg.TaskResult != nil && msg.TaskResult.TaskID == "task-test-01" {
			foundTaskResult = true
			break
		}
	}
	if !foundTaskResult {
		t.Fatal("expected task result message to be pushed into WAL")
	}
}
