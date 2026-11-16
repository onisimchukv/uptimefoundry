// SPDX-License-Identifier: MIT

// UptimeFoundry Agent Actuator — Separate Binary for Remediation Execution
//
// This is intentionally a distinct binary and package from the main agent supervisor.
// The actuator ships as its own RPM (uptimefoundry-agent-actuator) with its own systemd
// unit, disabled by default, so a host that does not install it has zero remediation
// code on disk.
//
// See Specification v1.3 Section 3.4 and Roadmap Step A5.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/onisimchukv/uptimefoundry/internal/agent/actuator"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	stateDir := os.Getenv("UPTIMEFOUNDRY_ACTUATOR_STATE_DIR")
	if stateDir == "" {
		stateDir = "/var/lib/uptimefoundry-agent/actuator"
	}

	socketPath := os.Getenv("UPTIMEFOUNDRY_ACTUATOR_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(stateDir, "actuator.sock")
	}

	_ = os.MkdirAll(stateDir, 0700)

	logger.Info("uptimefoundry-agent-actuator starting",
		slog.String("state_dir", stateDir),
		slog.String("socket", socketPath),
		slog.String("version", "1.3.0"),
	)

	// Set up Verifier
	var verifier actuator.TaskVerifier
	secretKey := os.Getenv("UPTIMEFOUNDRY_ACTUATOR_SECRET_KEY")
	if secretKey != "" {
		verifier = actuator.NewHMACVerifier([]byte(secretKey))
	} else {
		// Development default key with explicit warning
		logger.Warn("running with default development actuator HMAC key; do not use in production")
		verifier = actuator.NewHMACVerifier([]byte("uptimefoundry-actuator-dev-key"))
	}

	// Local Security Policy Ceiling
	allowedActions := map[string]bool{
		"systemd_restart": true,
		"drop_caches":     true,
		"tcp_kill":        true,
	}
	if customActions := os.Getenv("UPTIMEFOUNDRY_ACTUATOR_ALLOWED_ACTIONS"); customActions != "" {
		allowedActions = make(map[string]bool)
		for _, a := range strings.Split(customActions, ",") {
			a = strings.TrimSpace(a)
			if a != "" {
				allowedActions[a] = true
			}
		}
	}

	dryRunDefault := os.Getenv("UPTIMEFOUNDRY_ACTUATOR_DRY_RUN") != "false"
	auditLogPath := filepath.Join(stateDir, "audit.jsonl")
	audit, err := actuator.NewFileAuditLogger(auditLogPath)
	if err != nil {
		logger.Warn("failed to initialize audit log", slog.Any("error", err))
	}

	executor := actuator.NewSystemExecutor()
	act := actuator.NewActuator(
		actuator.ActuatorConfig{
			AllowedActions: allowedActions,
			DryRunDefault:  dryRunDefault,
		},
		verifier,
		executor,
		audit,
		logger,
	)

	// Set up Unix domain socket listener (no TCP — Section 3.4)
	_ = os.Remove(socketPath)
	_ = os.MkdirAll(filepath.Dir(socketPath), 0755)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		logger.Error("failed to bind Unix socket", slog.Any("error", err))
		os.Exit(1)
	}
	defer listener.Close()
	_ = os.Chmod(socketPath, 0660)

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ready",
			"version": "1.3.0",
		})
	})

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state":           "ready",
			"dry_run_default": dryRunDefault,
			"socket":          socketPath,
			"allowed_actions": allowedActions,
			"version":         "1.3.0",
			"audit_log":       auditLogPath,
		})
	})

	// Execute Action Endpoint (POST /execute or /tasks/execute)
	handleExecute := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var task actuator.SignedActionTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid task json: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}

		result, err := act.Handle(r.Context(), task)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, actuator.ErrSignatureInvalid) || errors.Is(err, actuator.ErrTaskExpired) || errors.Is(err, actuator.ErrTaskNotYetValid) {
				status = http.StatusUnauthorized
			} else if errors.Is(err, actuator.ErrActionDeniedByCeiling) {
				status = http.StatusForbidden
			}
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(result)
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(result)
	}

	mux.HandleFunc("/execute", handleExecute)
	mux.HandleFunc("/tasks/execute", handleExecute)

	mux.HandleFunc("/audit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		records, err := os.ReadFile(auditLogPath)
		if err != nil {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		// Return raw json lines wrapped in array or formatted
		lines := strings.Split(strings.TrimSpace(string(records)), "\n")
		var parsed []map[string]any
		for _, l := range lines {
			if strings.TrimSpace(l) == "" {
				continue
			}
			var m map[string]any
			if err := json.Unmarshal([]byte(l), &m); err == nil {
				parsed = append(parsed, m)
			}
		}
		_ = json.NewEncoder(w).Encode(parsed)
	})

	// Graceful shutdown
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	server := &http.Server{Handler: mux}
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			logger.Error("actuator server error", slog.Any("error", err))
		}
	}()

	logger.Info("uptimefoundry-agent-actuator ready (Unix socket only; no TCP listeners)")

	<-ctx.Done()
	logger.Info("shutting down actuator")
	_ = server.Shutdown(context.Background())
	_ = os.Remove(socketPath)
	fmt.Println("uptimefoundry-agent-actuator stopped")
}
