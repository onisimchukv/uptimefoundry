// SPDX-License-Identifier: MIT

package http

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	t.Run("DisabledAuth_AllowsAll", func(t *testing.T) {
		h := AuthMiddleware("", logger, protectedHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("PublicEndpoints_AllowedWithoutAuth", func(t *testing.T) {
		h := AuthMiddleware("super-secret", logger, protectedHandler)
		for _, path := range []string{"/healthz", "/readyz", "/api/v1/version", "/metrics"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 OK for %s, got %d", path, rec.Code)
			}
		}
	})

	t.Run("MissingToken_Returns401", func(t *testing.T) {
		h := AuthMiddleware("super-secret", logger, protectedHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "unauthorized") {
			t.Errorf("expected unauthorized error message, got: %s", rec.Body.String())
		}
	})

	t.Run("InvalidToken_Returns401", func(t *testing.T) {
		h := AuthMiddleware("super-secret", logger, protectedHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
		req.Header.Set("Authorization", "Bearer wrong-token")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("ValidBearerToken_Allowed", func(t *testing.T) {
		h := AuthMiddleware("super-secret", logger, protectedHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
		req.Header.Set("Authorization", "Bearer super-secret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("ValidAPIKeyHeader_Allowed", func(t *testing.T) {
		h := AuthMiddleware("super-secret", logger, protectedHandler)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
		req.Header.Set("X-API-Key", "super-secret")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
	})
}

func TestMetricsMiddleware(t *testing.T) {
	collector := NewMetricsCollector()
	collector.SetActiveAgents(5)
	collector.IncrementAgentMessages()

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	h := collector.MetricsMiddleware(dummyHandler)

	// Issue request to record metrics
	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Query /metrics
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	h.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Fatalf("expected 200 from /metrics, got %d", recMetrics.Code)
	}

	body := recMetrics.Body.String()
	if !strings.Contains(body, "uptimefoundry_http_requests_total") {
		t.Errorf("expected http_requests_total in metrics, got: %s", body)
	}
	if !strings.Contains(body, "uptimefoundry_active_agents 5") {
		t.Errorf("expected active_agents 5 in metrics, got: %s", body)
	}
	if !strings.Contains(body, "uptimefoundry_agent_messages_total 1") {
		t.Errorf("expected agent_messages_total 1 in metrics, got: %s", body)
	}
	if !strings.Contains(body, "uptimefoundry_go_goroutines") {
		t.Errorf("expected go_goroutines in metrics, got: %s", body)
	}
}
