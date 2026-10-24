package flows

import (
	"time"
)

// FlowKey uniquely identifies an aggregated workload communication stream.
// Ephemeral source ports are deliberately excluded to prevent cardinality explosion.
type FlowKey struct {
	SourceWorkload string `json:"source_workload"`
	DestWorkload   string `json:"dest_workload"`
	DestPort       int    `json:"dest_port"`
	Protocol       string `json:"protocol"` // "tcp", "http", "grpc", "sql"
}

// FlowAccumulator aggregates socket-level measurements over flush intervals.
type FlowAccumulator struct {
	Key         FlowKey
	BytesSent   uint64
	BytesRecv   uint64
	Retransmits uint32
	MinRTTMs    float64
	MaxRTTMs    float64
	SumRTTMs    float64
	CountRTT    uint64
	FirstSeen   time.Time
	LastSeen    time.Time
	ServiceVIP  string // Recorded ClusterIP or Docker VIP if resolved
}

// RawConnection represents an observed socket lifecycle event from eBPF or procfs netlink.
type RawConnection struct {
	SourcePID       uint32
	SourceCgroupID  uint64
	SourceIP        string
	DestIP          string
	DestPort        int
	Protocol        string
	RTTMs           float64
	BytesSent       uint64
	BytesRecv       uint64
	Retransmits     uint32
	Timestamp       time.Time
}

// DNSQueryEvent represents an observed DNS resolution request.
type DNSQueryEvent struct {
	ClientWorkload string
	ResolverIP     string
	QueryName      string
	ResponseCode   int           // 0: NOERROR, 3: NXDOMAIN, 2: SERVFAIL
	Duration       time.Duration
	TimedOut       bool
	Timestamp      time.Time
}

// DNSMetricSummary contains aggregated DNS metrics per workload and domain.
type DNSMetricSummary struct {
	ClientWorkload string        `json:"client_workload"`
	ResolverIP     string        `json:"resolver_ip"`
	DomainSuffix   string        `json:"domain_suffix"`
	TotalQueries   uint64        `json:"total_queries"`
	SuccessCount   uint64        `json:"success_count"`
	NXDomainCount  uint64        `json:"nxdomain_count"`
	ServFailCount  uint64        `json:"servfail_count"`
	TimeoutCount   uint64        `json:"timeout_count"`
	MeanLatencyMs  float64       `json:"mean_latency_ms"`
}
