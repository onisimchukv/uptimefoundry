// SPDX-License-Identifier: MIT
//go:build linux

package host

import (
	"bufio"
	"os"
	"strings"
)

func enrichHostAttributes(attrs map[string]any) {
	// Parse /etc/os-release
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				attrs["os_pretty_name"] = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
			} else if strings.HasPrefix(line, "ID=") {
				attrs["os_distro"] = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			} else if strings.HasPrefix(line, "VERSION_ID=") {
				attrs["os_version"] = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	// Parse /proc/meminfo
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "MemTotal:") {
				attrs["mem_total"] = strings.TrimSpace(strings.TrimPrefix(line, "MemTotal:"))
			} else if strings.HasPrefix(line, "MemAvailable:") {
				attrs["mem_available"] = strings.TrimSpace(strings.TrimPrefix(line, "MemAvailable:"))
			}
		}
	}

	// Parse /proc/cpuinfo for CPU model
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					attrs["cpu_model"] = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}
}
