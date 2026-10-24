package victoriametrics

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

// Client implements telemetry.MetricsProvider for VictoriaMetrics and Prometheus.
type Client struct {
	endpoint   string
	httpClient *http.Client
	maxSeries  int
	timeout    time.Duration
	logger     *slog.Logger
}

var _ telemetry.MetricsProvider = (*Client)(nil)

// New creates a new VictoriaMetrics metrics provider client.
func New(endpoint string, timeout time.Duration, maxSeries int, logger *slog.Logger) *Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	if maxSeries <= 0 {
		maxSeries = 2000
	}
	return &Client{
		endpoint:   stringsTrimTrailingSlash(endpoint),
		httpClient: &http.Client{Timeout: timeout},
		maxSeries:  maxSeries,
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

type prometheusResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
	Data      struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Value  []any             `json:"value,omitempty"`  // [timestamp_float, "value_string"]
			Values [][]any           `json:"values,omitempty"` // [[timestamp_float, "value_string"], ...]
		} `json:"result"`
	} `json:"data"`
}

// QueryInstant executes a point-in-time PromQL / MetricsQL query.
func (c *Client) QueryInstant(ctx context.Context, query string, at time.Time) (*telemetry.MetricResult, error) {
	startTime := time.Now()

	params := url.Values{}
	params.Set("query", query)
	if !at.IsZero() {
		params.Set("time", strconv.FormatInt(at.Unix(), 10))
	}

	reqURL := fmt.Sprintf("%s/api/v1/query?%s", c.endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create instant query request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute instant query against %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("victoriametrics returned status %d", resp.StatusCode)
	}

	var pResp prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}

	if pResp.Status != "success" {
		return nil, fmt.Errorf("victoriametrics error (%s): %s", pResp.ErrorType, pResp.Error)
	}

	result := &telemetry.MetricResult{
		ResultType: pResp.Data.ResultType,
		QueryTime:  time.Since(startTime),
	}

	count := 0
	for _, raw := range pResp.Data.Result {
		if count >= c.maxSeries {
			c.logger.Warn("metric query exceeded max_series limit; results truncated",
				slog.Int("limit", c.maxSeries),
				slog.String("query", query),
			)
			break
		}

		series := telemetry.MetricSeries{
			Metric: raw.Metric,
		}

		if len(raw.Value) >= 2 {
			pt := parsePoint(raw.Value)
			series.Points = append(series.Points, pt)
		}

		result.Series = append(result.Series, series)
		count++
	}

	return result, nil
}

// QueryRange executes an interval range query.
func (c *Client) QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) (*telemetry.MetricResult, error) {
	startTime := time.Now()

	if step <= 0 {
		step = 15 * time.Second
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(start.Unix(), 10))
	params.Set("end", strconv.FormatInt(end.Unix(), 10))
	params.Set("step", fmt.Sprintf("%ds", int(step.Seconds())))

	reqURL := fmt.Sprintf("%s/api/v1/query_range?%s", c.endpoint, params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create range query request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute range query against %s: %w", c.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("victoriametrics returned status %d", resp.StatusCode)
	}

	var pResp prometheusResponse
	if err := json.NewDecoder(resp.Body).Decode(&pResp); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}

	if pResp.Status != "success" {
		return nil, fmt.Errorf("victoriametrics error (%s): %s", pResp.ErrorType, pResp.Error)
	}

	result := &telemetry.MetricResult{
		ResultType: pResp.Data.ResultType,
		QueryTime:  time.Since(startTime),
	}

	count := 0
	for _, raw := range pResp.Data.Result {
		if count >= c.maxSeries {
			c.logger.Warn("metric query exceeded max_series limit; results truncated",
				slog.Int("limit", c.maxSeries),
				slog.String("query", query),
			)
			break
		}

		series := telemetry.MetricSeries{
			Metric: raw.Metric,
		}

		for _, rawPt := range raw.Values {
			pt := parsePoint(rawPt)
			series.Points = append(series.Points, pt)
		}

		result.Series = append(result.Series, series)
		count++
	}

	return result, nil
}

// HealthCheck verifies VictoriaMetrics is responsive.
func (c *Client) HealthCheck(ctx context.Context) error {
	reqURL := fmt.Sprintf("%s/health", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check connectivity failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("victoriametrics health endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

func parsePoint(raw []any) telemetry.MetricPoint {
	var pt telemetry.MetricPoint
	if len(raw) < 2 {
		return pt
	}

	// timestamp can be float64 or int64
	switch v := raw[0].(type) {
	case float64:
		sec := int64(v)
		nsec := int64((v - float64(sec)) * 1e9)
		pt.Timestamp = time.Unix(sec, nsec).UTC()
	case int64:
		pt.Timestamp = time.Unix(v, 0).UTC()
	}

	// value is string in Prometheus JSON format
	if valStr, ok := raw[1].(string); ok {
		val, _ := strconv.ParseFloat(valStr, 64)
		pt.Value = val
	}

	return pt
}
