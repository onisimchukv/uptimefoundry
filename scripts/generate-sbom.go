package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
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

	// Compute hashes for binaries if present in bin/
	binFiles := []string{
		"bin/forgeops-agent", "bin/forgeops-agentctl",
		"bin/forgeops", "bin/forgeops-cli",
		"bin/forgeops-agent.exe", "bin/forgeops-agentctl.exe",
		"bin/forgeops.exe", "bin/forgeops-cli.exe",
	}

	for _, binPath := range binFiles {
		if f, err := os.Open(binPath); err == nil {
			h := sha256.New()
			_, _ = io.Copy(h, f)
			f.Close()
			hashHex := hex.EncodeToString(h.Sum(nil))
			components = append(components, Component{
				Type:    "application",
				Name:    binPath,
				Version: "1.3.0",
				SHA256:  hashHex,
			})
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	sbom := SBOM{
		BOMFormat:    "CycloneDX",
		SpecVersion:  "1.5",
		SerialNumber: fmt.Sprintf("urn:uuid:forgeops-sbom-%d", time.Now().Unix()),
		Version:      1,
		Timestamp:    now,
		Metadata: Metadata{
			Timestamp: now,
			Tool: Tool{
				Vendor:  "ForgeOps",
				Name:    "forgeops-sbom-generator",
				Version: "1.3.0",
			},
			Component: Component{
				Type:    "application",
				Name:    "github.com/forgeops/forgeops",
				Version: "1.3.0",
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
