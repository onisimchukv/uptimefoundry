// SPDX-License-Identifier: MIT

package capability

import (
	"context"
	"log/slog"
	"runtime"
)

// Prober inspects the current host environment to determine supported telemetry tiers.
type Prober struct {
	logger *slog.Logger
}

// NewProber creates a capability prober.
func NewProber(logger *slog.Logger) *Prober {
	return &Prober{logger: logger}
}

// Probe evaluates system features and calculates the appropriate operational tier.
func (p *Prober) Probe(ctx context.Context) *Report {
	report := &Report{
		Architecture:     runtime.GOARCH,
		CPUCores:         runtime.NumCPU(),
		ProbeStates:      make(map[string]ProbeState),
	}

	// Platform-specific probing
	probePlatform(report)

	// Determine operational degradation tier based on verified capabilities
	report.Tier = determineTier(report)

	p.logger.Info("capability probe evaluated",
		slog.String("tier", report.Tier.String()),
		slog.Bool("btf", report.HasBTF),
		slog.String("cgroups", report.CgroupVersion),
		slog.String("selinux", report.SELinuxMode),
	)

	return report
}

func determineTier(r *Report) Tier {
	if r.HasBTF && r.HasBPFFilesystem {
		// Kernel supports full eBPF bytecode loading & BTF CO-RE
		return TierFullEBPF
	}

	if r.HasConntrack || r.CgroupVersion != "none" {
		// BPF unsupported or unavailable, but network / cgroup introspection is readable
		return TierProcfs
	}

	// Basic discovery only
	return TierDiscovery
}
