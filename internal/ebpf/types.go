// SPDX-License-Identifier: MIT

package ebpf

import (
	"time"
)

// ProbeStatus defines the operational lifecycle of an eBPF sensor.
type ProbeStatus string

const (
	StatusActive            ProbeStatus = "active"
	StatusKernelUnsupported ProbeStatus = "kernel_unsupported"
	StatusDisabled          ProbeStatus = "disabled"
	StatusError             ProbeStatus = "error"
)

// KernelCapabilities records eBPF features available in the current host kernel.
type KernelCapabilities struct {
	HasBTF          bool   `json:"has_btf"`
	HasBPFFilesystem bool  `json:"has_bpf_fs"`
	KernelRelease   string `json:"kernel_release"`
	SupportsUprobes bool   `json:"supports_uprobes"`
}

// REDMetric represents Rate, Errors, and Duration collected at the kernel boundary.
type REDMetric struct {
	ServiceName string        `json:"service_name"`
	Protocol    string        `json:"protocol"` // "http", "grpc", "postgres", "mysql"
	Method      string        `json:"method"`   // "GET", "POST", "SELECT", etc.
	Status      string        `json:"status"`   // "200", "500", "OK", "ERROR"
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	IsError     bool          `json:"is_error"`
}
