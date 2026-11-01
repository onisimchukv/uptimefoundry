// SPDX-License-Identifier: MIT

package actuator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// AuditRecord represents an immutable, hash-chained log entry for every actuator invocation.
type AuditRecord struct {
	Sequence       uint64    `json:"sequence"`
	Timestamp      time.Time `json:"timestamp"`
	TaskID         string    `json:"task_id"`
	ActionType     string    `json:"action_type"`
	Target         string    `json:"target"`
	ApprovedBy     string    `json:"approved_by"`
	DryRun         bool      `json:"dry_run"`
	Outcome        string    `json:"outcome"` // "executed", "dry_run", "rejected", "failed"
	ErrorMessage   string    `json:"error_message,omitempty"`
	PrevRecordHash string    `json:"prev_record_hash"`
	RecordHash     string    `json:"record_hash"`
}

// ComputeHash calculates the cryptographic SHA-256 fingerprint of the audit entry.
func (r *AuditRecord) ComputeHash() string {
	raw := fmt.Sprintf("%d|%s|%s|%s|%s|%s|%t|%s|%s|%s",
		r.Sequence,
		r.Timestamp.UTC().Format(time.RFC3339Nano),
		r.TaskID,
		r.ActionType,
		r.Target,
		r.ApprovedBy,
		r.DryRun,
		r.Outcome,
		r.ErrorMessage,
		r.PrevRecordHash,
	)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// AuditLogger manages an append-only, tamper-evident cryptographic log file.
type AuditLogger struct {
	writer   io.Writer
	lastHash string
	sequence uint64
	mu       sync.Mutex
}

// NewAuditLogger initializes an audit logger writing to the provided io.Writer.
func NewAuditLogger(w io.Writer) *AuditLogger {
	return &AuditLogger{
		writer:   w,
		lastHash: "GENESIS_ROOT_00000000000000000000000000000000000000000000000000000000",
		sequence: 0,
	}
}

// NewFileAuditLogger creates an audit logger that appends to a local log file.
func NewFileAuditLogger(path string) (*AuditLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit log file: %w", err)
	}
	return NewAuditLogger(file), nil
}

// Log records an actuator action outcome into the tamper-evident log.
func (l *AuditLogger) Log(task SignedActionTask, outcome string, errMsg string) (AuditRecord, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sequence++
	rec := AuditRecord{
		Sequence:       l.sequence,
		Timestamp:      time.Now().UTC(),
		TaskID:         task.TaskID,
		ActionType:     task.ActionType,
		Target:         task.Target,
		ApprovedBy:     task.ApprovedBy,
		DryRun:         task.DryRun,
		Outcome:        outcome,
		ErrorMessage:   errMsg,
		PrevRecordHash: l.lastHash,
	}

	rec.RecordHash = rec.ComputeHash()
	l.lastHash = rec.RecordHash

	bytes, err := json.Marshal(rec)
	if err != nil {
		return rec, err
	}

	if _, err := fmt.Fprintf(l.writer, "%s\n", string(bytes)); err != nil {
		return rec, fmt.Errorf("write audit record: %w", err)
	}

	return rec, nil
}

// LastHash returns the current cryptographic tip of the audit chain.
func (l *AuditLogger) LastHash() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.lastHash
}
