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
	"github.com/forgeops/forgeops/internal/telemetry"
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

	case "metrics":
		if len(args) < 3 || args[1] != "query" {
			fmt.Println("Usage: forgeops-cli metrics query <promql_expression>")
			os.Exit(1)
		}
		handleMetricsQuery(*serverURL, args[2])

	case "logs":
		if len(args) < 3 || args[1] != "query" {
			fmt.Println("Usage: forgeops-cli logs query <logql_expression>")
			os.Exit(1)
		}
		handleLogsQuery(*serverURL, args[2])

	case "incidents":
		if len(args) < 2 {
			fmt.Println("Usage: forgeops-cli incidents [list|create <title> <severity> [primary_resource_id]]")
			os.Exit(1)
		}
		switch args[1] {
		case "list":
			handleIncidentsList(*serverURL)
		case "create":
			if len(args) < 4 {
				fmt.Println("Usage: forgeops-cli incidents create <title> <severity> [primary_resource_id]")
				os.Exit(1)
			}
			title := args[2]
			sev := args[3]
			primaryRes := ""
			if len(args) > 4 {
				primaryRes = args[4]
			}
			handleIncidentCreate(*serverURL, title, sev, primaryRes)
		default:
			fmt.Printf("unknown incidents action: %s\n", args[1])
		}

	case "investigate":
		if len(args) < 2 {
			fmt.Println("Usage: forgeops-cli investigate <incident_id>")
			os.Exit(1)
		}
		handleInvestigate(*serverURL, args[1])

	case "knowledge":
		if len(args) < 3 || args[1] != "search" {
			fmt.Println("Usage: forgeops-cli knowledge search <query>")
			os.Exit(1)
		}
		handleKnowledgeSearch(*serverURL, args[2])

	case "remediations":
		if len(args) < 3 || args[1] != "approve" {
			fmt.Println("Usage: forgeops-cli remediations approve <remediation_id> [approver]")
			os.Exit(1)
		}
		approver := "operator"
		if len(args) > 3 {
			approver = args[3]
		}
		handleRemediationApprove(*serverURL, args[2], approver)

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
  metrics query <promql>                 Query VictoriaMetrics / Prometheus instant metrics
  logs query <logql>                     Query Grafana Loki operational logs
  incidents list                         List active production incidents
  incidents create <title> <sev> [res]   Manually create an incident
  investigate <incident_id>              Run AI-assisted multi-signal investigation
  knowledge search <query>               GraphRAG vector search over runbooks and postmortems
  remediations approve <id> [approver]   Approve a pending human-in-the-loop remediation proposal
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

