// SPDX-License-Identifier: MIT
//go:build linux

package capability

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func probePlatform(r *Report) {
	// 1. Kernel release via uname
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err == nil {
		var buf [65]byte
		for i, b := range uname.Release {
			if b == 0 {
				break
			}
			buf[i] = byte(b)
		}
		r.KernelVersion = string(buf[:])
	}

	// 2. BTF support
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err == nil {
		r.HasBTF = true
		r.ProbeStates["btf_vmlinux"] = ProbeState{Status: ProbeActive}
	} else {
		r.ProbeStates["btf_vmlinux"] = ProbeState{Status: ProbeUnsupported, Reason: "no /sys/kernel/btf/vmlinux file"}
	}

	// 3. BPF filesystem
	if _, err := os.Stat("/sys/fs/bpf"); err == nil {
		r.HasBPFFilesystem = true
		r.ProbeStates["bpf_filesystem"] = ProbeState{Status: ProbeActive}
	} else {
		r.ProbeStates["bpf_filesystem"] = ProbeState{Status: ProbeUnsupported, Reason: "no /sys/fs/bpf mount"}
	}

	// 4. unprivileged_bpf_disabled
	if data, err := os.ReadFile("/proc/sys/kernel/unprivileged_bpf_disabled"); err == nil {
		val, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		r.UnprivilegedBPF = val
	}

	// 5. Cgroups version detection
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		r.CgroupVersion = "v2"
	} else if _, err := os.Stat("/sys/fs/cgroup/cpu"); err == nil {
		r.CgroupVersion = "v1"
	} else {
		r.CgroupVersion = "none"
	}

	// 6. Conntrack availability
	if _, err := os.Stat("/proc/net/nf_conntrack"); err == nil {
		r.HasConntrack = true
	} else if _, err := os.Stat("/proc/net/ip_conntrack"); err == nil {
		r.HasConntrack = true
	}

	// 7. TPM 2.0 presence
	if _, err := os.Stat("/dev/tpmrm0"); err == nil {
		r.HasTPM2 = true
	} else if _, err := os.Stat("/dev/tpm0"); err == nil {
		r.HasTPM2 = true
	}

	// 8. Machine-ID health check
	r.MachineIDHealth = "valid"
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		mID := strings.TrimSpace(string(data))
		if mID == "" || mID == "uninitialized" {
			r.MachineIDHealth = "empty"
		}
	} else {
		r.MachineIDHealth = "missing"
	}

	// 9. DMI product UUID
	if data, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		r.DMIReadable = true
	}

	// 10. SELinux Mode
	if data, err := os.ReadFile("/sys/fs/selinux/enforce"); err == nil {
		if strings.TrimSpace(string(data)) == "1" {
			r.SELinuxMode = "enforcing"
		} else {
			r.SELinuxMode = "permissive"
		}
	} else {
		r.SELinuxMode = "disabled"
	}

	// Read SELinux current domain
	if data, err := os.ReadFile("/proc/self/attr/current"); err == nil {
		r.SELinuxDomain = strings.TrimSpace(string(data))
	}

	// Distribution detection
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				r.Distribution = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
				break
			}
		}
	}
}
