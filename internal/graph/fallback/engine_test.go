// SPDX-License-Identifier: MIT

package fallback

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

func TestFallbackEngine_Basics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := New(nil, logger)

	// 1. Name
	if engine.Name() != "recursive_cte" {
		t.Errorf("expected engine name 'recursive_cte', got %s", engine.Name())
	}

	// 2. UpsertNode (no-op in fallback)
	res := &models.Resource{ID: "node-1", Name: "srv-1"}
	if err := engine.UpsertNode(context.Background(), res); err != nil {
		t.Errorf("expected UpsertNode to succeed, got %v", err)
	}

	// 3. UpsertEdge (no-op in fallback)
	rel := &models.Relationship{ID: "rel-1", SourceResourceID: "node-1", TargetResourceID: "node-2"}
	if err := engine.UpsertEdge(context.Background(), rel); err != nil {
		t.Errorf("expected UpsertEdge to succeed, got %v", err)
	}

	// 4. QueryCypher should return expected error indicating Apache AGE requirement
	_, err := engine.QueryCypher(context.Background(), "MATCH (n) RETURN n")
	if err == nil {
		t.Fatal("expected error from QueryCypher on fallback engine, got nil")
	}
	if !strings.Contains(err.Error(), "Apache AGE") {
		t.Errorf("expected error message mentioning Apache AGE, got: %s", err.Error())
	}
}
