//go:build !linux

package ebpf

import (
	"runtime"
)

func detectKernelCapabilities() KernelCapabilities {
	return KernelCapabilities{
		HasBTF:          false,
		HasBPFFilesystem: false,
		KernelRelease:   runtime.GOOS,
		SupportsUprobes: false,
	}
}
