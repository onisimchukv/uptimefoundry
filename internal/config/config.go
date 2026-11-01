// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete UptimeFoundry operational configuration.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Discovery DiscoveryConfig `yaml:"discovery"`
	EBPF      EBPFConfig      `yaml:"ebpf"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	RAG       RAGConfig       `yaml:"rag"`
	MCP       MCPConfig       `yaml:"mcp"`
	AI        AIConfig        `yaml:"ai"`
	Security  SecurityConfig  `yaml:"security"`
}

type ServerConfig struct {
	ListenAddress string    `yaml:"listen_address"`
	TLS           TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type DatabaseConfig struct {
	DSN                 string           `yaml:"dsn"`
	DSNSecretRef        string           `yaml:"dsn_secret_ref"`
	MaxOpenConnections  int              `yaml:"max_open_connections"`
	MaxIdleConnections  int              `yaml:"max_idle_connections"`
	ConnMaxLifetime     time.Duration    `yaml:"conn_max_lifetime"`
	Extensions          ExtensionsConfig `yaml:"extensions"`
}

type ExtensionsConfig struct {
	AGE    AGEExtensionConfig    `yaml:"age"`
	Vector VectorExtensionConfig `yaml:"vector"`
}

type AGEExtensionConfig struct {
	Enabled   bool   `yaml:"enabled"`
	GraphName string `yaml:"graph_name"`
}

type VectorExtensionConfig struct {
	Enabled            bool `yaml:"enabled"`
	EmbeddingDimension int  `yaml:"embedding_dimension"`
}

type DiscoveryConfig struct {
	Interval time.Duration           `yaml:"interval"`
	Adapters DiscoveryAdaptersConfig `yaml:"adapters"`
}

type DiscoveryAdaptersConfig struct {
	Host        AdapterSwitch `yaml:"host"`
	Kubernetes  K8sAdapter    `yaml:"kubernetes"`
	DockerSwarm AdapterSwitch `yaml:"docker_swarm"`
}

type AdapterSwitch struct {
	Enabled bool `yaml:"enabled"`
}

type K8sAdapter struct {
	Enabled    bool `yaml:"enabled"`
	InCluster  bool `yaml:"in_cluster"`
	Kubeconfig string `yaml:"kubeconfig"`
}

type EBPFConfig struct {
	Enabled             bool   `yaml:"enabled"`
	ExportMetrics       bool   `yaml:"export_metrics"`
	OTLPMetricsEndpoint string `yaml:"otlp_metrics_endpoint"`
	MonitorTLS          bool   `yaml:"monitor_tls"`
	CaptureSQL          bool   `yaml:"capture_sql"`
	RingBufferPages     int    `yaml:"ring_buffer_pages"`
}

type TelemetryConfig struct {
	Metrics MetricsTelemetryConfig `yaml:"metrics"`
	Logs    LogsTelemetryConfig    `yaml:"logs"`
	Traces  TracesTelemetryConfig  `yaml:"traces"`
}

type MetricsTelemetryConfig struct {
	Provider        string          `yaml:"provider"` // "victoriametrics" or "prometheus"
	VictoriaMetrics VMConfig        `yaml:"victoriametrics"`
	Prometheus      PrometheusConfig `yaml:"prometheus"`
}

type VMConfig struct {
	QueryEndpoint string        `yaml:"query_endpoint"`
	QueryTimeout  time.Duration `yaml:"query_timeout"`
	MaxSeries     int           `yaml:"max_series"`
}

type PrometheusConfig struct {
	QueryEndpoint string        `yaml:"query_endpoint"`
	QueryTimeout  time.Duration `yaml:"query_timeout"`
}

type LogsTelemetryConfig struct {
	Provider string     `yaml:"provider"` // "loki"
	Loki     LokiConfig `yaml:"loki"`
}

type LokiConfig struct {
	QueryEndpoint string        `yaml:"query_endpoint"`
	QueryTimeout  time.Duration `yaml:"query_timeout"`
}

type TracesTelemetryConfig struct {
	Provider string     `yaml:"provider"` // "otel"
	OTel     OTelConfig `yaml:"otel"`
}

type OTelConfig struct {
	Endpoint string `yaml:"endpoint"`
}

type RAGConfig struct {
	Enabled             bool              `yaml:"enabled"`
	EmbeddingProvider   string            `yaml:"embedding_provider"` // "local_tei", "ollama", "openai"
	EmbeddingEndpoint   string            `yaml:"embedding_endpoint"`
	TopK                int               `yaml:"top_k"`
	SimilarityThreshold float64           `yaml:"similarity_threshold"`
	Sources             []RAGSourceConfig `yaml:"sources"`
}

type RAGSourceConfig struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"` // "runbook", "postmortem"
}

