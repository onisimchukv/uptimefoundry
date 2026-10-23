package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/forgeops/forgeops/internal/core/models"
	"github.com/forgeops/forgeops/internal/graph/traverser"
)

var (
	cliVersion = "v1.2.0-dev"
	client     = &http.Client{Timeout: 15 * time.Second}
)

func main() {
	serverURL := flag.String("server", "http://localhost:8080", "ForgeOps Control Plane API URL")
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]
	switch command {
	case "version":
		fmt.Printf("ForgeOps CLI %s\n", cliVersion)

	case "status":
		handleStatus(*serverURL)

	case "discovery":
		if len(args) < 2 {
			fmt.Println("Usage: forgeops-cli discovery [run|list]")
			os.Exit(1)
		}
		switch args[1] {
		case "run":
			envID := ""
			if len(args) > 2 {
				envID = args[2]
			}
			handleDiscoveryRun(*serverURL, envID)
		case "list":
			handleDiscoveryList(*serverURL)
		default:
			fmt.Printf("unknown discovery action: %s\n", args[1])
		}

	case "resources":
		if len(args) < 2 || args[1] != "list" {
			fmt.Println("Usage: forgeops-cli resources list [kind]")
			os.Exit(1)
		}
		kind := ""
		if len(args) > 2 {
			kind = args[2]
		}
		handleResourcesList(*serverURL, kind)

	case "topology":
		if len(args) < 3 {
			fmt.Println("Usage: forgeops-cli topology [get|blast-radius] <resource_id> [hops]")
			os.Exit(1)
		}
		action := args[1]
		resourceID := args[2]
		hops := "2"
		if len(args) > 3 {
			hops = args[3]
		}

		switch action {
		case "get":
			handleTopologyGet(*serverURL, resourceID, hops)
		case "blast-radius":
			handleBlastRadius(*serverURL, resourceID, hops)
		default:
			fmt.Printf("unknown topology action: %s\n", action)
		}

	default:
		fmt.Printf("unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`ForgeOps CLI - Sovereign SRE & Operations Management

Usage:
  forgeops-cli [flags] <command> [subcommand] [arguments...]

Flags:
  -server string   Control plane endpoint (default: http://localhost:8080)

Commands:
  version                                Print CLI version
  status                                 Show control plane operational status
  discovery run [env_id]                 Trigger immediate infrastructure discovery sweep
  discovery list                         List historical discovery runs
  resources list [kind]                  List discovered assets (optional kind filter)
  topology get <resource_id> [hops]      Display neighborhood topology subgraph
  topology blast-radius <resource_id>    Compute blast-radius severity report for an asset
`)
}

func handleStatus(serverURL string) {
	resp, err := client.Get(serverURL + "/api/v1/status")
	if err != nil {
		fmt.Printf("Error: failed to connect to %s: %v\n", serverURL, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var formatted bytes.Buffer
	_ = json.Indent(&formatted, body, "", "  ")
	fmt.Println(formatted.String())
}

func handleDiscoveryRun(serverURL, envID string) {
	url := fmt.Sprintf("%s/api/v1/discovery/runs?environment_id=%s", serverURL, envID)
	resp, err := client.Post(url, "application/json", nil)
	if err != nil {
		fmt.Printf("Error: discovery run request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var run models.DiscoveryRun
	if err := json.NewDecoder(resp.Body).Decode(&run); err != nil {
		fmt.Printf("Error: failed to decode response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Discovery run %s completed with status: %s\n", run.ID, run.Status)
	fmt.Printf("  Resources discovered:     %d\n", run.ResourcesDiscovered)
	fmt.Printf("  Relationships discovered: %d\n", run.RelationshipsDiscovered)
	if len(run.Errors) > 0 {
		fmt.Printf("  Errors encountered (%d):\n", len(run.Errors))
		for _, e := range run.Errors {
			fmt.Printf("    - %s\n", e)
		}
	}
}

func handleDiscoveryList(serverURL string) {
	resp, err := client.Get(serverURL + "/api/v1/discovery/runs")
	if err != nil {
		fmt.Printf("Error: failed to list runs: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var runs []*models.DiscoveryRun
	_ = json.NewDecoder(resp.Body).Decode(&runs)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tSTATUS\tRESOURCES\tRELATIONSHIPS\tSTARTED AT")
	for _, r := range runs {
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%s\n",
			r.ID[:8], r.Status, r.ResourcesDiscovered, r.RelationshipsDiscovered,
			r.StartedAt.Format("2006-01-02 15:04:05"),
		)
	}
	w.Flush()
}

func handleResourcesList(serverURL, kind string) {
	url := fmt.Sprintf("%s/api/v1/resources?kind=%s", serverURL, kind)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Error: failed to list resources: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var resources []*models.Resource
	_ = json.NewDecoder(resp.Body).Decode(&resources)

	if len(resources) == 0 {
		fmt.Println("No resources discovered yet. Run `forgeops-cli discovery run` to scan.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tKIND\tNAME\tSTATUS\tPROVIDER\tHIERARCHY PATH")
	for _, r := range resources {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			r.ID[:8], r.Kind, r.Name, r.Status, r.Provider, r.HierarchyPath,
		)
	}
	w.Flush()
}

func handleTopologyGet(serverURL, resourceID, hops string) {
	url := fmt.Sprintf("%s/api/v1/topology/subgraph?resource_id=%s&hops=%s", serverURL, resourceID, hops)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Error: failed to fetch topology: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result struct {
		RootID        string                 `json:"root_id"`
		Hops          int                    `json:"hops"`
		Resources     []*models.Resource     `json:"resources"`
		Relationships []*models.Relationship `json:"relationships"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("\nTopology Subgraph for [%s] (%d hops):\n", resourceID, result.Hops)
	fmt.Printf("Total vertices: %d, Total edges: %d\n\n", len(result.Resources), len(result.Relationships))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tSOURCE\t\tRELATION\tTARGET")
	for _, rel := range result.Relationships {
		fmt.Fprintf(w, "EDGE\t%s\t--[%s]-->\t%s\n",
			rel.SourceResourceID[:8], rel.RelationshipType, rel.TargetResourceID[:8],
		)
	}
	w.Flush()
}

func handleBlastRadius(serverURL, resourceID, hops string) {
	url := fmt.Sprintf("%s/api/v1/topology/blast-radius?resource_id=%s&hops=%s", serverURL, resourceID, hops)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Printf("Error: failed to compute blast radius: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var report traverser.BlastRadiusReport
	_ = json.NewDecoder(resp.Body).Decode(&report)

	fmt.Println("\n=======================================================")
	fmt.Printf(" BLAST RADIUS REPORT FOR RESOURCE: %s\n", report.RootResourceID)
	fmt.Println("=======================================================")
	fmt.Printf(" Severity Score:         %.1f / 100.0\n", report.SeverityScore)
	fmt.Printf(" Total Impacted Assets:  %d\n", report.TotalImpacted)
	if len(report.ImpactedNodes) > 0 {
		fmt.Printf(" Impacted Nodes:         %s\n", strings.Join(report.ImpactedNodes, ", "))
	}
	if len(report.ImpactedServices) > 0 {
		fmt.Printf(" Impacted Services:      %s\n", strings.Join(report.ImpactedServices, ", "))
	}
	if len(report.ImpactedWorkloads) > 0 {
		fmt.Printf(" Impacted Workloads:     %s\n", strings.Join(report.ImpactedWorkloads, ", "))
	}
	fmt.Println("=======================================================")
}
