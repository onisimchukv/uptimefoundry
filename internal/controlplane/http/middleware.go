// SPDX-License-Identifier: MIT

package http

import (
	"log/slog"
	"net/http"
	"strings"
)

// AuthMiddleware enforces token authentication when authToken is configured.
// When authToken is empty, a warning is logged once and all requests are permitted (development mode).
func AuthMiddleware(authToken string, logger *slog.Logger, next http.Handler) http.Handler {
	if authToken == "" {
		if logger != nil {
			logger.Warn("control plane api authentication is disabled; set security.auth_token or UPTIMEFOUNDRY_AUTH_TOKEN in production")
		}
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Public health, version, and metrics endpoints
		if path == "/healthz" || path == "/readyz" || path == "/api/v1/version" || path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		token := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if strings.HasPrefix(authHeader, "token ") {
			token = strings.TrimPrefix(authHeader, "token ")
		} else {
			token = r.Header.Get("X-API-Key")
		}

		if token == "" || token != authToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized: invalid or missing authentication token"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}
