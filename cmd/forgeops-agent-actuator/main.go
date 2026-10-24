// ForgeOps Agent Actuator — Separate Binary for Remediation Execution
//
// This is intentionally a distinct binary and package from the main agent supervisor.
// The actuator ships as its own RPM (forgeops-agent-actuator) with its own systemd
// unit, disabled by default, so a host that does not install it has zero remediation
// code on disk.
//
// See Specification v1.3 Section 3.4 and Roadmap Step A5.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	stateDir := os.Getenv("FORGEOPS_ACTUATOR_STATE_DIR")
	if stateDir == "" {
		stateDir = "/var/lib/forgeops-agent/actuator"
	}

	socketPath := os.Getenv("FORGEOPS_ACTUATOR_SOCKET")
	if socketPath == "" {
		socketPath = filepath.Join(stateDir, "actuator.sock")
	}

	_ = os.MkdirAll(stateDir, 0700)

	logger.Info("forgeops-agent-actuator starting",
		slog.String("state_dir", stateDir),
		slog.String("socket", socketPath),
		slog.String("version", "1.3.0"),
	)

	// Set up Unix domain socket listener (no TCP — Section 3.4)
	_ = os.Remove(socketPath)
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
			"state":      "idle",
			"dry_run":    true,
			"socket":     socketPath,
			"version":    "1.3.0",
			"audit_log":  filepath.Join(stateDir, "audit.jsonl"),
		})
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

	logger.Info("forgeops-agent-actuator ready (Unix socket only; no TCP listeners)")

	<-ctx.Done()
	logger.Info("shutting down actuator")
	_ = server.Shutdown(context.Background())
	_ = os.Remove(socketPath)
	fmt.Println("forgeops-agent-actuator stopped")
}