func handleMetricsQuery(serverURL, promql string) {
	payload, _ := json.Marshal(map[string]any{
		"query": promql,
		"time":  time.Now().UTC(),
	})

	resp, err := client.Post(serverURL+"/api/v1/telemetry/metrics/query", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error: metric query failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result telemetry.MetricResult
	_ = json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("\nMetrics Result (Query Time: %v, Series: %d):\n", result.QueryTime, len(result.Series))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SERIES LABELS\tVALUE\tTIMESTAMP")
	for _, s := range result.Series {
		val := "N/A"
		ts := "N/A"
		if len(s.Points) > 0 {
			val = fmt.Sprintf("%.4f", s.Points[0].Value)
			ts = s.Points[0].Timestamp.Format("15:04:05")
		}
		var labelPairs []string
		for k, v := range s.Metric {
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%q", k, v))
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", strings.Join(labelPairs, ","), val, ts)
	}
	w.Flush()
}

func handleLogsQuery(serverURL, logql string) {
	payload, _ := json.Marshal(telemetry.LogQuery{
		Query: logql,
		Limit: 50,
	})

	resp, err := client.Post(serverURL+"/api/v1/telemetry/logs/query", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error: log query failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var result telemetry.LogResult
	_ = json.NewDecoder(resp.Body).Decode(&result)

	fmt.Printf("\nLogs Result (Query Time: %v, Entries: %d):\n", result.QueryTime, result.Total)
	for _, entry := range result.Entries {
		fmt.Printf("[%s] %s\n", entry.Timestamp.Format("2006-01-02 15:04:05"), entry.Line)
	}
}

func handleIncidentsList(serverURL string) {
	resp, err := client.Get(serverURL + "/api/v1/incidents")
	if err != nil {
		fmt.Printf("Error: failed to fetch incidents: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var incidents []*models.Incident
	_ = json.NewDecoder(resp.Body).Decode(&incidents)

	fmt.Printf("\nIncidents (%d total):\n", len(incidents))
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tSEVERITY\tSTATUS\tPRIMARY RESOURCE\tTITLE\tDETECTED AT")
	for _, inc := range incidents {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			inc.ID, inc.Severity, inc.Status, inc.PrimaryResourceID, inc.Title, inc.DetectedAt.Format("15:04:05"))
	}
	w.Flush()
}

func handleIncidentCreate(serverURL, title, sev, primaryRes string) {
	payload, _ := json.Marshal(models.Incident{
		EnvironmentID:     "00000000-0000-0000-0000-000000000001",
		Title:             title,
		Severity:          models.IncidentSeverity(sev),
		Status:            models.StatusOpen,
		Source:            "operator_cli",
		PrimaryResourceID: primaryRes,
		DetectedAt:        time.Now().UTC(),
	})

	resp, err := client.Post(serverURL+"/api/v1/incidents", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error: failed to create incident: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var created models.Incident
	_ = json.NewDecoder(resp.Body).Decode(&created)
	fmt.Printf("Incident created successfully!\nID: %s\nTitle: %s\nSeverity: %s\n", created.ID, created.Title, created.Severity)
}

func handleInvestigate(serverURL, incidentID string) {
	payload, _ := json.Marshal(map[string]string{"incident_id": incidentID})
	fmt.Printf("Triggering multimodal AI investigation for incident %s...\n", incidentID)

	resp, err := client.Post(serverURL+"/api/v1/incidents/investigate", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error: investigation failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	var inv models.Investigation
	_ = json.NewDecoder(resp.Body).Decode(&inv)

	fmt.Printf("\n=== INVESTIGATION REPORT (ID: %s) ===\n", inv.ID)
	fmt.Printf("Status: %s\nConfidence: %.1f%%\n", inv.Status, inv.Confidence*100)
	fmt.Printf("Summary: %s\n\n", inv.Summary)

	fmt.Printf("Evidence Collected (%d items):\n", len(inv.EvidenceBundle))
	for _, ev := range inv.EvidenceBundle {
		fmt.Printf(" - [%s] (%s): %s\n", ev.ID, ev.EvidenceType, ev.Summary)
	}

	fmt.Printf("\nDiagnostic Hypotheses (%d):\n", len(inv.Hypotheses))
	for _, h := range inv.Hypotheses {
		fmt.Printf("\n#%d %s (Confidence: %.1f%%)\n", h.Rank, h.Title, h.Confidence*100)
		fmt.Printf("   Explanation: %s\n", h.Explanation)
		if len(h.RunbookCitations) > 0 {
			fmt.Printf("   Citations: %s\n", strings.Join(h.RunbookCitations, ", "))
		}
	}
}

func handleKnowledgeSearch(serverURL, query string) {
	payload, _ := json.Marshal(map[string]any{"query": query, "top_k": 3})
	resp, err := client.Post(serverURL+"/api/v1/knowledge/search", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error: search failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var results []*models.KnowledgeSearchResult
	_ = json.NewDecoder(resp.Body).Decode(&results)

	fmt.Printf("\nKnowledge Search Results (%d matches for %q):\n\n", len(results), query)
	for idx, res := range results {
		fmt.Printf("[%d] %s (Similarity: %.2f)\n    Path: %s\n    Preview: %s\n\n",
			idx+1, res.Document.Title, res.Similarity, res.Document.PathOrURL, strings.ReplaceAll(res.Document.Content[:min(len(res.Document.Content), 120)], "\n", " "))
	}
}

func handleRemediationApprove(serverURL, remediationID, approver string) {
	payload, _ := json.Marshal(map[string]string{
		"id":       remediationID,
		"approver": approver,
	})

	resp, err := client.Post(serverURL+"/api/v1/remediations/approve", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Error: approve failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	fmt.Printf("Remediation proposal %s successfully APPROVED by %s.\n", remediationID, approver)
}
