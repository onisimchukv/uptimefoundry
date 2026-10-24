package loki

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/forgeops/forgeops/internal/telemetry"
)

// Client implements telemetry.LogsProvider for Grafana Loki.
type Client struct {
	endpoint   string
	httpClient *http.Client
	timeout    time.Duration
	logger     *slog.Logger
}

var _ telemetry.LogsProvider = (*Client)(nil)

// New creates a new Grafana Loki log provider client.
func New(endpoint string, timeout time.Duration, logger *slog.Logger) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &Client{
		endpoint:   stringsTrimTrailingSlash(endpoint),
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
		logger:     logger,
	}
}

func stringsTrimTrailingSlash(s string) string {
	if len(s) > 0 && s[len(s)-1] == '/' {
		return s[:len(s)-1]
	}
	return s
}

type lokiResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [[nanos_timestamp_str, line_str], ...]
		} `json:"result"`
	} `json:"data"`
}

// QueryLogs executes a LogQL range query.
func (c *Client) QueryLogs(ctx context.Context, query telemetry.LogQuery) (*telemetry.LogResult, error) {
	startTime := time.Now()

	params := url.Values{}
	params.Set("query", query.Query)
	if !query.Start.IsZero() {
		params.Set("start", strconv.FormatInt(query.Start.UnixNano(), 10))
	}
	if !query.End.IsZero() {
		params.Set("end", strconv.FormatInt(query.End.UnixNano(), 10))
	}
	if query.Limit <= 0 {
		query.Limit = 100
	}
	params.Set("limit", strconv.Itoa(query.Limit))
	if query.Direction == "" {
		query.Direction = "backward"
	}
	params.Set("direction", query.Direction)

	reqURL := fmt.Sprintf("%s/loki/api/v1/query_range?%s", c.endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create loki request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute loki query against %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki returned status %d", resp.StatusCode)
	}

	var lResp lokiResponse
	if err := json.NewDecoder(resp.Body).Decode(&lResp); err != nil {
		return nil, fmt.Errorf("decode loki response: %w", err)
	}

	result := &telemetry.LogResult{
		QueryTime: time.Since(startTime),
	}

	for _, stream := range lResp.Data.Result {
		for _, rawVal := range stream.Values {
			if len(rawVal) >= 2 {
				nanos, _ := strconv.ParseInt(rawVal[0], 10, 64)
				entry := telemetry.LogEntry{
					Timestamp: time.Unix(0, nanos).UTC(),
					Line:      rawVal[1],
					Labels:    stream.Stream,
				}
				result.Entries = append(result.Entries, entry)
			}
		}
	}

	result.Total = len(result.Entries)
	return result, nil
}

// HealthCheck verifies Loki readiness.
func (c *Client) HealthCheck(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/ready", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("loki health check connectivity failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki ready endpoint returned status %d", resp.StatusCode)
	}

	return nil
}
