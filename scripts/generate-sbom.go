// SPDX-License-Identifier: MIT
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Component struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
	SHA256  string `json:"sha256,omitempty"`
}

type SBOM struct {
	BOMFormat    string      `json:"bomFormat"`
	SpecVersion  string      `json:"specVersion"`
	SerialNumber string      `json:"serialNumber"`
	Version      int         `json:"version"`
	Timestamp    string      `json:"timestamp"`
	Metadata     Metadata    `json:"metadata"`
	Components   []Component `json:"components"`
}

type Metadata struct {
	Timestamp string    `json:"timestamp"`
	Tool      Tool      `json:"tool"`
	Component Component `json:"component"`
}

type Tool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

func main() {
	releaseVersion := os.Getenv("UPTIMEFOUNDRY_VERSION")
	if releaseVersion == "" {
		releaseVersion = "0.0.0-dev"
	}

	cmd := exec.Command("go", "list", "-m", "-json", "all")
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run go list: %v\n", err)
		os.Exit(1)
	}

	decoder := json.NewDecoder(strings.NewReader(string(out)))
	var components []Component

	for {
		var mod struct {
			Path    string
			Version string
			Dir     string
		}
		if err := decoder.Decode(&mod); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		if mod.Version == "" {
			continue
		}

		components = append(components, Component{
			Type:    "library",
			Name:    mod.Path,
			Version: mod.Version,
			PURL:    fmt.Sprintf("pkg:golang/%s@%s", mod.Path, mod.Version),
		})
	}

	// Include release binaries in the SBOM with their content hashes.
	binFiles, err := filepath.Glob("dist/uptimefoundry_*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find release binaries: %v\n", err)
		os.Exit(1)
	}
	for _, binPath := range binFiles {
		if f, err := os.Open(binPath); err == nil {
			h := sha256.New()
			if _, err := io.Copy(h, f); err != nil {
				_ = f.Close()
				fmt.Fprintf(os.Stderr, "failed to hash %s: %v\n", binPath, err)
				os.Exit(1)
			}
			if err := f.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "failed to close %s: %v\n", binPath, err)
				os.Exit(1)
			}
			hashHex := hex.EncodeToString(h.Sum(nil))
			components = append(components, Component{
				Type:    "application",
				Name:    filepath.Base(binPath),
				Version: releaseVersion,
				SHA256:  hashHex,
			})
		} else {
			fmt.Fprintf(os.Stderr, "failed to open %s: %v\n", binPath, err)
			os.Exit(1)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	sbom := SBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: fmt.Sprintf("urn:uuid:%s", uuid.NewString()),
		Version:      1,
		Timestamp:    now,
		Metadata: Metadata{
			Timestamp: now,
			Tool: Tool{
				Vendor:  "UptimeFoundry",
				Name:    "uptimefoundry-sbom-generator",
				Version: releaseVersion,
			},
			Component: Component{
				Type:    "application",
				Name:    "github.com/onisimchukv/uptimefoundry",
				Version: releaseVersion,
			},
		},
		Components: components,
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sbom); err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal sbom: %v\n", err)
		os.Exit(1)
	}
}
