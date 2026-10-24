package actuator

import (
	"errors"
	"time"
)

var (
	ErrSignatureInvalid       = errors.New("actuator: cryptographic signature is invalid")
	ErrTaskExpired            = errors.New("actuator: action task has expired")
	ErrTaskNotYetValid        = errors.New("actuator: action task timestamp is in the future")
	ErrActionDeniedByCeiling  = errors.New("actuator: action type is denied by local policy ceiling")
	ErrUnsupportedActionType  = errors.New("actuator: unsupported action type")
	ErrMissingParameter       = errors.New("actuator: required parameter missing")
)

// SignedActionTask represents a cryptographically authorized remediation instruction.
type SignedActionTask struct {
	TaskID      string            `json:"task_id"`
	ActionType  string            `json:"action_type"`
	Target      string            `json:"target"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	ApprovedBy  string            `json:"approved_by"`
	IssuedAt    time.Time         `json:"issued_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	DryRun      bool              `json:"dry_run"`
	Signature   string            `json:"signature"`
}

// ExecutionResult captures the outcome of an actuator task execution.
type ExecutionResult struct {
	TaskID       string    `json:"task_id"`
	Success      bool      `json:"success"`
	DryRun       bool      `json:"dry_run"`
	Output       string    `json:"output,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ExecutedAt   time.Time `json:"executed_at"`
	DurationMs   int64     `json:"duration_ms"`
}
