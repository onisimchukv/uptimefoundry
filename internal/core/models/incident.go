// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// IncidentSeverity defines the criticality level of an operational problem.
type IncidentSeverity string

const (
	SeverityCritical IncidentSeverity = "critical"
	SeverityHigh     IncidentSeverity = "high"
	SeverityMedium   IncidentSeverity = "medium"
	SeverityLow      IncidentSeverity = "low"
)

// IncidentStatus tracks the operational lifecycle of an incident.
type IncidentStatus string

const (
	StatusOpen          IncidentStatus = "open"
	StatusInvestigating IncidentStatus = "investigating"
	StatusMitigated     IncidentStatus = "mitigated"
	StatusResolved      IncidentStatus = "resolved"
	StatusClosed        IncidentStatus = "closed"
)

// Incident represents an operational outage, degradation, or anomaly.
type Incident struct {
	ID                string           `json:"id"`
	EnvironmentID     string           `json:"environment_id"`
	Title             string           `json:"title"`
	Description       string           `json:"description"`
	Severity          IncidentSeverity `json:"severity"`
	Status            IncidentStatus   `json:"status"`
	Source            string           `json:"source"` // e.g. "ebpf_anomaly", "alertmanager", "k8s_event"
	PrimaryResourceID string           `json:"primary_resource_id,omitempty"`
	DetectedAt        time.Time        `json:"detected_at"`
	ResolvedAt        *time.Time       `json:"resolved_at,omitempty"`
	Attributes        map[string]any   `json:"attributes,omitempty"`
}

// InvestigationStatus represents the completion state of an automated investigation.
type InvestigationStatus string

const (
	InvestigationRunning   InvestigationStatus = "running"
	InvestigationCompleted InvestigationStatus = "completed"
	InvestigationFailed    InvestigationStatus = "failed"
)

// EvidenceRecord represents a single piece of collected operational truth.
type EvidenceRecord struct {
	ID                 string         `json:"id"`
	EvidenceType       string         `json:"evidence_type"` // e.g. "metric_timeseries", "log_stream", "trace_span", "ebpf_socket_rtt"
	Provider           string         `json:"provider"`      // e.g. "victoriametrics", "loki", "ebpf"
	ResourceIDs        []string       `json:"resource_ids"`
	TimestampRange     [2]time.Time   `json:"timestamp_range"`
	QueryOrMetadata    string         `json:"query_or_metadata"`
	Summary            string         `json:"summary"`
	IsAnomaly          bool           `json:"is_anomaly"`
	Confidence         float64        `json:"confidence"`
	RawPayloadSnippet  string         `json:"raw_payload_snippet,omitempty"`
}

// Hypothesis represents an AI or rule-derived explanation for an incident.
type Hypothesis struct {
	ID                 string   `json:"id"`
	Rank               int      `json:"rank"`
	Title              string   `json:"title"`
	Explanation        string   `json:"explanation"`
	Confidence         float64  `json:"confidence"`
	SupportingEvidence []string `json:"supporting_evidence_ids"` // References EvidenceRecord.ID
	RunbookCitations   []string `json:"runbook_citations,omitempty"`
}

// Investigation records the execution and findings of an incident analysis.
type Investigation struct {
	ID             string              `json:"id"`
	IncidentID     string              `json:"incident_id"`
	Status         InvestigationStatus `json:"status"`
	StartedAt      time.Time           `json:"started_at"`
	CompletedAt    *time.Time          `json:"completed_at,omitempty"`
	Hypotheses     []Hypothesis        `json:"hypotheses"`
	EvidenceBundle []EvidenceRecord    `json:"evidence_bundle"`
	Summary        string              `json:"summary"`
	Confidence     float64             `json:"confidence"`
	ModelMetadata  map[string]any      `json:"model_metadata,omitempty"`
}
