# ForgeOps

**Self-Hosted DevOps and SRE Automation Platform**

ForgeOps is a sovereign, self-hosted reliability engineering platform built in Go. It discovers heterogeneous infrastructure, continuously builds a property graph topology, correlates zero-touch eBPF telemetry with VictoriaMetrics and Loki, grounds incident investigation in internal runbooks using GraphRAG, and exposes native Model Context Protocol (MCP) interfaces to external AI assistants.

---

## Architecture Highlights (v1.2)

* **Unified PostgreSQL Storage:** Single database engine combining Relational ACID metadata, **Apache AGE** (openCypher property graph), and **pgvector** (HNSW vector index for RAG).
* **OpenTelemetry eBPF Instrumentation (OBI):** Zero-code RED metrics (HTTP/gRPC/SQL), user-space TLS inspection via uprobes, and live socket connection tracking (`CONNECTS_TO` edges with RTT).
* **Operational GraphRAG:** Subgraph-filtered vector search over organizational runbooks, SOPs, and post-mortems.
* **Model Context Protocol (MCP):** Dual-mode MCP Server (stdio & SSE) and Client gateway.
* **Controlled Remediation:** Explicit human-in-the-loop (HITL) authorization with cryptographic audit logging.

---

## Directory Structure

```text
forgeops/
├── cmd/
│   ├── forgeops/         # Main Control Plane server
│   ├── forgeops-agent/   # Host agent with eBPF / OBI
│   ├── forgeops-cli/     # Administrative CLI
│   └── forgeops-mcp/     # Dedicated MCP server binary
├── configs/              # Default YAML configurations
├── deploy/               # Docker Compose, systemd, K8s manifests
├── internal/
│   ├── adapters/         # Host (sysfs/procfs), K8s, Docker Swarm adapters
│   ├── config/           # YAML configuration parser & validation
│   ├── core/             # Core domain models and interfaces
│   ├── ebpf/             # OBI kernel & uprobe probe engine
│   ├── graph/            # Apache AGE property graph traverser
│   ├── investigation/    # Incident detection & correlation engine
│   ├── mcp/              # Model Context Protocol server/client
│   ├── rag/              # Markdown chunker & pgvector retrieval
│   ├── remediation/      # Human-in-the-loop remediation guard
│   ├── storage/          # PostgreSQL migrations & pgx connection pool
│   └── telemetry/        # VictoriaMetrics, Prometheus, Loki providers
├── migrations/           # SQL schema (relational, AGE, pgvector)
├── Makefile
├── go.mod
└── go.sum
```

---

## Quick Start (Development)

### 1. Prerequisites
* Go 1.23+ (or 1.27+)
* Docker and Docker Compose

### 2. Start PostgreSQL, VictoriaMetrics, and Loki
```bash
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d
```

### 3. Build and Run the Control Plane
```bash
go build -o bin/forgeops ./cmd/forgeops
./bin/forgeops -config configs/forgeops.example.yaml
```

### 4. Verify Health
```bash
curl -s http://localhost:8080/readyz
curl -s http://localhost:8080/api/v1/status
```
