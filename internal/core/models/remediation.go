// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// RemediationStatus defines the approval and execution state of a remediation proposal.
type RemediationStatus string

const (
	RemediationPendingApproval RemediationStatus = "pending_approval"
	RemediationApproved        RemediationStatus = "approved"
	RemediationExecuted        RemediationStatus = "executed"
	RemediationRejected        RemediationStatus = "rejected"
	RemediationFailed          RemediationStatus = "failed"
)

// Remediation represents an action proposed to resolve or mitigate an incident.
type Remediation struct {
	ID              string            `json:"id"`
	InvestigationID string            `json:"investigation_id"`
	Title           string            `json:"title"`
	ActionType      string            `json:"action_type"` // e.g. "restart_pod", "scale_workload", "flush_cache"
	Parameters      map[string]any    `json:"parameters"`
	Status          RemediationStatus `json:"status"`
	ProposedBy      string            `json:"proposed_by"` // e.g. "ai_investigator", "operator"
	ApprovedBy      string            `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time        `json:"approved_at,omitempty"`
	ExecutedAt      *time.Time        `json:"executed_at,omitempty"`
	ExecutionResult map[string]any    `json:"execution_result,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

// AuditEvent records every privileged, administrative, or mutation action taken in UptimeFoundry.
type AuditEvent struct {
	ID           string         `json:"id"`
	Actor        string         `json:"actor"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   string         `json:"resource_id"`
	Outcome      string         `json:"outcome"` // "success", "failure", "denied"
	Details      map[string]any `json:"details,omitempty"`
	Timestamp    time.Time      `json:"timestamp"`
}
