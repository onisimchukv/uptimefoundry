// SPDX-License-Identifier: MIT

package obi

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/onisimchukv/uptimefoundry/internal/agent/policy"
)

func TestOBIProcessOptIn(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	effPolicy := &policy.EffectivePolicy{
		EBPF: policy.EBPFRule{
			TLSInspection: policy.ProcessInspection{
				AllowedProcesses: []policy.ProcessSelector{
					{Exe: "/usr/sbin/nginx"},
				},
			},
			SQLParsing: policy.ProcessInspection{
				AllowedProcesses: []policy.ProcessSelector{
					{Exe: "/usr/pgsql-16/bin/postgres"},
				},
			},
		},
	}

	w := NewWrapper(effPolicy, logger)

	// 1. Opted-in processes must be allowed
	if !w.IsProcessOptedIn("/usr/sbin/nginx") {
		t.Fatal("expected /usr/sbin/nginx to be opted in")
	}
	if !w.IsProcessOptedIn("/usr/pgsql-16/bin/postgres") {
		t.Fatal("expected /usr/pgsql-16/bin/postgres to be opted in")
	}

	// 2. Non-opted-in processes must be denied
	if w.IsProcessOptedIn("/usr/bin/redis-server") {
		t.Fatal("expected /usr/bin/redis-server to be denied (not in allowlist)")
	}
	if w.IsProcessOptedIn("/usr/sbin/apache2") {
		t.Fatal("expected /usr/sbin/apache2 to be denied")
	}

	// 3. Test HTTP event recording on opted-in vs non-opted-in
	_, ok := w.RecordHTTPEvent("/usr/bin/redis-server", "GET", "/info", 200, 1.0, 10, 10)
	if ok {
		t.Fatal("HTTP event should have been dropped for non-opted-in process")
	}

	summary, ok := w.RecordHTTPEvent("/usr/sbin/nginx", "GET", "/api/v1/users/42", 200, 2.5, 100, 500)
	if !ok {
		t.Fatal("HTTP event should have been recorded for opted-in process")
	}
	if summary.RouteTemplate != "/api/v1/users/:id" {
		t.Fatalf("expected route template /api/v1/users/:id, got %s", summary.RouteTemplate)
	}
}

func TestOBICardinalityCap(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	effPolicy := &policy.EffectivePolicy{
		EBPF: policy.EBPFRule{
			TLSInspection: policy.ProcessInspection{
				AllowedProcesses: []policy.ProcessSelector{
					{Exe: "/usr/sbin/nginx"},
				},
			},
		},
	}

	w := NewWrapper(effPolicy, logger)
	// Lower series limit to 5 for test
	w.seriesLimit = 5

	// Push 10 distinct routes
	for i := 0; i < 10; i++ {
		uri := fmt.Sprintf("/static/route_%d", i)
		w.RecordHTTPEvent("/usr/sbin/nginx", "GET", uri, 200, 1.0, 50, 50)
	}

	seriesCount, overflows := w.Stats()
	if seriesCount > 5 {
		t.Fatalf("series count %d exceeded limit of 5", seriesCount)
	}
	if overflows == 0 {
		t.Fatal("expected overflows to be recorded beyond series cap")
	}

	snapshot := w.FlushSnapshot()
	if len(snapshot) == 0 {
		t.Fatal("expected non-empty snapshot after flush")
	}
	// Verify "GET:/*:2xx" overflow bucket exists
	if _, ok := snapshot["GET:/*:2xx"]; !ok {
		t.Fatal("expected overflow bucket GET:/*:2xx in flushed snapshot")
	}
}
