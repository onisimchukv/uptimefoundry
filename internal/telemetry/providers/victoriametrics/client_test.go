package victoriametrics

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestVictoriaMetricsQueryInstant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/api/v1/query" {
			query := r.URL.Query().Get("query")
			if query != "up" {
				t.Fatalf("expected query 'up', got %s", query)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"status": "success",
				"data": {
					"resultType": "vector",
					"result": [
						{
							"metric": {"__name__": "up", "instance": "srv-1"},
							"value": [1698060000, "1"]
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
	client := New(server.URL, 5*time.Second, 100, logger)

	// Test HealthCheck
	if err := client.HealthCheck(context.Background()); err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}

	// Test Instant Query
	result, err := client.QueryInstant(context.Background(), "up", time.Now())
	if err != nil {
		t.Fatalf("unexpected query error: %v", err)
	}

	if result.ResultType != "vector" {
		t.Fatalf("expected resultType 'vector', got %s", result.ResultType)
	}
	if len(result.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(result.Series))
	}
	if len(result.Series[0].Points) != 1 || result.Series[0].Points[0].Value != 1.0 {
		t.Fatalf("expected point value 1.0, got %v", result.Series[0].Points)
	}
}
