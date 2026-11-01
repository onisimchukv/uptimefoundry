// SPDX-License-Identifier: MIT

package host

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

func TestHostDiscovery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adapter := New(logger)

	if adapter.Name() != "host" {
		t.Fatalf("expected adapter name 'host', got %s", adapter.Name())
	}

	result, err := adapter.Discover(context.Background(), "test-env-01")
	if err != nil {
		t.Fatalf("unexpected discovery error: %v", err)
	}

	if len(result.Resources) == 0 {
		t.Fatal("expected at least 1 resource discovered (host)")
	}

	hostRes := result.Resources[0]
	if hostRes.Kind != models.KindHost {
		t.Fatalf("expected first resource to be KindHost, got %s", hostRes.Kind)
	}
	if hostRes.EnvironmentID != "test-env-01" {
		t.Fatalf("expected environment 'test-env-01', got %s", hostRes.EnvironmentID)
	}
	if hostRes.Status != "online" {
		t.Fatalf("expected status 'online', got %s", hostRes.Status)
	}

	// Verify Network Interfaces (if any non-loopback present on machine)
	for _, res := range result.Resources[1:] {
		if res.Kind != models.KindNetworkInterface {
			t.Fatalf("expected network interface, got %s", res.Kind)
		}
	}
}
