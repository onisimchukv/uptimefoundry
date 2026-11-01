// SPDX-License-Identifier: MIT

package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ProcessSelector specifies an executable or cgroup allowed for deep parsing.
type ProcessSelector struct {
	Exe    string `yaml:"exe" json:"exe"`
	Cgroup string `yaml:"cgroup,omitempty" json:"cgroup,omitempty"`
}

// LocalPolicy defines the non-negotiable security ceiling enforced on the host.
type LocalPolicy struct {
	Version int                    `yaml:"version" json:"version"`
	Modules map[string]ModuleRule  `yaml:"modules" json:"modules"`
	EBPF    EBPFRule               `yaml:"ebpf" json:"ebpf"`
	DNS     DNSRule                `yaml:"dns" json:"dns"`
	Tasks   TaskRule               `yaml:"tasks" json:"tasks"`
	Egress  EgressRule             `yaml:"egress" json:"egress"`
	Limits  ResourceLimits         `yaml:"limits" json:"limits"`
	KillSwitch KillSwitchRule      `yaml:"kill_switch" json:"kill_switch"`
}

type ModuleRule struct {
	Allowed bool `yaml:"allowed" json:"allowed"`
}

type EBPFRule struct {
	MaxTier              int               `yaml:"max_tier" json:"max_tier"` // 1 to 4
	TLSInspection        ProcessInspection `yaml:"tls_inspection" json:"tls_inspection"`
	SQLParsing           ProcessInspection `yaml:"sql_parsing" json:"sql_parsing"`
	TraceparentInjection bool              `yaml:"traceparent_injection" json:"traceparent_injection"`
}

type ProcessInspection struct {
	AllowedProcesses []ProcessSelector `yaml:"allowed_processes" json:"allowed_processes"`
}

type DNSRule struct {
	FullNames bool `yaml:"full_names" json:"full_names"`
}

type TaskRule struct {
	AllowedTypes []string `yaml:"allowed_types" json:"allowed_types"`
}

type EgressEndpoint struct {
	Address string `yaml:"address" json:"address"`
	CAFile  string `yaml:"ca_file" json:"ca_file"`
	PinHash string `yaml:"pin_hash,omitempty" json:"pin_hash,omitempty"`
}

type EgressRule struct {
	Endpoints []EgressEndpoint `yaml:"endpoints" json:"endpoints"`
}

type ResourceLimits struct {
	CPUQuotaPercent int `yaml:"cpu_quota_percent" json:"cpu_quota_percent"` // default 5
	MemoryMaxMB     int `yaml:"memory_max_mb" json:"memory_max_mb"`         // default 256
	BufferMaxMB     int `yaml:"buffer_max_mb" json:"buffer_max_mb"`         // default 512
}

type KillSwitchRule struct {
	File string `yaml:"file" json:"file"` // default /etc/uptimefoundry-agent/disable
}

// RemotePolicy contains the operational configuration pushed by the control plane.
type RemotePolicy struct {
	Version int                    `json:"version"`
	Modules map[string]ModuleRule  `json:"modules"`
	EBPF    EBPFRule               `json:"ebpf"`
	DNS     DNSRule                `json:"dns"`
	Tasks   TaskRule               `json:"tasks"`
	Limits  ResourceLimits         `json:"limits"`
}

// EffectivePolicy is the verified intersection of LocalPolicy and RemotePolicy.
type EffectivePolicy struct {
	Version         int                    `json:"version"`
	Modules         map[string]ModuleRule  `json:"modules"`
	EBPF            EBPFRule               `json:"ebpf"`
	DNS             DNSRule                `json:"dns"`
	Tasks           TaskRule               `json:"tasks"`
	Egress          EgressRule             `json:"egress"`
	Limits          ResourceLimits         `json:"limits"`
	Conflicts       []string               `json:"conflicts"`
	KillSwitchActive bool                  `json:"kill_switch_active"`
}

// DefaultLocalPolicy provides safe out-of-the-box ceiling defaults.
func DefaultLocalPolicy() *LocalPolicy {
	return &LocalPolicy{
		Version: 1,
		Modules: map[string]ModuleRule{
			"discovery":   {Allowed: true},
			"flows":       {Allowed: true},
			"obi":         {Allowed: false},
			"hostmetrics": {Allowed: false},
			"logs":        {Allowed: false},
		},
		EBPF: EBPFRule{
			MaxTier:              3, // Safe Tier 3 default unless explicitly raised
			TraceparentInjection: false,
		},
		DNS: DNSRule{
			FullNames: false,
		},
		Tasks: TaskRule{
			AllowedTypes: []string{
				"collect_diagnostics",
				"resync_inventory",
			},
		},
		Limits: ResourceLimits{
			CPUQuotaPercent: 5,
			MemoryMaxMB:     256,
			BufferMaxMB:     512,
		},
		KillSwitch: KillSwitchRule{
			File: "/etc/uptimefoundry-agent/disable",
		},
	}
}

// LoadLocalPolicy loads the ceiling policy from disk or returns safe defaults if absent.
func LoadLocalPolicy(path string) (*LocalPolicy, error) {
	policy := DefaultLocalPolicy()

	if path == "" {
		return policy, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return policy, nil
		}
		return nil, fmt.Errorf("read local policy: %w", err)
	}

	if err := yaml.Unmarshal(data, policy); err != nil {
		return nil, fmt.Errorf("parse local policy yaml: %w", err)
	}

	return policy, nil
}
