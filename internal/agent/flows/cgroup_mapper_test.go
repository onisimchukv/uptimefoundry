// SPDX-License-Identifier: MIT

package flows

import (
	"testing"
)

func TestCgroupMapper(t *testing.T) {
	mapper := NewCgroupMapper()

	// 1. Kubernetes Pod path
	k8sPath := "/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-poda1b2c3d4-e5f6-7890-abcd-1234567890ab.slice/docker-abcdef123456.scope"
	workload := mapper.ResolveFromPath(k8sPath)
	if workload != "k8s_pod:a1b2c3d4" {
		t.Fatalf("expected k8s_pod:a1b2c3d4, got %s", workload)
	}

	// 2. Systemd service unit
	systemdPath := "/system.slice/nginx.service"
	workload = mapper.ResolveFromPath(systemdPath)
	if workload != "systemd:nginx" {
		t.Fatalf("expected systemd:nginx, got %s", workload)
	}

	// 3. Docker container path
	dockerPath := "/docker/1234567890abcdef12345678"
	workload = mapper.ResolveFromPath(dockerPath)
	if workload != "docker:1234567890ab" {
		t.Fatalf("expected docker:1234567890ab, got %s", workload)
	}
}
