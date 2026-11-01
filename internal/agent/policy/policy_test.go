// SPDX-License-Identifier: MIT

package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPolicyIntersection(t *testing.T) {
	local := DefaultLocalPolicy()
	local.EBPF.MaxTier = 3
	local.EBPF.TLSInspection.AllowedProcesses = []ProcessSelector{
		{Exe: "/usr/sbin/nginx"},
	}
	local.EBPF.TraceparentInjection = false

	remote := &RemotePolicy{
		Version: 2,
		Modules: map[string]ModuleRule{
			"discovery": {Allowed: true},
			"logs":      {Allowed: true}, // Disallowed by local ceiling!
		},
		EBPF: EBPFRule{
			MaxTier: 1, // Exceeds local ceiling tier 3!
			TLSInspection: ProcessInspection{
				AllowedProcesses: []ProcessSelector{
					{Exe: "/usr/sbin/nginx"},
					{Exe: "/usr/sbin/apache2"}, // Not in local ceiling!
				},
			},
			TraceparentInjection: true, // Disallowed by local ceiling!
		},
		DNS: DNSRule{
			FullNames: true, // Disallowed!
		},
		Limits: ResourceLimits{
			CPUQuotaPercent: 3, // More restrictive -> accepted
		},
	}

	eff := local.Intersect(remote)

	// 1. Logs module should remain false
	if eff.Modules["logs"].Allowed {
		t.Fatal("expected logs module to be disallowed by local ceiling")
	}

	// 2. MaxTier should remain 3
	if eff.EBPF.MaxTier != 3 {
		t.Fatalf("expected effective tier 3, got %d", eff.EBPF.MaxTier)
	}

	// 3. Traceparent injection must remain false
	if eff.EBPF.TraceparentInjection {
		t.Fatal("expected traceparent_injection to be false")
	}

	// 4. TLS inspection must only allow /usr/sbin/nginx, NOT apache2
	if len(eff.EBPF.TLSInspection.AllowedProcesses) != 1 || eff.EBPF.TLSInspection.AllowedProcesses[0].Exe != "/usr/sbin/nginx" {
		t.Fatalf("expected only nginx allowed for TLS, got %v", eff.EBPF.TLSInspection.AllowedProcesses)
	}

	// 5. DNS full names must remain false
	if eff.DNS.FullNames {
		t.Fatal("expected dns.full_names to remain false")
	}

	// 6. CPU Quota should take more restrictive (3%)
	if eff.Limits.CPUQuotaPercent != 3 {
		t.Fatalf("expected CPU quota 3, got %d", eff.Limits.CPUQuotaPercent)
	}

	// 7. Conflicts should be populated
	if len(eff.Conflicts) < 4 {
		t.Fatalf("expected at least 4 conflicts recorded, got %d: %v", len(eff.Conflicts), eff.Conflicts)
	}
}

func TestKillSwitch(t *testing.T) {
	tempDir := t.TempDir()
	killFile := filepath.Join(tempDir, "disable")

	local := DefaultLocalPolicy()
	local.KillSwitch.File = killFile

	// Create kill-switch file
	if err := os.WriteFile(killFile, []byte("shutdown"), 0644); err != nil {
		t.Fatalf("failed to create kill file: %v", err)
	}

	eff := local.Intersect(nil)

	if !eff.KillSwitchActive {
		t.Fatal("expected kill switch to be active")
	}
	if eff.Modules["flows"].Allowed {
		t.Fatal("expected flows module to be forced off by kill switch")
	}
	// Discovery should still be allowed
	if !eff.Modules["discovery"].Allowed {
		t.Fatal("discovery should remain allowed under kill switch")
	}
}
