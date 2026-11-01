// SPDX-License-Identifier: MIT

package loki

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/telemetry"
)

func TestLokiQueryLogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/loki/api/v1/query_range" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": {
					"resultType": "streams",
					"result": [
						{
							"stream": {"app": "billing", "level": "error"},
							"values": [
								["1698060000000000000", "database connection timed out"]
							]
						}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := New(server.URL, 5*time.Second, logger)

	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}

	result, err := client.QueryLogs(context.Background(), telemetry.LogQuery{
		Query: `{app="billing"}`,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected query logs error: %v", err)
	}

	if result.Total != 1 {
		t.Fatalf("expected 1 log entry, got %d", result.Total)
	}
	if result.Entries[0].Line != "database connection timed out" {
		t.Fatalf("expected specific line, got %s", result.Entries[0].Line)
	}
}
