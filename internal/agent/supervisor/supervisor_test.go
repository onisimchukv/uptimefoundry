// SPDX-License-Identifier: MIT

package supervisor

import (
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
