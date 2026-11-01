// SPDX-License-Identifier: MIT

package flows

import (
	"testing"
	"time"
)

func TestDNSTracker(t *testing.T) {
	// 1. Redaction on by default (fullNamesAllowed = false)
	tracker := NewDNSTracker(false)

	tracker.RecordQuery(DNSQueryEvent{
		ClientWorkload: "pod:billing",
		ResolverIP:     "10.96.0.10",
		QueryName:      "user-auth.staging.company.internal",
		ResponseCode:   0,
		Duration:       2 * time.Millisecond,
		Timestamp:      time.Now(),
	})

	tracker.RecordQuery(DNSQueryEvent{
		ClientWorkload: "pod:billing",
		ResolverIP:     "10.96.0.10",
		QueryName:      "payment-gateway.prod.company.internal",
		ResponseCode:   3, // NXDOMAIN
		Duration:       5 * time.Millisecond,
		Timestamp:      time.Now(),
	})

	metrics := tracker.FlushSnapshot()
	if len(metrics) != 1 {
		t.Fatalf("expected 1 aggregated domain suffix (*.company.internal), got %d", len(metrics))
	}

	m := metrics[0]
	if m.DomainSuffix != "*.company.internal" {
		t.Fatalf("expected domain suffix *.company.internal, got %s", m.DomainSuffix)
	}
	if m.TotalQueries != 2 {
		t.Fatalf("expected 2 queries, got %d", m.TotalQueries)
	}
	if m.SuccessCount != 1 || m.NXDomainCount != 1 {
		t.Fatalf("expected 1 success and 1 NXDomain, got %d and %d", m.SuccessCount, m.NXDomainCount)
	}
}
