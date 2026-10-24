package flows

import (
	"strings"
	"sync"
)

// DNSTracker tracks and aggregates DNS query metrics while strictly redacting subdomains unless authorized.
type DNSTracker struct {
	fullNamesAllowed bool
	metrics          map[string]*DNSMetricSummary // "client:resolver:domain" -> Summary
	mu               sync.RWMutex
}

// NewDNSTracker creates a DNS observer.
func NewDNSTracker(fullNamesAllowed bool) *DNSTracker {
	return &DNSTracker{
		fullNamesAllowed: fullNamesAllowed,
		metrics:          make(map[string]*DNSMetricSummary),
	}
}

// ExtractDomainSuffix extracts the registered domain or top 2 domain segments (e.g. "api.internal.example.com" -> "*.example.com").
func (d *DNSTracker) ExtractDomainSuffix(queryName string) string {
	queryName = strings.TrimSuffix(strings.TrimSpace(queryName), ".")
	if queryName == "" {
		return "unknown"
	}

	if d.fullNamesAllowed {
		return queryName
	}

	parts := strings.Split(queryName, ".")
	if len(parts) <= 2 {
		return queryName
	}

	// Keep only the last 2 parts (e.g. *.cluster.local or *.internal)
	return "*." + strings.Join(parts[len(parts)-2:], ".")
}

// RecordQuery aggregates a single DNS query event.
func (d *DNSTracker) RecordQuery(event DNSQueryEvent) {
	domain := d.ExtractDomainSuffix(event.QueryName)
	key := event.ClientWorkload + ":" + event.ResolverIP + ":" + domain

	d.mu.Lock()
	defer d.mu.Unlock()

	summary, exists := d.metrics[key]
	if !exists {
		summary = &DNSMetricSummary{
			ClientWorkload: event.ClientWorkload,
			ResolverIP:     event.ResolverIP,
			DomainSuffix:   domain,
		}
		d.metrics[key] = summary
	}

	summary.TotalQueries++

	latencyMs := float64(event.Duration.Microseconds()) / 1000.0
	// Moving average calculation
	summary.MeanLatencyMs = ((summary.MeanLatencyMs * float64(summary.TotalQueries-1)) + latencyMs) / float64(summary.TotalQueries)

	if event.TimedOut {
		summary.TimeoutCount++
		return
	}

	switch event.ResponseCode {
	case 0:
		summary.SuccessCount++
	case 2:
		summary.ServFailCount++
	case 3:
		summary.NXDomainCount++
	}
}

// FlushSnapshot returns and resets aggregated DNS query metrics.
func (d *DNSTracker) FlushSnapshot() []DNSMetricSummary {
	d.mu.Lock()
	defer d.mu.Unlock()

	var result []DNSMetricSummary
	for _, v := range d.metrics {
		result = append(result, *v)
	}

	d.metrics = make(map[string]*DNSMetricSummary)
	return result
}
