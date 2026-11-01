// SPDX-License-Identifier: MIT

package flows

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var (
	// Match Kubernetes pod UID from cgroup path
	reK8sPodUID = regexp.MustCompile(`pod([0-9a-fA-F-]+)`)
	// Match systemd service from cgroup path
	reSystemdUnit = regexp.MustCompile(`system\.slice/([a-zA-Z0-9_\-\.]+)\.service`)
	// Match Docker container ID
	reDockerContainer = regexp.MustCompile(`docker[-/]([a-f0-9]{12,64})`)
)

// CgroupMapper maps Linux cgroup paths and IDs to workload identities.
type CgroupMapper struct {
	cache map[uint64]string // cgroupID -> WorkloadName
	mu    sync.RWMutex
}

// NewCgroupMapper creates a new cgroup identity resolver.
func NewCgroupMapper() *CgroupMapper {
	return &CgroupMapper{
		cache: make(map[uint64]string),
	}
}

// RegisterCgroup caches a mapped identity for a cgroup ID.
func (m *CgroupMapper) RegisterCgroup(cgroupID uint64, workload string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[cgroupID] = workload
}

// ResolveFromPath parses cgroup v1 and v2 hierarchical filesystem paths into workload identities.
func (m *CgroupMapper) ResolveFromPath(cgroupPath string) string {
	cgroupPath = strings.TrimSpace(cgroupPath)
	if cgroupPath == "" || cgroupPath == "/" {
		return "host:init"
	}

	// 1. Check for Kubernetes Pod
	if matches := reK8sPodUID.FindStringSubmatch(cgroupPath); len(matches) > 1 {
		podUID := matches[1]
		// Shorten UID for topology clarity
		if len(podUID) > 8 {
			podUID = podUID[:8]
		}
		return fmt.Sprintf("k8s_pod:%s", podUID)
	}

	// 2. Check for systemd service unit
	if matches := reSystemdUnit.FindStringSubmatch(cgroupPath); len(matches) > 1 {
		return fmt.Sprintf("systemd:%s", matches[1])
	}

	// 3. Check for standalone Docker container
	if matches := reDockerContainer.FindStringSubmatch(cgroupPath); len(matches) > 1 {
		cID := matches[1]
		if len(cID) > 12 {
			cID = cID[:12]
		}
		return fmt.Sprintf("docker:%s", cID)
	}

	// Fallback to top-level slice / path name
	parts := strings.Split(strings.Trim(cgroupPath, "/"), "/")
	if len(parts) > 0 {
		return fmt.Sprintf("cgroup:%s", parts[len(parts)-1])
	}

	return "workload:unknown"
}

// ResolveCgroupID retrieves mapped workload identity by kernel cgroup ID.
func (m *CgroupMapper) ResolveCgroupID(cgroupID uint64) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	val, ok := m.cache[cgroupID]
	return val, ok
}
