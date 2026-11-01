// SPDX-License-Identifier: MIT

package flows

import (
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/agent/protocol"
)

// Table manages thread-safe aggregation of network socket connections into workload-to-workload topology edges.
type Table struct {
	vipResolver   *VIPResolver
	cgroupMapper  *CgroupMapper
	flows         map[FlowKey]*FlowAccumulator
	maxEdges      int
	edgeTTL       time.Duration
	droppedFlows  uint64
	logger        *slog.Logger
	mu            sync.RWMutex
}

// NewTable creates a new aggregated connection flow table.
func NewTable(vipResolver *VIPResolver, cgroupMapper *CgroupMapper, logger *slog.Logger) *Table {
	if vipResolver == nil {
		vipResolver = NewVIPResolver()
	}
	if cgroupMapper == nil {
		cgroupMapper = NewCgroupMapper()
	}

	return &Table{
		vipResolver:  vipResolver,
		cgroupMapper: cgroupMapper,
		flows:        make(map[FlowKey]*FlowAccumulator),
		maxEdges:     2000,              // Section 11.4: 2,000 distinct destination edges cap per source
		edgeTTL:      10 * time.Minute,  // Section 7.3: 10 minute edge TTL
		logger:       logger,
	}
}

// RecordConnection processes an observed TCP socket event, resolves VIPs and cgroups, and updates the aggregated flow.
func (t *Table) RecordConnection(conn RawConnection) {
	// 1. Resolve Source Workload
	sourceWorkload := "host:unknown"
	if mapped, ok := t.cgroupMapper.ResolveCgroupID(conn.SourceCgroupID); ok {
		sourceWorkload = mapped
	} else if conn.SourceIP != "" {
		sourceWorkload = conn.SourceIP
	}

	// 2. Resolve Destination Virtual IP to backing workload
	destWorkload, serviceVIP, _ := t.vipResolver.Resolve(conn.DestIP, conn.DestPort)

	// 3. Build FlowKey (Notice: Ephemeral source port is excluded!)
	key := FlowKey{
		SourceWorkload: sourceWorkload,
		DestWorkload:   destWorkload,
		DestPort:       conn.DestPort,
		Protocol:       conn.Protocol,
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	accum, exists := t.flows[key]
	if !exists {
		if len(t.flows) >= t.maxEdges {
			t.droppedFlows++
			return // Edge cardinality cap reached
		}

		accum = &FlowAccumulator{
			Key:        key,
			FirstSeen:  conn.Timestamp,
			LastSeen:   conn.Timestamp,
			MinRTTMs:   conn.RTTMs,
			MaxRTTMs:   conn.RTTMs,
			ServiceVIP: serviceVIP,
		}
		t.flows[key] = accum
	}

	// Aggregate metrics
	accum.BytesSent += conn.BytesSent
	accum.BytesRecv += conn.BytesRecv
	accum.Retransmits += conn.Retransmits
	accum.LastSeen = conn.Timestamp

	if conn.RTTMs > 0 {
		if conn.RTTMs < accum.MinRTTMs || accum.MinRTTMs == 0 {
			accum.MinRTTMs = conn.RTTMs
		}
		if conn.RTTMs > accum.MaxRTTMs {
			accum.MaxRTTMs = conn.RTTMs
		}
		accum.SumRTTMs += conn.RTTMs
		accum.CountRTT++
	}
}

// FlushDelta extracts the current delta batch of flow edges and removes expired edges past TTL.
func (t *Table) FlushDelta(intervalSeconds int) protocol.FlowDeltaPayload {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().UTC()
	var edges []protocol.FlowEdge

	for key, accum := range t.flows {
		// Calculate mean RTT
		meanRTT := 0.0
		if accum.CountRTT > 0 {
			meanRTT = accum.SumRTTMs / float64(accum.CountRTT)
		}

		// Estimate P95 RTT (conservative high-percentile approximation)
		p95RTT := math.Max(meanRTT*1.25, accum.MaxRTTMs*0.95)

		edge := protocol.FlowEdge{
			SourceWorkload: key.SourceWorkload,
			DestWorkload:   key.DestWorkload,
			DestPort:       key.DestPort,
			Protocol:       key.Protocol,
			MinRTTMs:       accum.MinRTTMs,
			MeanRTTMs:      meanRTT,
			P95RTTMs:       p95RTT,
			BytesSent:      accum.BytesSent,
			BytesRecv:      accum.BytesRecv,
			Retransmits:    accum.Retransmits,
			FirstSeen:      accum.FirstSeen,
			LastSeen:       accum.LastSeen,
			ServiceVIP:     accum.ServiceVIP,
		}
		edges = append(edges, edge)

		// Reset interval counters
		accum.BytesSent = 0
		accum.BytesRecv = 0
		accum.Retransmits = 0
		accum.SumRTTMs = 0
		accum.CountRTT = 0

		// Check TTL
		if now.Sub(accum.LastSeen) > t.edgeTTL {
			delete(t.flows, key)
		}
	}

	delta := protocol.FlowDeltaPayload{
		Edges:           edges,
		IntervalSeconds: intervalSeconds,
		DroppedFlows:    t.droppedFlows,
	}

	return delta
}

// ActiveFlowCount returns the number of tracked active flows.
func (t *Table) ActiveFlowCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.flows)
}

// SetMaxEdges configures the maximum capacity of the flow table.
func (t *Table) SetMaxEdges(limit int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.maxEdges = limit
}

// PruneToTopN retains only the top N flows sorted by total bytes transferred (Shedding Step 4).
func (t *Table) PruneToTopN(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.flows) <= n {
		return
	}

	type flowItem struct {
		key        FlowKey
		totalBytes uint64
	}

	items := make([]flowItem, 0, len(t.flows))
	for k, accum := range t.flows {
		items = append(items, flowItem{
			key:        k,
			totalBytes: accum.BytesSent + accum.BytesRecv,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].totalBytes > items[j].totalBytes
	})

	// Keep first n items, delete the rest
	pruned := make(map[FlowKey]*FlowAccumulator, n)
	for i := 0; i < n && i < len(items); i++ {
		pruned[items[i].key] = t.flows[items[i].key]
	}
	t.flows = pruned
}
