// SPDX-License-Identifier: MIT

package capability

// Tier represents the operational capability level of the agent.
type Tier int

const (
	TierUnknown     Tier = 0
	TierFullEBPF    Tier = 1 // Full eBPF (BTF, OBI RED metrics, TLS uprobes opt-in, flow table)
	TierReducedEBPF Tier = 2 // Reduced eBPF (flow table & socket counters only)
	TierProcfs      Tier = 3 // Procfs & netlink sampling
	TierDiscovery   Tier = 4 // Discovery only (no network traffic observation)
)

func (t Tier) String() string {
	switch t {
	case TierFullEBPF:
		return "tier1_full_ebpf"
	case TierReducedEBPF:
		return "tier2_reduced_ebpf"
	case TierProcfs:
		return "tier3_procfs_sampling"
	case TierDiscovery:
		return "tier4_discovery_only"
	default:
		return "tier_unknown"
	}
}

// ProbeStatus specifies the state of an individual telemetry sensor.
type ProbeStatus string

const (
	ProbeActive           ProbeStatus = "active"
	ProbeDisabledByPolicy ProbeStatus = "disabled_by_policy"
	ProbeUnsupported      ProbeStatus = "unsupported"
	ProbeDenied           ProbeStatus = "denied"
	ProbeDegraded         ProbeStatus = "degraded"
)

// Report contains comprehensive hardware, kernel, security, and probe state.
type Report struct {
	Tier              Tier                  `json:"tier"`
	KernelVersion     string                `json:"kernel_version"`
	Architecture      string                `json:"architecture"`
	Distribution      string                `json:"distribution"`
	HasBTF            bool                  `json:"has_btf"`
	HasBPFFilesystem  bool                  `json:"has_bpf_filesystem"`
	UnprivilegedBPF   int                   `json:"unprivileged_bpf_disabled"`
	SELinuxMode       string                `json:"selinux_mode"` // "enforcing", "permissive", "disabled"
	SELinuxDomain     string                `json:"selinux_domain"`
	CgroupVersion     string                `json:"cgroup_version"` // "v1", "v2", "hybrid", "none"
	HasConntrack      bool                  `json:"has_conntrack"`
	HasTPM2           bool                  `json:"has_tpm2"`
	MachineIDHealth   string                `json:"machine_id_health"` // "valid", "empty", "template_clone"
	DMIReadable       bool                  `json:"dmi_readable"`
	GrantedCaps       []string              `json:"granted_capabilities"`
	ProbeStates       map[string]ProbeState `json:"probe_states"`
	CPUCores          int                   `json:"cpu_cores"`
	TotalMemoryBytes  uint64                `json:"total_memory_bytes"`
	AvailableBufferMB int64                 `json:"available_buffer_mb"`
}

// ProbeState records detail and reason for a specific probe.
type ProbeState struct {
	Status ProbeStatus `json:"status"`
	Reason string      `json:"reason,omitempty"`
}
