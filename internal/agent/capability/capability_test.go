// SPDX-License-Identifier: MIT

package capability

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestTier_String(t *testing.T) {
	tests := []struct {
		tier     Tier
		expected string
	}{
		{TierFullEBPF, "tier1_full_ebpf"},
		{TierReducedEBPF, "tier2_reduced_ebpf"},
		{TierProcfs, "tier3_procfs_sampling"},
		{TierDiscovery, "tier4_discovery_only"},
		{TierUnknown, "tier_unknown"},
		{Tier(999), "tier_unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.tier.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.tier.String())
			}
		})
	}
}

func TestDetermineTier(t *testing.T) {
	t.Run("BTF_and_BPFFilesystem_Gives_Tier1", func(t *testing.T) {
		r := &Report{
			HasBTF:           true,
			HasBPFFilesystem: true,
		}
		if tier := determineTier(r); tier != TierFullEBPF {
			t.Errorf("expected TierFullEBPF, got %v", tier)
		}
	})

	t.Run("NoBTF_WithConntrack_Gives_Tier3", func(t *testing.T) {
		r := &Report{
			HasBTF:        false,
			HasConntrack:  true,
			CgroupVersion: "none",
		}
		if tier := determineTier(r); tier != TierProcfs {
			t.Errorf("expected TierProcfs, got %v", tier)
		}
	})

	t.Run("NoBTF_WithCgroupV2_Gives_Tier3", func(t *testing.T) {
		r := &Report{
			HasBTF:        false,
			HasConntrack:  false,
			CgroupVersion: "v2",
		}
		if tier := determineTier(r); tier != TierProcfs {
			t.Errorf("expected TierProcfs, got %v", tier)
		}
	})

	t.Run("NoBTF_NoConntrack_NoCgroups_Gives_Tier4", func(t *testing.T) {
		r := &Report{
			HasBTF:        false,
			HasConntrack:  false,
			CgroupVersion: "none",
		}
		if tier := determineTier(r); tier != TierDiscovery {
			t.Errorf("expected TierDiscovery, got %v", tier)
		}
	})
}

func TestProber_Probe(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(logger)

	report := prober.Probe(context.Background())
	if report == nil {
		t.Fatal("expected non-nil report from Probe")
	}

	if report.Architecture == "" {
		t.Error("expected non-empty Architecture")
	}
	if report.CPUCores <= 0 {
		t.Errorf("expected positive CPUCores, got %d", report.CPUCores)
	}
	if report.Tier == TierUnknown {
		t.Error("expected non-unknown Tier from probe")
	}
}
