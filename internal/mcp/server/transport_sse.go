// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/mcp/protocol"
)

// RunHTTP launches an HTTP listener exposing SSE (/sse) and JSON-RPC dispatch (/rpc or /message).
func (s *Server) RunHTTP(ctx context.Context, addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/rpc", s.handleHTTPRPC)
	mux.HandleFunc("/message", s.handleHTTPRPC)
	mux.HandleFunc("/sse", s.handleSSE)

	srv := &http.Server{
		Addr:         addr,
		Handler:      s.authMiddleware(mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	s.logger.Info("mcp server listening over http/sse", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken != "" {
			authHeader := r.Header.Get("Authorization")
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token != s.cfg.AuthToken {
				http.Error(w, `{"jsonrpc":"2.0","error":{"code":-32000,"message":"Unauthorized"}}`, http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHTTPRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req protocol.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(protocol.Response{
			JSONRPC: "2.0",
			Error:   &protocol.Error{Code: protocol.ErrCodeParse, Message: err.Error()},
		})
		return
	}

	resp := s.HandleRequest(r.Context(), req)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Emit initial endpoint notification
	_, _ = fmt.Fprintf(w, "event: endpoint\ndata: /rpc\n\n")
	flusher.Flush()

	// Keep alive ticker
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}
