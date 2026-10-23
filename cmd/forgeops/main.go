package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/forgeops/forgeops/internal/config"
	"github.com/forgeops/forgeops/internal/storage/postgres"
)

var (
	version   = "v1.2.0-dev"
	buildTime = time.Now().UTC().Format(time.RFC3339)
)

func main() {
	configPath := flag.String("config", "configs/forgeops.example.yaml", "Path to ForgeOps YAML configuration file")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting forgeops control plane",
		slog.String("version", version),
		slog.String("build_time", buildTime),
		slog.String("config", *configPath),
	)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", slog.Any("error", err))
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Try connecting to PostgreSQL
	var db *postgres.DB
	if cfg.Database.DSN != "" {
		dbInstance, err := postgres.New(ctx, &cfg.Database, logger)
		if err != nil {
			logger.Warn("database connection failed; starting in offline discovery mode",
				slog.Any("error", err),
			)
		} else {
			db = dbInstance
			defer db.Close()
			logger.Info("connected to postgresql metadata engine")
		}
	}

	mux := http.NewServeMux()

	// Health and Status Endpoints
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ready := true
		details := map[string]any{
			"status": "ready",
		}

		if db != nil {
			if err := db.Ping(r.Context()); err != nil {
				ready = false
				details["database"] = "unhealthy"
			} else {
				details["database"] = "connected"
				details["apache_age"] = db.HasAGE
				details["pgvector"] = db.HasVector
			}
		} else {
			details["database"] = "disconnected"
		}

		if !ready {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		_ = json.NewEncoder(w).Encode(details)
	})

	mux.HandleFunc("/api/v1/version", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"version":    version,
			"build_time": buildTime,
			"runtime":    "go",
		})
	})

	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := map[string]any{
			"version":    version,
			"discovery":  cfg.Discovery.Interval.String(),
			"telemetry":  cfg.Telemetry.Metrics.Provider,
			"ebpf":       cfg.EBPF.Enabled,
			"rag":        cfg.RAG.Enabled,
			"mcp_server": cfg.MCP.Server.Enabled,
		}
		_ = json.NewEncoder(w).Encode(status)
	})

	server := &http.Server{
		Addr:         cfg.Server.ListenAddress,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown listener
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("http server listening", slog.String("addr", cfg.Server.ListenAddress))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	<-stopChan
	logger.Info("shutdown signal received; commencing graceful teardown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown failed", slog.Any("error", err))
	} else {
		logger.Info("server shutdown completed successfully")
	}
}
