//go:build !linux

package capability

import (
	"runtime"
)

func probePlatform(r *Report) {
	r.KernelVersion = runtime.GOOS
	r.Distribution = runtime.GOOS
	r.HasBTF = false
	r.HasBPFFilesystem = false
	r.SELinuxMode = "disabled"
	r.CgroupVersion = "none"
	r.MachineIDHealth = "valid"
	r.DMIReadable = false
	r.ProbeStates["btf_vmlinux"] = ProbeState{Status: ProbeUnsupported, Reason: "non-linux operating system"}
}
