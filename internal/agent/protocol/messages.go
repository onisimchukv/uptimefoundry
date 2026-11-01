// SPDX-License-Identifier: MIT

package protocol

import (
	"time"

	"github.com/onisimchukv/uptimefoundry/internal/core/models"
)

// ProtocolVersion tracks compatibility (supports N and N-1).
const CurrentProtocolVersion = 1

// MessageClass defines priority and buffering semantics.
type MessageClass string

const (
	ClassRegister  MessageClass = "register"
	ClassHeartbeat MessageClass = "heartbeat"
	ClassInventory MessageClass = "inventory"
	ClassFlow      MessageClass = "flow"
	ClassMetric    MessageClass = "metric"
	ClassEvent     MessageClass = "event"
	ClassTask      MessageClass = "task"
	ClassCertRenew MessageClass = "cert_renew"
)

// AgentMessage is the multiplexed envelope sent from the agent to the control plane.
type AgentMessage struct {
	AgentID      string                 `json:"agent_id"`
	StreamEpoch  uint64                 `json:"stream_epoch"`
	Sequence     uint64                 `json:"sequence"`
	Class        MessageClass           `json:"class"`
	Timestamp    time.Time              `json:"timestamp"`
	Register     *RegisterPayload       `json:"register,omitempty"`
	Heartbeat    *HeartbeatPayload      `json:"heartbeat,omitempty"`
	Inventory    *InventoryDeltaPayload `json:"inventory,omitempty"`
	Flow         *FlowDeltaPayload      `json:"flow,omitempty"`
	MetricBatch  *MetricBatchPayload    `json:"metric_batch,omitempty"`
	EventBatch   *EventBatchPayload     `json:"event_batch,omitempty"`
	TaskResult   *TaskResultPayload     `json:"task_result,omitempty"`
	CertRenew    *CertRenewRequest      `json:"cert_renew,omitempty"`
}

// ServerMessage is the multiplexed envelope sent from the control plane to the agent.
type ServerMessage struct {
	AckSequence   uint64              `json:"ack_sequence"`
	AckClass      MessageClass        `json:"ack_class"`
	Policy        *PolicyPayload      `json:"policy,omitempty"`
	Task          *TaskPayload        `json:"task,omitempty"`
	Resync        *ResyncRequest      `json:"resync,omitempty"`
	CertRenewResp *CertRenewResponse  `json:"cert_renew_resp,omitempty"`
}

// RegisterPayload introduces an agent to the control plane during stream establishment.
type RegisterPayload struct {
	ProtocolVersion uint32         `json:"protocol_version"`
	AgentVersion    string         `json:"agent_version"`
	HostFingerprint string         `json:"host_fingerprint"`
	ResumeToken     string         `json:"resume_token,omitempty"`
	Capabilities    map[string]any `json:"capabilities"`
}

// HeartbeatPayload is sent every 15 seconds to report agent health and telemetry accounting.
type HeartbeatPayload struct {
	HealthStatus    string            `json:"health_status"` // "healthy", "degraded", "disconnected", "blocked"
	CurrentTier     int               `json:"current_tier"`  // 1 to 4
	ProbeStates     map[string]string `json:"probe_states"`  // probe -> "active"|"denied"|"unsupported"|"shed"
	DropCounters    map[string]uint64 `json:"drop_counters"` // class -> dropped records count
	WALBytes        int64             `json:"wal_bytes"`
	UnackedMessages int               `json:"unacked_messages"`
	ShedLevel       int               `json:"shed_level"` // 0 to 5
	PolicyConflicts int               `json:"policy_conflicts"`
}

// InventoryDeltaPayload conveys incremental infrastructure state changes.
type InventoryDeltaPayload struct {
	UpsertedResources []*models.Resource     `json:"upserted_resources"`
	Relationships     []*models.Relationship `json:"relationships"`
	DeletedIDs        []string               `json:"deleted_ids,omitempty"`
	IsFullSnapshot    bool                   `json:"is_full_snapshot"`
	IntervalStart     time.Time              `json:"interval_start"`
	IntervalEnd       time.Time              `json:"interval_end"`
}

// FlowEdge represents an aggregated communication edge between two workloads.
type FlowEdge struct {
	SourceWorkload string    `json:"source_workload"`
	DestWorkload   string    `json:"dest_workload"`
	DestPort       int       `json:"dest_port"`
	Protocol       string    `json:"protocol"` // "tcp", "http", "grpc", "sql"
	MinRTTMs       float64   `json:"min_rtt_ms"`
	MeanRTTMs      float64   `json:"mean_rtt_ms"`
	P95RTTMs       float64   `json:"p95_rtt_ms"`
	BytesSent      uint64    `json:"bytes_sent"`
	BytesRecv      uint64    `json:"bytes_recv"`
	Retransmits    uint32    `json:"retransmits"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	ServiceVIP     string    `json:"service_vip,omitempty"` // Captured virtual IP if resolved
}

// FlowDeltaPayload delivers host-side aggregated network flow topology.
type FlowDeltaPayload struct {
	Edges           []FlowEdge `json:"edges"`
	IntervalSeconds int        `json:"interval_seconds"`
	DroppedFlows    uint64     `json:"dropped_flows"`
}

// MetricBatchPayload transfers OTLP-compatible metric data for relay to VictoriaMetrics.
type MetricBatchPayload struct {
	Data []byte `json:"data"` // Serialized OTLP or pre-aggregated histogram records
}

// EventBatchPayload transfers security or journald events with trust labelling.
type EventBatchPayload struct {
	Events []OperationalEvent `json:"events"`
}

// OperationalEvent records a single audit, systemd, or log event.
type OperationalEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Source       string    `json:"source"`
	Severity     string    `json:"severity"`
	Message      string    `json:"message"`
	ContentTrust string    `json:"content_trust"` // Always "untrusted" for external/workload data
}

// PolicyPayload delivers the remote policy from the control plane to the agent.
type PolicyPayload struct {
	Version      uint64         `json:"version"`
	ConfigJSON   string         `json:"config_json"`
	SignedBy     string         `json:"signed_by"`
	Signature    string         `json:"signature"`
	GeneratedAt  time.Time      `json:"generated_at"`
}

// TaskPayload describes a signed operational task from the closed task set.
type TaskPayload struct {
	TaskID      string            `json:"task_id"`
	TaskType    string            `json:"task_type"` // e.g. "collect_diagnostics", "resync_inventory", "set_probe_state", "restart_module"
	Parameters  map[string]string `json:"parameters"`
	ExpiresAt   time.Time         `json:"expires_at"`
	Signature   string            `json:"signature"`
}

// TaskResultPayload conveys the outcome of a signed task.
type TaskResultPayload struct {
	TaskID       string    `json:"task_id"`
	Status       string    `json:"status"` // "success", "failed", "rejected"
	Output       string    `json:"output,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CompletedAt  time.Time `json:"completed_at"`
}

// ResyncRequest is sent by the server when sequence gaps require a full state snapshot.
type ResyncRequest struct {
	Class  MessageClass `json:"class"`
	Reason string       `json:"reason"`
}

// CertRenewRequest requests client certificate renewal over the live mTLS stream.
type CertRenewRequest struct {
	CSR            []byte `json:"csr"` // PEM-encoded Certificate Signing Request
	CurrentCertSN  string `json:"current_cert_sn"`
}

// CertRenewResponse delivers the signed certificate from the control plane CA.
type CertRenewResponse struct {
	CertificateChain [][]byte  `json:"certificate_chain"` // PEM-encoded certificate chain
	ExpiresAt        time.Time `json:"expires_at"`
}
