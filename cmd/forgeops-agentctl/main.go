package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"text/tabwriter"
	"time"
)

var (
	ctlVersion = "v1.3.0-dev"
)

func main() {
	socketPath := flag.String("socket", "./state/agent.sock", "Path to supervisor Unix domain socket")
	flag.Usage = printUsage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", *socketPath)
			},
		},
		Timeout: 5 * time.Second,
	}

	command := args[0]
	switch command {
	case "version":
		fmt.Printf("forgeops-agentctl %s\n", ctlVersion)

	case "status":
		handleStatus(client)

	case "policy":
		if len(args) < 2 || args[1] != "show" {
			fmt.Println("Usage: forgeops-agentctl policy show")
			os.Exit(1)
		}
		handlePolicyShow(client)

	case "diagnostics":
		handleDiagnostics(client)

	default:
		fmt.Printf("unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Print(`forgeops-agentctl - Host Agent Inspection & Management CLI

Usage:
  forgeops-agentctl [flags] <command> [arguments...]

Flags:
  -socket string   Path to supervisor Unix domain socket (default: ./state/agent.sock)

Commands:
  version          Show CLI version
  status           Report operational tier, probe states, WAL buffer, and drop counters
  policy show      Display effective policy and conflicts between local ceiling and remote
  diagnostics      Generate host diagnostics report
`)
}

func handleStatus(client *http.Client) {
	resp, err := client.Get("http://unix/status")
	if err != nil {
		fmt.Printf("Error: failed to connect to agent supervisor at unix socket: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var status map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		fmt.Printf("Error decoding status json: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=======================================================")
	fmt.Println(" FORGEOPS-AGENT OPERATIONAL STATUS (v1.3)")
	fmt.Println("=======================================================")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Agent ID:\t%v\n", status["agent_id"])
	fmt.Fprintf(w, "Host Fingerprint:\t%v\n", status["host_fingerprint"])
	fmt.Fprintf(w, "Operational Tier:\t%v (Tier %v)\n", status["tier"], status["tier_number"])
	fmt.Fprintf(w, "Control Plane Connected:\t%v\n", status["connected"])
	fmt.Fprintf(w, "WAL Buffer Size:\t%v bytes\n", status["wal_bytes"])
	fmt.Fprintf(w, "Unacknowledged Records:\t%v\n", status["unacked_messages"])
	fmt.Fprintf(w, "Shed Level:\t%v / 4\n", status["shed_level"])
	fmt.Fprintf(w, "Kill Switch Active:\t%v\n", status["kill_switch"])
	fmt.Fprintf(w, "Certificate Expiry:\t%v\n", status["certificate_expiry"])

	if conflicts, ok := status["policy_conflicts"].([]any); ok && len(conflicts) > 0 {
		fmt.Fprintf(w, "Policy Conflicts:\t%d conflicts detected\n", len(conflicts))
		for _, c := range conflicts {
			fmt.Fprintf(w, "  - %v\n", c)
		}
	} else {
		fmt.Fprintln(w, "Policy Conflicts:\t0 (ceiling respected)")
	}
	w.Flush()
	fmt.Println("=======================================================")
}

func handlePolicyShow(client *http.Client) {
	resp, err := client.Get("http://unix/policy")
	if err != nil {
		fmt.Printf("Error: failed to connect to agent supervisor: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var formatted bytes.Buffer
	_ = json.Indent(&formatted, body, "", "  ")
	fmt.Println(formatted.String())
}

func handleDiagnostics(client *http.Client) {
	fmt.Println("Collecting diagnostics bundle from agent supervisor...")
	resp, err := client.Get("http://unix/diagnostics")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error (%d): %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	filename := resp.Header.Get("X-Bundle-Filename")
	if filename == "" {
		filename = fmt.Sprintf("diagnostics-%d.tar.gz", time.Now().Unix())
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading diagnostics bundle: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		fmt.Printf("Error writing bundle to disk: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Diagnostics Bundle successfully generated!\nSaved to: %s (%d bytes)\n", filename, len(data))
}