type MCPConfig struct {
	Server MCPServerConfig `yaml:"server"`
	Client MCPClientConfig `yaml:"client"`
}

type MCPServerConfig struct {
	Enabled    bool               `yaml:"enabled"`
	Transports MCPTransportConfig `yaml:"transports"`
}

type MCPTransportConfig struct {
	Stdio bool           `yaml:"stdio"`
	SSE   MCPSSEConfig   `yaml:"sse"`
}

type MCPSSEConfig struct {
	Enabled            bool   `yaml:"enabled"`
	Port               int    `yaml:"port"`
	AuthTokenSecretRef string `yaml:"auth_token_secret_ref"`
}

type MCPClientConfig struct {
	Enabled bool              `yaml:"enabled"`
	Servers []MCPServerTarget `yaml:"servers"`
}

type MCPServerTarget struct {
	Name    string            `yaml:"name"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

type AIConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Provider          string        `yaml:"provider"`
	Endpoint          string        `yaml:"endpoint"`
	Model             string        `yaml:"model"`
	RequestTimeout    time.Duration `yaml:"request_timeout"`
	MaxContextTokens  int           `yaml:"max_context_tokens"`
}

type SecurityConfig struct {
	Remediation RemediationSecurityConfig `yaml:"remediation"`
}

type RemediationSecurityConfig struct {
	Enabled                  bool `yaml:"enabled"`
	RequireTwoPersonApproval bool `yaml:"require_two_person_approval"`
}

// DefaultConfig returns safe operational defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			ListenAddress: "127.0.0.1:8080",
			TLS: TLSConfig{
				Enabled: false,
			},
		},
		Database: DatabaseConfig{
			DSN:                "",
			MaxOpenConnections: 25,
			MaxIdleConnections: 5,
			ConnMaxLifetime:    15 * time.Minute,
			Extensions: ExtensionsConfig{
				AGE: AGEExtensionConfig{
					Enabled:   true,
					GraphName: "opsfoundry_graph",
				},
				Vector: VectorExtensionConfig{
					Enabled:            true,
					EmbeddingDimension: 768,
				},
			},
		},
		Discovery: DiscoveryConfig{
			Interval: 5 * time.Minute,
			Adapters: DiscoveryAdaptersConfig{
				Host: AdapterSwitch{Enabled: true},
				Kubernetes: K8sAdapter{
					Enabled:   true,
					InCluster: false,
				},
				DockerSwarm: AdapterSwitch{Enabled: false},
			},
		},
		EBPF: EBPFConfig{
			Enabled:             false, // Off by default on non-Linux, enabled via agent
			ExportMetrics:       true,
			OTLPMetricsEndpoint: "http://127.0.0.1:8428/opentelemetry/v1/metrics",
			MonitorTLS:          true,
			CaptureSQL:          true,
			RingBufferPages:     64,
		},
		Telemetry: TelemetryConfig{
			Metrics: MetricsTelemetryConfig{
				Provider: "victoriametrics",
				VictoriaMetrics: VMConfig{
					QueryEndpoint: "http://localhost:8428",
					QueryTimeout:  15 * time.Second,
					MaxSeries:     2000,
				},
			},
			Logs: LogsTelemetryConfig{
				Provider: "loki",
				Loki: LokiConfig{
					QueryEndpoint: "http://localhost:3100",
					QueryTimeout:  15 * time.Second,
				},
			},
			Traces: TracesTelemetryConfig{
				Provider: "none",
			},
		},
		RAG: RAGConfig{
			Enabled:             false,
			EmbeddingProvider:   "local_tei",
			EmbeddingEndpoint:   "http://localhost:8081/embed",
			TopK:                4,
			SimilarityThreshold: 0.72,
		},
		MCP: MCPConfig{
			Server: MCPServerConfig{
				Enabled: true,
				Transports: MCPTransportConfig{
					Stdio: false,
					SSE: MCPSSEConfig{
						Enabled: true,
						Port:    8081,
					},
				},
			},
			Client: MCPClientConfig{
				Enabled: false,
			},
		},
		AI: AIConfig{
			Enabled:          false,
			Provider:         "local_openai_compatible",
			Endpoint:         "http://localhost:11434/v1",
			Model:            "qwen2.5-coder:14b",
			RequestTimeout:   60 * time.Second,
			MaxContextTokens: 16384,
		},
		Security: SecurityConfig{
			Remediation: RemediationSecurityConfig{
				Enabled:                  false,
				RequireTwoPersonApproval: true,
			},
		},
	}
}

// Load reads a YAML configuration file from disk and applies defaults for unset values.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return defaults if optional file doesn't exist
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config yaml: %w", err)
	}

	return cfg, nil
}
