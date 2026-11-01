// SPDX-License-Identifier: MIT
//go:build !linux

package host

func enrichHostAttributes(attrs map[string]any) {
	attrs["os_distro"] = "generic"
	attrs["collector_mode"] = "standard_cross_platform"
}
