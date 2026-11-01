// SPDX-License-Identifier: MIT

package telemetry

import (
	"context"
	"time"
)

// MetricPoint represents a timestamp-value pair in a time-series.
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// MetricSeries represents a single time-series stream.
type MetricSeries struct {
	Metric map[string]string `json:"metric"`
	Points []MetricPoint     `json:"points"`
}

// MetricResult represents the output of instant or range metric queries.
type MetricResult struct {
	ResultType string         `json:"result_type"` // "vector", "matrix", "scalar"
	Series     []MetricSeries `json:"series"`
	QueryTime  time.Duration  `json:"query_time_ms"`
}

// MetricsProvider defines retrieval of numerical time-series metrics.
type MetricsProvider interface {
	QueryInstant(ctx context.Context, query string, at time.Time) (*MetricResult, error)
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*MetricResult, error)
	HealthCheck(ctx context.Context) error
}

// LogEntry represents an operational log line with structured labels.
type LogEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Line      string            `json:"line"`
	Labels    map[string]string `json:"labels,omitempty"`
}

// LogQuery defines parameters for filtering logs.
type LogQuery struct {
	Query     string        `json:"query"` // LogQL expression (e.g. `{app="billing"} |= "error"`)
	Start     time.Time     `json:"start"`
	End       time.Time     `json:"end"`
	Limit     int           `json:"limit"`
	Direction string        `json:"direction"` // "backward" or "forward"
}

// LogResult contains log lines matching a LogQL query.
type LogResult struct {
	Entries   []LogEntry    `json:"entries"`
	Total     int           `json:"total"`
	QueryTime time.Duration `json:"query_time_ms"`
}

// LogsProvider defines retrieval of structured logs.
type LogsProvider interface {
	QueryLogs(ctx context.Context, query LogQuery) (*LogResult, error)
	HealthCheck(ctx context.Context) error
}

// ConnectionEvent represents a socket network interaction captured by eBPF.
type ConnectionEvent struct {
	SourceHost     string        `json:"source_host"`
	SourcePort     int           `json:"source_port"`
	SourceIP       string        `json:"source_ip"`
	DestHost       string        `json:"dest_host"`
	DestPort       int           `json:"dest_port"`
	DestIP         string        `json:"dest_ip"`
	Protocol       string        `json:"protocol"` // "http/1.1", "http/2", "grpc", "postgres", "mysql", "tcp"
	RoundTripTime  time.Duration `json:"rtt"`      // Kernel TCP smoothed RTT
	BytesSent      uint64        `json:"bytes_sent"`
	BytesReceived  uint64        `json:"bytes_received"`
	Retransmits    uint32        `json:"retransmits"`
	Timestamp      time.Time     `json:"timestamp"`
	SourcePID      uint32        `json:"source_pid,omitempty"`
	SourceComm     string        `json:"source_comm,omitempty"`
}
