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

	"github.com/forgeops/forgeops/internal/adapters"
	"github.com/forgeops/forgeops/internal/adapters/host"
	"github.com/forgeops/forgeops/internal/agent/protocol"
	"github.com/forgeops/forgeops/internal/config"
	cpagent "github.com/forgeops/forgeops/internal/controlplane/agent"
	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/graph"
	"github.com/forgeops/forgeops/internal/graph/age"
	"github.com/forgeops/forgeops/internal/graph/fallback"
	"github.com/forgeops/forgeops/internal/graph/traverser"
	"github.com/forgeops/forgeops/internal/storage/postgres"
	"github.com/forgeops/forgeops/internal/telemetry"
	"github.com/forgeops/forgeops/internal/telemetry/providers/loki"
	"github.com/forgeops/forgeops/internal/telemetry/providers/victoriametrics"
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
		db              *postgres.DB
		resourceRepo    *postgres.ResourceRepository
		discoveryRepo   *postgres.DiscoveryRepository
		graphEngine     graph.GraphEngine
		topTraverser    *traverser.TopologyTraverser
		discoveryRunner *adapters.Runner
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

			if err := db.Migrate(ctx); err != nil {
				logger.Error("failed to run database migrations", slog.Any("error", err))
			} else {
				logger.Info("database migrations verified")
			}

			if db.HasAGE && cfg.Database.Extensions.AGE.Enabled {
				graphEngine = age.New(db.Pool, cfg.Database.Extensions.AGE.GraphName, logger)
				logger.Info("selected primary graph engine", slog.String("engine", graphEngine.Name()))
			} else {
				graphEngine = fallback.New(db.Pool, logger)
				logger.Info("selected fallback graph engine", slog.String("engine", graphEngine.Name()))
			}

			resourceRepo = postgres.NewResourceRepository(db, graphEngine, logger)
			discoveryRepo = postgres.NewDiscoveryRepository(db)
			topTraverser = traverser.New(graphEngine, logger)

			// Setup Discovery Adapters
			var activeAdapters []adapters.DiscoveryAdapter
			if cfg.Discovery.Adapters.Host.Enabled {
				activeAdapters = append(activeAdapters, host.New(logger))
				logger.Info("registered discovery adapter", slog.String("adapter", "host"))
			}

			discoveryRunner = adapters.NewRunner(activeAdapters, resourceRepo, discoveryRepo, logger)

			// Background Discovery Scheduler
			if cfg.Discovery.Interval > 0 && len(activeAdapters) > 0 {
				go func() {
					ticker := time.NewTicker(cfg.Discovery.Interval)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							_, _ = discoveryRunner.RunAll(ctx, "")
						}
					}
				}()
			}
		}
	}

	// Initialize Agent CA & Ingestion Service (v1.3 Security Boundary)
	agentCA, err := cpagent.NewCA()
	if err != nil {
		logger.Error("failed to initialize agent ca", slog.Any("error", err))
		os.Exit(1)
	}
	agentIngestion := cpagent.NewIngestionService(agentCA, resourceRepo, logger)

	// Initialize Telemetry Providers
	var (
		metricsProvider telemetry.MetricsProvider
		logsProvider    telemetry.LogsProvider
	)

	if cfg.Telemetry.Metrics.Provider == "victoriametrics" {
		metricsProvider = victoriametrics.New(
			cfg.Telemetry.Metrics.VictoriaMetrics.QueryEndpoint,
			cfg.Telemetry.Metrics.VictoriaMetrics.QueryTimeout,
			cfg.Telemetry.Metrics.VictoriaMetrics.MaxSeries,
			logger,
		)
		logger.Info("initialized victoriametrics provider",
			slog.String("endpoint", cfg.Telemetry.Metrics.VictoriaMetrics.QueryEndpoint),
		)
	}

	if cfg.Telemetry.Logs.Provider == "loki" {
		logsProvider = loki.New(
			cfg.Telemetry.Logs.Loki.QueryEndpoint,
			cfg.Telemetry.Logs.Loki.QueryTimeout,
			logger,
		)
		logger.Info("initialized loki provider",
			slog.String("endpoint", cfg.Telemetry.Logs.Loki.QueryEndpoint),
		)
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

	// Discovery Endpoints
	mux.HandleFunc("/api/v1/discovery/runs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			if discoveryRunner == nil {
				http.Error(w, `{"error":"discovery runner unavailable"}`, http.StatusServiceUnavailable)
				return
			}
			envID := r.URL.Query().Get("environment_id")
			run, err := discoveryRunner.RunAll(r.Context(), envID)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(run)
			return
		}

		if r.Method == http.MethodGet {
			if discoveryRepo == nil {
				http.Error(w, `{"error":"database unavailable"}`, http.StatusServiceUnavailable)
				return
			}
			envID := r.URL.Query().Get("environment_id")
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

			runs, err := discoveryRepo.ListRuns(r.Context(), envID, limit, offset)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(runs)
			return
		}

		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
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

	// Telemetry Endpoints
	mux.HandleFunc("/api/v1/telemetry/metrics/query", func(w http.ResponseWriter, r *http.Request) {
		if metricsProvider == nil {
			http.Error(w, `{"error":"metrics provider unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		var req struct {
			Query string    `json:"query"`
			Time  time.Time `json:"time"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
			return
		}

		result, err := metricsProvider.QueryInstant(r.Context(), req.Query, req.Time)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	mux.HandleFunc("/api/v1/telemetry/logs/query", func(w http.ResponseWriter, r *http.Request) {
		if logsProvider == nil {
			http.Error(w, `{"error":"logs provider unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		var req telemetry.LogQuery
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
			return
		}

		result, err := logsProvider.QueryLogs(r.Context(), req)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	// Agent Control Plane Endpoints (v1.3)
	mux.HandleFunc("/api/v1/agent/enroll", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Token           string `json:"token"`
			HostFingerprint string `json:"host_fingerprint"`
			CSRPEM          string `json:"csr_pem"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request json"}`, http.StatusBadRequest)
			return
		}

		agentID, certPEM, expiresAt, err := agentCA.Enroll(req.Token, req.HostFingerprint, []byte(req.CSRPEM))
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusUnauthorized)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"agent_id":        agentID,
			"certificate_pem": string(certPEM),
			"ca_cert_pem":     string(agentCA.CACertificatePEM()),
			"expires_at":      expiresAt,
		})
	})

	mux.HandleFunc("/api/v1/agent/message", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var msg protocol.AgentMessage
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, `{"error":"invalid message json"}`, http.StatusBadRequest)
			return
		}

		resp, err := agentIngestion.HandleMessage(r.Context(), &msg)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusInternalServerError)
			return
		}

		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/agents", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(agentIngestion.ActiveAgents())
	})

	mux.HandleFunc("/api/v1/agents/tokens", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			EnvironmentID string `json:"environment_id"`
			MaxUses       int    `json:"max_uses"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.EnvironmentID == "" {
			req.EnvironmentID = "00000000-0000-0000-0000-000000000001"
		}
		if req.MaxUses <= 0 {
			req.MaxUses = 1
		}

		tok := agentCA.CreateBootstrapToken(req.EnvironmentID, req.MaxUses, 24*time.Hour)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":          tok,
			"environment_id": req.EnvironmentID,
			"max_uses":       req.MaxUses,
			"expires_in":     "24h",
		})
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
