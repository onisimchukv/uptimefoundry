package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/forgeops/forgeops/internal/config"
	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/graph"
	"github.com/forgeops/forgeops/internal/graph/age"
	"github.com/forgeops/forgeops/internal/graph/fallback"
	"github.com/forgeops/forgeops/internal/graph/traverser"
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

	// Initialize Storage and Graph Engine
	var (
		db           *postgres.DB
		resourceRepo *postgres.ResourceRepository
		graphEngine  graph.GraphEngine
		topTraverser *traverser.TopologyTraverser
	)

	if cfg.Database.DSN != "" {
		dbInstance, err := postgres.New(ctx, &cfg.Database, logger)
		if err != nil {
			logger.Warn("database connection failed; starting in offline mode",
				slog.Any("error", err),
			)
		} else {
			db = dbInstance
			defer db.Close()
			logger.Info("connected to postgresql metadata engine")

			// Run embedded migrations
			if err := db.Migrate(ctx); err != nil {
				logger.Error("failed to run database migrations", slog.Any("error", err))
			} else {
				logger.Info("database migrations verified")
			}

			// Select Graph Engine: Apache AGE if available, else recursive CTE fallback
			if db.HasAGE && cfg.Database.Extensions.AGE.Enabled {
				graphEngine = age.New(db.Pool, cfg.Database.Extensions.AGE.GraphName, logger)
				logger.Info("selected primary graph engine", slog.String("engine", graphEngine.Name()))
			} else {
				graphEngine = fallback.New(db.Pool, logger)
				logger.Info("selected fallback graph engine", slog.String("engine", graphEngine.Name()))
			}

			resourceRepo = postgres.NewResourceRepository(db, graphEngine, logger)
			topTraverser = traverser.New(graphEngine, logger)
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
				if graphEngine != nil {
					details["graph_engine"] = graphEngine.Name()
				}
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
		if graphEngine != nil {
			status["graph_engine"] = graphEngine.Name()
		}
		_ = json.NewEncoder(w).Encode(status)
	})

	// Resource API Endpoints
	mux.HandleFunc("/api/v1/resources", func(w http.ResponseWriter, r *http.Request) {
		if resourceRepo == nil {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		envID := r.URL.Query().Get("environment_id")
		kind := models.ResourceKind(r.URL.Query().Get("kind"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		resources, err := resourceRepo.ListResources(r.Context(), envID, kind, limit, offset)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resources)
	})

	// Topology & Blast Radius Endpoints
	mux.HandleFunc("/api/v1/topology/subgraph", func(w http.ResponseWriter, r *http.Request) {
		if resourceRepo == nil {
			http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		rootID := r.URL.Query().Get("resource_id")
		if rootID == "" {
			http.Error(w, `{"error":"resource_id is required"}`, http.StatusBadRequest)
			return
		}
		hops, _ := strconv.Atoi(r.URL.Query().Get("hops"))
		if hops <= 0 {
			hops = 2
		}

		resources, rels, err := resourceRepo.GetTopologySubgraph(r.Context(), rootID, hops)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"root_id":       rootID,
			"hops":          hops,
			"resources":     resources,
			"relationships": rels,
		})
	})

	mux.HandleFunc("/api/v1/topology/blast-radius", func(w http.ResponseWriter, r *http.Request) {
		if topTraverser == nil {
			http.Error(w, `{"error":"traverser unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		rootID := r.URL.Query().Get("resource_id")
		if rootID == "" {
			http.Error(w, `{"error":"resource_id is required"}`, http.StatusBadRequest)
			return
		}
		hops, _ := strconv.Atoi(r.URL.Query().Get("hops"))
		if hops <= 0 {
			hops = 2
		}

		report, err := topTraverser.ComputeBlastRadius(r.Context(), rootID, hops)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)
	})

	server := &http.Server{
		Addr:         cfg.Server.ListenAddress,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

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
