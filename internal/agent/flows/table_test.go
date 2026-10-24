package flows

import (
	"log/slog"
	"os"
	"testing"
	"time"
)

func TestFlowTableAggregation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	vipResolver := NewVIPResolver()
	vipResolver.RegisterService("k8s_service:payment-svc", "10.96.0.50", 443, []Endpoint{
		{WorkloadName: "pod:payment-api", IP: "10.244.2.10", Port: 443},
	})

	cgroupMapper := NewCgroupMapper()
	cgroupMapper.RegisterCgroup(1001, "pod:frontend-ui")

	table := NewTable(vipResolver, cgroupMapper, logger)

	now := time.Now().UTC()

	// 1. Record Connection 1: Ephemeral port 54122
	table.RecordConnection(RawConnection{
		SourceCgroupID: 1001,
		SourceIP:       "10.244.1.20",
		DestIP:         "10.96.0.50", // VIP
		DestPort:       443,
		Protocol:       "http",
		RTTMs:          1.2,
		BytesSent:      512,
		BytesRecv:      2048,
		Timestamp:      now,
	})

	// 2. Record Connection 2: Different ephemeral port 54198 on same stream
	table.RecordConnection(RawConnection{
		SourceCgroupID: 1001,
		SourceIP:       "10.244.1.20",
		DestIP:         "10.96.0.50", // Same VIP
		DestPort:       443,
		Protocol:       "http",
		RTTMs:          1.8,
		BytesSent:      1024,
		BytesRecv:      4096,
		Timestamp:      now.Add(1 * time.Second),
	})

	// Must aggregate into a single flow edge! (ephemeral ports excluded)
	if table.ActiveFlowCount() != 1 {
		t.Fatalf("expected exactly 1 aggregated flow edge, got %d", table.ActiveFlowCount())
	}

	// 3. Flush Delta
	delta := table.FlushDelta(15)
	if len(delta.Edges) != 1 {
		t.Fatalf("expected 1 edge in flushed delta, got %d", len(delta.Edges))
	}

	edge := delta.Edges[0]
	if edge.SourceWorkload != "pod:frontend-ui" {
		t.Fatalf("expected source pod:frontend-ui, got %s", edge.SourceWorkload)
	}
	if edge.DestWorkload != "pod:payment-api" {
		t.Fatalf("expected resolved target pod:payment-api, got %s", edge.DestWorkload)
	}
	if edge.ServiceVIP != "10.96.0.50" {
		t.Fatalf("expected recorded Service VIP 10.96.0.50, got %s", edge.ServiceVIP)
	}
	if edge.BytesSent != 1536 {
		t.Fatalf("expected 1536 bytes sent, got %d", edge.BytesSent)
	}
	if edge.BytesRecv != 6144 {
		t.Fatalf("expected 6144 bytes recv, got %d", edge.BytesRecv)
	}
	if edge.MeanRTTMs != 1.5 {
		t.Fatalf("expected mean RTT 1.5 ms, got %f", edge.MeanRTTMs)
	}
}
