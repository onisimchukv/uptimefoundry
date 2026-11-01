// SPDX-License-Identifier: MIT

package obi

import (
	"log/slog"
	"sync"

	"github.com/onisimchukv/uptimefoundry/internal/agent/policy"
	"github.com/onisimchukv/uptimefoundry/internal/agent/redaction"
)

// PinnedVersion records the upstream OBI engine release.
const PinnedVersion = "v0.9.0-upstream-obi"

// AggregatedRED tracks request count, error count, and cumulative latency.
type AggregatedRED struct {
	TotalRequests uint64
	TotalErrors   uint64
	TotalDuration float64 // in ms
}

// Wrapper supervises OBI eBPF instrumentation with strict opt-in allowlists and cardinality caps.
type Wrapper struct {
	effectivePolicy *policy.EffectivePolicy
	optedIn         map[string]bool
	seriesCount     int
	seriesLimit     int
	metricsMap      map[string]*AggregatedRED
	overflows       uint64
	logger          *slog.Logger
	mu              sync.RWMutex
}

// NewWrapper creates an OBI wrapper governed by the effective security ceiling.
func NewWrapper(eff *policy.EffectivePolicy, logger *slog.Logger) *Wrapper {
	w := &Wrapper{
		effectivePolicy: eff,
		optedIn:         make(map[string]bool),
		seriesLimit:     5000, // Section 11.4: default 5,000 series cap
		metricsMap:      make(map[string]*AggregatedRED),
		logger:          logger,
	}

	w.refreshOptedIn()
	return w
}

func (w *Wrapper) refreshOptedIn() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.optedIn = make(map[string]bool)
	if w.effectivePolicy == nil {
		return
	}

	// Register opted-in processes from TLS and SQL inspection lists
	for _, p := range w.effectivePolicy.EBPF.TLSInspection.AllowedProcesses {
		if p.Exe != "" {
			w.optedIn[p.Exe] = true
		}
	}
	for _, p := range w.effectivePolicy.EBPF.SQLParsing.AllowedProcesses {
		if p.Exe != "" {
			w.optedIn[p.Exe] = true
		}
	}

	w.logger.Info("obi process opt-in allowlist refreshed",
		slog.Int("opted_in_count", len(w.optedIn)),
	)
}

// IsProcessOptedIn returns true if the executable has been explicitly selected in the effective policy.
func (w *Wrapper) IsProcessOptedIn(exePath string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Default is empty: if not in allowlist, do NOT inspect!
	return w.optedIn[exePath]
}

// RecordHTTPEvent validates opt-in, executes redaction, and aggregates RED metrics under cardinality limits.
func (w *Wrapper) RecordHTTPEvent(exePath, method, rawURI string, status int, durationMs float64, reqBytes, respBytes uint64) (redaction.HTTPSummary, bool) {
	if !w.IsProcessOptedIn(exePath) {
		// Strictly dropped: process not opted in
		return redaction.HTTPSummary{}, false
	}

	// Execute redaction & route templating
	summary := redaction.SanitizeHTTP(method, rawURI, status, durationMs, reqBytes, respBytes)

	// Build fixed-schema metric key: method + route_template + status_class
	metricKey := summary.Method + ":" + summary.RouteTemplate + ":" + summary.StatusClass

	w.mu.Lock()
	defer w.mu.Unlock()

	rec, exists := w.metricsMap[metricKey]
	if !exists {
		if w.seriesCount >= w.seriesLimit {
			// Section 11.4: fold into "other" bucket beyond cap
			w.overflows++
			metricKey = summary.Method + ":/*:" + summary.StatusClass
			rec, exists = w.metricsMap[metricKey]
			if !exists {
				rec = &AggregatedRED{}
				w.metricsMap[metricKey] = rec
			}
		} else {
			rec = &AggregatedRED{}
			w.metricsMap[metricKey] = rec
			w.seriesCount++
		}
	}

	rec.TotalRequests++
	if summary.StatusCode >= 400 {
		rec.TotalErrors++
	}
	rec.TotalDuration += durationMs

	return summary, true
}

// RecordSQLEvent validates opt-in, strips literals, collapses IN clauses, and hashes the statement.
func (w *Wrapper) RecordSQLEvent(exePath, rawSQL string, durationMs float64, isErr bool) (redaction.SQLSummary, bool) {
	if !w.IsProcessOptedIn(exePath) {
		return redaction.SQLSummary{}, false
	}

	summary := redaction.NormalizeSQL(rawSQL)
	summary.IsError = isErr

	return summary, true
}

// FlushSnapshot drains and returns the pre-aggregated metrics.
func (w *Wrapper) FlushSnapshot() map[string]AggregatedRED {
	w.mu.Lock()
	defer w.mu.Unlock()

	snapshot := make(map[string]AggregatedRED, len(w.metricsMap))
	for k, v := range w.metricsMap {
		snapshot[k] = *v
	}

	// Reset counters for next interval
	w.metricsMap = make(map[string]*AggregatedRED)
	w.seriesCount = 0

	return snapshot
}

// Stats returns series count and overflow count.
func (w *Wrapper) Stats() (int, uint64) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.seriesCount, w.overflows
}
