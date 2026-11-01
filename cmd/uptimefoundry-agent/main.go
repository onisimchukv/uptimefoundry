// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/agent/supervisor"
)

var (
	agentVersion = "v1.3.0-dev"
	buildTime    = time.Now().UTC().Format(time.RFC3339)
)

func main() {
	serverURL := flag.String("server", "https://127.0.0.1:8443", "Control Plane gRPC mTLS endpoint")
	policyPath := flag.String("policy", "", "Path to local security ceiling policy YAML")
	stateDir := flag.String("state", "./state", "Directory for identity keys, WAL, and cache")
	socketPath := flag.String("socket", "./state/agent.sock", "Path to local Unix domain socket for agentctl")
	token := flag.String("token", "", "One-time bootstrap enrollment token")
	envID := flag.String("env", "00000000-0000-0000-0000-000000000001", "Environment UUID")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting uptimefoundry-agent supervisor (v1.3 security-boundary)",
		slog.String("version", agentVersion),
		slog.String("build_time", buildTime),
		slog.String("control_plane", *serverURL),
		slog.String("state_dir", *stateDir),
		slog.String("socket", *socketPath),
	)

	cfg := supervisor.Config{
		ControlPlaneURL: *serverURL,
		LocalPolicyPath: *policyPath,
		StateDir:        *stateDir,
		SocketPath:      *socketPath,
		BootstrapToken:  *token,
		EnvironmentID:   *envID,
		WALMaxMB:        512,
	}

	sup, err := supervisor.New(cfg, logger)
	if err != nil {
		logger.Error("failed to initialize supervisor", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		logger.Error("failed to start supervisor", slog.Any("error", err))
		os.Exit(1)
	}

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	<-stopChan
	logger.Info("shutdown signal received; commencing agent teardown")

	sup.Stop()
	logger.Info("uptimefoundry-agent supervisor stopped cleanly")
}
