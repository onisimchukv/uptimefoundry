// SPDX-License-Identifier: MIT

package http

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector tracks HTTP RED metrics and operational gauges for Prometheus exposition.
type MetricsCollector struct {
	startTime     time.Time
	requestsTotal sync.Map // key: "method|path|status" -> *uint64
	durationTotal sync.Map // key: "method|path" -> *uint64 (microseconds)
	activeAgents  int64
	messagesTotal uint64
}

// NewMetricsCollector initializes a metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime: time.Now(),
	}
}

// SetActiveAgents updates the active agents gauge.
func (m *MetricsCollector) SetActiveAgents(count int) {
	atomic.StoreInt64(&m.activeAgents, int64(count))
}

// IncrementAgentMessages records received agent ingestion messages.
func (m *MetricsCollector) IncrementAgentMessages() {
	atomic.AddUint64(&m.messagesTotal, 1)
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware records request count and duration and serves /metrics.
func (m *MetricsCollector) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			m.ServeMetrics(w, r)
			return
		}

		start := time.Now()
		rec := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		normPath := normalizePath(r.URL.Path)

		// Record total requests
		key := fmt.Sprintf("%s|%s|%d", r.Method, normPath, rec.statusCode)
		val, _ := m.requestsTotal.LoadOrStore(key, new(uint64))
		atomic.AddUint64(val.(*uint64), 1)

		// Record total duration in microseconds
		durKey := fmt.Sprintf("%s|%s", r.Method, normPath)
		durVal, _ := m.durationTotal.LoadOrStore(durKey, new(uint64))
		atomic.AddUint64(durVal.(*uint64), uint64(duration.Microseconds()))
	})
}

// ServeMetrics outputs the Prometheus text-based metrics format.
func (m *MetricsCollector) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

	var sb strings.Builder

	// HTTP Requests Total
	sb.WriteString("# HELP uptimefoundry_http_requests_total Total HTTP requests processed\n")
	sb.WriteString("# TYPE uptimefoundry_http_requests_total counter\n")
	m.requestsTotal.Range(func(k, v any) bool {
		parts := strings.Split(k.(string), "|")
		if len(parts) == 3 {
			count := atomic.LoadUint64(v.(*uint64))
			sb.WriteString(fmt.Sprintf("uptimefoundry_http_requests_total{method=%q,path=%q,status=%q} %d\n",
				parts[0], parts[1], parts[2], count))
		}
		return true
	})

	// HTTP Request Duration Seconds
	sb.WriteString("# HELP uptimefoundry_http_request_duration_seconds_total Total duration of HTTP requests in seconds\n")
	sb.WriteString("# TYPE uptimefoundry_http_request_duration_seconds_total counter\n")
	m.durationTotal.Range(func(k, v any) bool {
		parts := strings.Split(k.(string), "|")
		if len(parts) == 2 {
			micro := atomic.LoadUint64(v.(*uint64))
			seconds := float64(micro) / 1e6
			sb.WriteString(fmt.Sprintf("uptimefoundry_http_request_duration_seconds_total{method=%q,path=%q} %.6f\n",
				parts[0], parts[1], seconds))
		}
		return true
	})

	// Active Agents
	sb.WriteString("# HELP uptimefoundry_active_agents Number of active enrolled agents\n")
	sb.WriteString("# TYPE uptimefoundry_active_agents gauge\n")
	sb.WriteString(fmt.Sprintf("uptimefoundry_active_agents %d\n", atomic.LoadInt64(&m.activeAgents)))

	// Ingested Agent Messages
	sb.WriteString("# HELP uptimefoundry_agent_messages_total Total messages received from agents\n")
	sb.WriteString("# TYPE uptimefoundry_agent_messages_total counter\n")
	sb.WriteString(fmt.Sprintf("uptimefoundry_agent_messages_total %d\n", atomic.LoadUint64(&m.messagesTotal)))

	// Process Uptime
	uptime := time.Since(m.startTime).Seconds()
	sb.WriteString("# HELP uptimefoundry_uptime_seconds Process uptime in seconds\n")
	sb.WriteString("# TYPE uptimefoundry_uptime_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("uptimefoundry_uptime_seconds %.2f\n", uptime))

	// Go Runtime Metrics
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	sb.WriteString("# HELP uptimefoundry_go_goroutines Number of active goroutines\n")
	sb.WriteString("# TYPE uptimefoundry_go_goroutines gauge\n")
	sb.WriteString(fmt.Sprintf("uptimefoundry_go_goroutines %d\n", runtime.NumGoroutine()))

	sb.WriteString("# HELP uptimefoundry_go_memstats_alloc_bytes Number of heap bytes allocated\n")
	sb.WriteString("# TYPE uptimefoundry_go_memstats_alloc_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("uptimefoundry_go_memstats_alloc_bytes %d\n", mem.Alloc))

	_, _ = w.Write([]byte(sb.String()))
}

func normalizePath(path string) string {
	if strings.HasPrefix(path, "/api/v1/topology/") {
		return "/api/v1/topology/*"
	}
	if strings.HasPrefix(path, "/api/v1/incidents/") {
		return "/api/v1/incidents/*"
	}
	if strings.HasPrefix(path, "/api/v1/remediations/") {
		return "/api/v1/remediations/*"
	}
	if strings.HasPrefix(path, "/mcp/") {
		return "/mcp/*"
	}
	return path
}
