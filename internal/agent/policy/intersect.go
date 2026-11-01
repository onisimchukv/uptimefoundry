// SPDX-License-Identifier: MIT

package policy

import (
	"fmt"
	"os"
)

// Intersect computes EffectivePolicy = LocalPolicy INTERSECT RemotePolicy.
// Remote policy can narrow privileges, but can NEVER widen beyond the local ceiling.
func (local *LocalPolicy) Intersect(remote *RemotePolicy) *EffectivePolicy {
	eff := &EffectivePolicy{
		Version:   local.Version,
		Modules:   make(map[string]ModuleRule),
		Egress:    local.Egress,
		Limits:    local.Limits,
		Conflicts: []string{},
	}

	if remote != nil && remote.Version > 0 {
		eff.Version = int(remote.Version)
	}

	// 1. Check Local Kill-Switch File
	if local.KillSwitch.File != "" {
		if _, err := os.Stat(local.KillSwitch.File); err == nil {
			eff.KillSwitchActive = true
		}
	}

	// 2. Intersect Modules
	for name, localRule := range local.Modules {
		allowed := localRule.Allowed

		if eff.KillSwitchActive && name != "discovery" {
			allowed = false
		} else if remote != nil && remote.Modules != nil {
			if remoteRule, ok := remote.Modules[name]; ok {
				if remoteRule.Allowed && !localRule.Allowed {
					eff.Conflicts = append(eff.Conflicts, fmt.Sprintf("module %q requested by remote but disallowed by local ceiling", name))
				}
				allowed = localRule.Allowed && remoteRule.Allowed
			}
		}

		eff.Modules[name] = ModuleRule{Allowed: allowed}
	}

	// 3. Intersect eBPF Rules
	eff.EBPF.MaxTier = local.EBPF.MaxTier
	if remote != nil && remote.EBPF.MaxTier > 0 {
		// Note: Tier 1 is highest privilege (Full eBPF), Tier 4 is lowest (Discovery only).
		// A lower number represents higher privilege.
		if remote.EBPF.MaxTier < local.EBPF.MaxTier {
			eff.Conflicts = append(eff.Conflicts, fmt.Sprintf("remote requested tier %d which exceeds local ceiling tier %d", remote.EBPF.MaxTier, local.EBPF.MaxTier))
			eff.EBPF.MaxTier = local.EBPF.MaxTier
		} else {
			eff.EBPF.MaxTier = remote.EBPF.MaxTier
		}
	}

	// Traceparent injection
	if remote != nil && remote.EBPF.TraceparentInjection {
		if !local.EBPF.TraceparentInjection {
			eff.Conflicts = append(eff.Conflicts, "traceparent_injection requested by remote but disallowed by local ceiling")
			eff.EBPF.TraceparentInjection = false
		} else {
			eff.EBPF.TraceparentInjection = true
		}
	} else {
		eff.EBPF.TraceparentInjection = false
	}

	// TLS Inspection Process Allowlist Intersection
	eff.EBPF.TLSInspection.AllowedProcesses = intersectProcessSelectors(
		local.EBPF.TLSInspection.AllowedProcesses,
		func() []ProcessSelector {
			if remote != nil {
				return remote.EBPF.TLSInspection.AllowedProcesses
			}
			return nil
		}(),
		&eff.Conflicts,
		"tls_inspection",
	)

	// SQL Parsing Process Allowlist Intersection
	eff.EBPF.SQLParsing.AllowedProcesses = intersectProcessSelectors(
		local.EBPF.SQLParsing.AllowedProcesses,
		func() []ProcessSelector {
			if remote != nil {
				return remote.EBPF.SQLParsing.AllowedProcesses
			}
			return nil
		}(),
		&eff.Conflicts,
		"sql_parsing",
	)

	// 4. Intersect DNS
	eff.DNS.FullNames = local.DNS.FullNames
	if remote != nil && remote.DNS.FullNames {
		if !local.DNS.FullNames {
			eff.Conflicts = append(eff.Conflicts, "dns.full_names requested by remote but disallowed by local ceiling")
			eff.DNS.FullNames = false
		} else {
			eff.DNS.FullNames = true
		}
	}

	// 5. Intersect Tasks
	localTaskSet := make(map[string]bool)
	for _, t := range local.Tasks.AllowedTypes {
		localTaskSet[t] = true
	}

	if remote != nil && len(remote.Tasks.AllowedTypes) > 0 {
		for _, t := range remote.Tasks.AllowedTypes {
			if localTaskSet[t] {
				eff.Tasks.AllowedTypes = append(eff.Tasks.AllowedTypes, t)
			} else {
				eff.Conflicts = append(eff.Conflicts, fmt.Sprintf("task type %q requested by remote but disallowed by local ceiling", t))
			}
		}
	} else {
		eff.Tasks.AllowedTypes = local.Tasks.AllowedTypes
	}

	// 6. Intersect Resource Limits (always choose the more restrictive ceiling)
	if remote != nil && remote.Limits.CPUQuotaPercent > 0 && remote.Limits.CPUQuotaPercent < eff.Limits.CPUQuotaPercent {
		eff.Limits.CPUQuotaPercent = remote.Limits.CPUQuotaPercent
	}
	if remote != nil && remote.Limits.MemoryMaxMB > 0 && remote.Limits.MemoryMaxMB < eff.Limits.MemoryMaxMB {
		eff.Limits.MemoryMaxMB = remote.Limits.MemoryMaxMB
	}
	if remote != nil && remote.Limits.BufferMaxMB > 0 && remote.Limits.BufferMaxMB < eff.Limits.BufferMaxMB {
		eff.Limits.BufferMaxMB = remote.Limits.BufferMaxMB
	}

	return eff
}

func intersectProcessSelectors(local, remote []ProcessSelector, conflicts *[]string, category string) []ProcessSelector {
	localMap := make(map[string]bool)
	for _, p := range local {
		if p.Exe != "" {
			localMap[p.Exe] = true
		}
	}

	var intersected []ProcessSelector
	if remote == nil {
		return intersected
	}

	for _, rp := range remote {
		if localMap[rp.Exe] {
			intersected = append(intersected, rp)
		} else {
			*conflicts = append(*conflicts, fmt.Sprintf("%s process %q requested by remote but absent from local ceiling", category, rp.Exe))
		}
	}

	return intersected
}
