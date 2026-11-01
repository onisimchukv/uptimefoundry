// SPDX-License-Identifier: MIT
//go:build linux

package ebpf

import (
	"os"
	"syscall"
)

func detectKernelCapabilities() KernelCapabilities {
	var caps KernelCapabilities

	// 1. Check BTF (BPF Type Format)
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err == nil {
		caps.HasBTF = true
	}

	// 2. Check BPF Filesystem
	if _, err := os.Stat("/sys/fs/bpf"); err == nil {
		caps.HasBPFFilesystem = true
	}

	// 3. Check Uprobes support
	if _, err := os.Stat("/sys/kernel/tracing/uprobe_events"); err == nil {
		caps.SupportsUprobes = true
	} else if _, err := os.Stat("/sys/kernel/debug/tracing/uprobe_events"); err == nil {
		caps.SupportsUprobes = true
	}

	// 4. Kernel release string
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err == nil {
		var buf [65]byte
		for i, b := range uname.Release {
			if b == 0 {
				break
			}
			buf[i] = byte(b)
		}
		caps.KernelRelease = string(buf[:])
	}

	return caps
}
