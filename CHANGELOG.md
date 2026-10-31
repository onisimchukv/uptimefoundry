# Changelog

<!-- SPDX-License-Identifier: MIT -->

All notable changes to UptimeFoundry are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased] — v0.1.0

> **Status:** Early implementation — core agent loop functional; eBPF, GraphRAG,
> and MCP are design-complete with initial implementations requiring external
> runtime services (PostgreSQL, VictoriaMetrics, Loki).

### Added

#### Core Platform (Agent Steps A1–A5)
- Agent supervisor with health-check loop, policy ceiling enforcement, and graceful shutdown
- Write-Ahead Log (WAL) for durable agent action journaling
- Identity & RBAC subsystem (JWT-based, scoped tokens)
- Observability Instrumentation (OBI) — OpenTelemetry traces, metrics, structured logs
- Policy engine with hard limits (CPU budget, memory ceiling, action rate)
- Redaction engine — regex + NLP-based PII/secret scrubbing from agent outputs
- Agent control-plane: enrollment, de-enrollment, heartbeat

#### Platform Milestones M6–M8
- Host discovery adapter (network scanner → resource repository)
- RAG indexer with pgvector semantic search
- GraphRAG engine (Apache AGE graph store + fallback in-memory store)
- MCP server: stdio transport + SSE transport (HTTP/2)
- AI investigator: LLM-driven incident analysis with evidence gathering
- Correlator: cross-signal incident correlation (metrics + logs + traces)
- Diagnostics bundle generator (`diagnostics-*.tar.gz`) — redacted
- Watchdog supervisor for agent process lifecycle
- Agent actuator binary for privileged remediation actions

#### Infrastructure
- 5 binaries: `uptimefoundry`, `uptimefoundry-cli`, `uptimefoundry-agent`, `uptimefoundry-agentctl`, `uptimefoundry-agent-actuator`
- PostgreSQL storage layer: discovery repo, incident repo, knowledge repo, resource repo, migrator
- VictoriaMetrics telemetry provider
- Loki log provider

#### Security & Repository Hardening
- Product and repository renamed from **ForgeOps** through **OpsFoundry** to **UptimeFoundry**
- Canonical module and repository URL: `github.com/onisimchukv/uptimefoundry`
- MIT License (`LICENSE`)
- `SECURITY.md` with private vulnerability reporting guidance
- `CONTRIBUTING.md` with DCO sign-off requirement
- `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1)
- Hardened `.gitignore` (`*.pem`, `*.key`, `*.crt`, `*.csr`, `*.pfx`, `*.sock`, state dirs)
- Docker Compose: ports bound to `127.0.0.1`; credentials via env vars
- Dependabot (Go modules + GitHub Actions)
- CI workflow: `go vet`, `golangci-lint` (gosec), `govulncheck`, `go test -race`, CodeQL, gitleaks — all Actions pinned to commit SHAs, read-only `GITHUB_TOKEN`
- Nightly `govulncheck` workflow
- Release workflow with cosign keyless signing and SBOM publication
- `.gitleaks.toml` keeps default secret-detection rules without broad path allowlists
- CODEOWNERS, PR template, issue templates
- Specs moved to `docs/specs/` with ASCII filenames

### Changed
- Command source directories and deployment identifiers now use the UptimeFoundry name
- `go.mod`: added `toolchain go1.27.0`
- `Makefile`: binary output names updated to `uptimefoundry*`; added `vulncheck` and `sbom` targets
- Example configs: TLS warnings, `127.0.0.1` bindings, MCP SSE moved to port 8082

### Fixed
- GraphRAG: corrected field names (`SourceResourceID`, `TargetResourceID`, `RelationshipType`)
- MCP server: fixed `QueryLogs` call signature to use `telemetry.LogQuery{}`
- MCP server test: corrected `telemetry.MetricResult` and `MetricSeries.Points[0].Value` usage
- Executor: removed unused `"time"` import

---

[Unreleased]: https://github.com/onisimchukv/uptimefoundry/compare/HEAD...HEAD
