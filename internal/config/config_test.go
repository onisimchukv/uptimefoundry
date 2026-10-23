package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Server.ListenAddress != "0.0.0.0:8080" {
		t.Fatalf("expected listen address 0.0.0.0:8080, got %s", cfg.Server.ListenAddress)
	}

	if !cfg.Database.Extensions.AGE.Enabled {
		t.Fatal("expected Apache AGE extension enabled by default")
	}

	if !cfg.Database.Extensions.Vector.Enabled {
		t.Fatal("expected pgvector extension enabled by default")
	}

	if cfg.Discovery.Interval != 5*time.Minute {
		t.Fatalf("expected discovery interval 5m, got %v", cfg.Discovery.Interval)
	}

	if cfg.Telemetry.Metrics.Provider != "victoriametrics" {
		t.Fatalf("expected victoriametrics as default provider, got %s", cfg.Telemetry.Metrics.Provider)
	}
}

func TestLoadNonExistent(t *testing.T) {
	cfg, err := Load("non_existent_file.yaml")
	if err != nil {
		t.Fatalf("expected no error for non-existent optional file, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected default config returned")
	}
}
