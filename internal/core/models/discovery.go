// SPDX-License-Identifier: MIT

package models

import (
	"time"
)

// DiscoveryStatus represents the state of an infrastructure discovery execution.
type DiscoveryStatus string

const (
	DiscoveryStatusRunning   DiscoveryStatus = "running"
	DiscoveryStatusCompleted DiscoveryStatus = "completed"
	DiscoveryStatusFailed    DiscoveryStatus = "failed"
	DiscoveryStatusCancelled DiscoveryStatus = "cancelled"
)

// DiscoveryRun records the audit and telemetry details of an infrastructure scan.
type DiscoveryRun struct {
	ID                      string          `json:"id"`
	EnvironmentID           string          `json:"environment_id"`
	AdapterType             string          `json:"adapter_type"` // e.g. "host", "kubernetes", "swarm", "all"
	StartedAt               time.Time       `json:"started_at"`
	CompletedAt             *time.Time      `json:"completed_at,omitempty"`
	Status                  DiscoveryStatus `json:"status"`
	ResourcesDiscovered     int             `json:"resources_discovered"`
	RelationshipsDiscovered int             `json:"relationships_discovered"`
	Errors                  []string        `json:"errors,omitempty"`
	Metadata                map[string]any  `json:"metadata,omitempty"`
}
