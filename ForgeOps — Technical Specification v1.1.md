# ForgeOps — Technical Specification v1.1

**Project:** ForgeOps
**Category:** Self-hosted DevOps and SRE automation platform
**Primary language:** Go
**Primary database:** PostgreSQL
**Preferred metrics backend:** VictoriaMetrics
**Telemetry standard:** OpenTelemetry
**Initial deployment target:** RHEL-compatible Linux
**Infrastructure targets:** Bare-metal Linux, Kubernetes, Docker Swarm, and hybrid environments
**Operating principle:** Read-only discovery and investigation by default
**AI strategy:** Provider-neutral, with local inference supported
**Status:** Implementation specification

---

# 1. Executive Summary

## 1.1 Purpose

ForgeOps is a self-hosted infrastructure operations and reliability platform designed to discover infrastructure, build a continuously updated resource graph, collect and correlate operational evidence, investigate incidents, and recommend remediation.

The platform targets organizations operating heterogeneous infrastructure across:

* Bare-metal Linux servers.
* RHEL-compatible distributions.
* Kubernetes clusters.
* Docker Swarm clusters.
* Standalone Docker hosts.
* Private datacentres.
* Hybrid and distributed environments.
* Existing monitoring and observability systems.

ForgeOps should provide capabilities comparable in concept to an AI-assisted DevOps operations agent while remaining deployable in environments where external SaaS dependencies are undesirable or prohibited.

The initial implementation prioritizes reliable discovery and evidence-backed investigation over autonomous infrastructure modification.

## 1.2 Core capabilities

ForgeOps will provide:

1. Infrastructure discovery and inventory.
2. Kubernetes and Docker Swarm discovery.
3. Host-level discovery and health information.
4. Resource relationships and dependency mapping.
5. OpenTelemetry integration for metrics, traces, and logs.
6. VictoriaMetrics integration for metrics ingestion and querying.
7. Prometheus-compatible metrics integration.
8. Loki-compatible log integration.
9. Incident detection and correlation.
10. AI-assisted troubleshooting and root-cause hypotheses.
11. Recommendations supported by inspectable evidence.
12. Controlled remediation with explicit authorization.
13. A REST API and command-line interface.
14. Native Linux deployment and containerized deployment options.
15. Auditing, authentication, authorization, and operational observability.

## 1.3 Architectural principles

ForgeOps must follow these principles:

* **Self-hosted first:** Core functionality must not require a public SaaS control plane.
* **Read-only first:** Discovery and investigation must not modify target infrastructure.
* **Provider-neutral:** Integrations must not require a particular observability or AI vendor.
* **Evidence-backed:** Findings must be traceable to actual observations, queries, events, or logs.
* **Incremental deployment:** Operators must be able to adopt ForgeOps without replacing their existing monitoring stack.
* **Least privilege:** Each integration must receive only the permissions it requires.
* **Explicit remediation:** Changes to production infrastructure require separate authorization.
* **Operational resilience:** Temporary failures in an integration must not bring down the control plane.
* **Resource efficiency:** Components must support deployment on modest on-premises infrastructure.
* **Extensibility:** New infrastructure adapters and telemetry providers must be addable without redesigning the platform.

---

# 2. Scope and Non-Goals

## 2.1 Initial scope

The initial release will support:

| Domain                  | Initial capabilities                                                                      |
| ----------------------- | ----------------------------------------------------------------------------------------- |
| Linux                   | Host discovery, OS information, CPU, memory, filesystems, network interfaces              |
| RHEL-compatible systems | Installation, service management, configuration, and RPM packaging                        |
| Kubernetes              | Clusters, nodes, namespaces, workloads, pods, services, events and resource relationships |
| Docker Swarm            | Managers, workers, services, tasks, networks and volumes where accessible                 |
| Standalone Docker       | Containers, images, networks, volumes and container state                                 |
| PostgreSQL              | ForgeOps metadata and application state                                                   |
| OpenTelemetry           | Integration with existing Collectors and supported OTLP endpoints                         |
| VictoriaMetrics         | Metrics queries, backend health and supported ingestion integrations                      |
| Prometheus              | Prometheus-compatible querying and optional scrape integration                            |
| Loki                    | Log querying and investigation support                                                    |
| AI                      | Evidence summarization, hypotheses, troubleshooting plans and recommendations             |
| Interfaces              | REST API and CLI                                                                          |
| Deployment              | Native Linux services and containerized deployment                                        |

The implementation should prioritize Linux and Kubernetes first, followed by Docker Swarm and broader hybrid support.

## 2.2 Non-goals for the first release

The first release will not attempt to:

* Replace Kubernetes or Docker Swarm.
* Replace an existing observability platform.
* Implement a new time-series database.
* Implement a new distributed tracing backend.
* Implement a new log aggregation engine.
* Require every workload to be instrumented with OpenTelemetry.
* Automatically install telemetry agents on every discovered host.
* Automatically remediate production incidents.
* Require an external AI provider.
* Require Kubernetes to deploy ForgeOps.
* Require a message broker or distributed workflow engine.
* Require an embedded model server.

These capabilities may be introduced later when supported by concrete requirements.

---

# 3. System Architecture

## 3.1 High-level architecture

ForgeOps will initially use a modular-monolith architecture for its control plane, with a separate lightweight agent available where remote or host-local access is necessary.

The control plane should be independently deployable on a RHEL-compatible server or as a containerized application.

```text
                         OPERATORS
                             |
                    Web UI / CLI / API
                             |
                  FORGEOPS CONTROL PLANE
          +--------------------------------------+
          | Authentication and Authorization     |
          | Configuration and Environment Mgmt  |
          | Infrastructure Inventory             |
          | Discovery Orchestration              |
          | Resource Topology                    |
          | Telemetry Query Service               |
          | Incident Correlation                  |
          | Investigation Engine                 |
          | AI Integration                       |
          | Audit and Job Management             |
          +-------------------+------------------+
                              |
                   PostgreSQL Metadata
                              |
          +-------------------+------------------+
          |                   |                  |
     Host Adapters       Kubernetes          Docker Swarm
          |              Adapters              Adapters
          |                   |                  |
          +-------------------+------------------+
                              |
                    Infrastructure Targets
                              |
                   Telemetry Integration
                              |
          +-------------------+------------------+
          |                   |                  |
   OpenTelemetry         Prometheus-         Existing
   Collectors            compatible          exporters
          |               endpoints              |
          +-------------------+------------------+
                              |
                    Observability Backends
          +-------------------+------------------+
          |                   |                  |
    VictoriaMetrics          Loki          Trace Backend
       (metrics)            (logs)          (optional)
          |                   |                  |
          +-------------------+------------------+
                              |
                     Investigation Engine
                              |
                  Evidence and Correlation
                              |
                    AI-Assisted Analysis
                              |
                  Recommendations / Reports
```

This diagram represents logical relationships, not mandatory deployment boundaries. Existing monitoring infrastructure may already provide some or all of the telemetry collection and storage components.

## 3.2 Major components

### Control plane

Responsibilities:

* Expose the REST API.
* Authenticate and authorize requests.
* Manage environments and integrations.
* Orchestrate discovery.
* Store inventory and relationships.
* Coordinate telemetry queries.
* Detect and correlate incidents.
* Execute investigation workflows.
* Manage AI-provider integrations.
* Maintain audit records.
* Schedule and track background jobs.

### Host agent

An optional Go executable for operations that require host-local access or cannot be safely or efficiently performed through remote APIs.

Responsibilities:

* Collect approved host information.
* Report host identity and capabilities.
* Execute explicitly configured discovery tasks.
* Report results to the control plane.
* Provide a secure boundary for host-level access.

The host agent must not automatically gain unrestricted root access.

### Infrastructure adapters

Adapters translate infrastructure-specific APIs and command results into ForgeOps resource models.

Initial adapters:

* Linux host adapter.
* Kubernetes adapter.
* Docker Swarm adapter.
* Standalone Docker adapter.
* Prometheus adapter.
* VictoriaMetrics adapter.
* OpenTelemetry integration.
* Loki adapter.

### Telemetry query service

Provides a consistent internal interface for retrieving:

* Instantaneous metrics.
* Historical time-series data.
* Logs.
* Traces, when a compatible tracing backend exists.
* Telemetry backend health.
* Query execution metadata.

The query service must not require the investigation engine to understand backend-specific API details.

### Investigation engine

Combines infrastructure state, telemetry, events, and resource relationships to identify possible causes of incidents.

The engine must separate:

1. Observation collection.
2. Deterministic correlation.
3. Hypothesis generation.
4. AI-assisted interpretation.
5. Recommendation generation.
6. Optional remediation execution.

### AI integration

AI is an optional analytical component.

It may summarize evidence, suggest queries, generate hypotheses, and recommend actions. It must not be the authoritative source of infrastructure state or telemetry values.

---

# 4. Architecture Decision Records

## ADR-001: Go as the primary language

Go is the primary implementation language.

Reasons:

* Strong support for infrastructure APIs.
* Good Kubernetes and container ecosystem.
* Straightforward deployment as static or self-contained binaries where practical.
* Good concurrency support.
* Suitable for Linux services and lightweight agents.
* Straightforward packaging for RPM-based distributions.

Rust may be considered for individual components if profiling demonstrates a meaningful advantage, but the initial implementation should not mix languages without a concrete requirement.

## ADR-002: Modular monolith for the control plane

The initial control plane will be a modular Go application.

Separate executables are permitted for:

* Control plane.
* Host agent.
* CLI.

The platform will not begin as a collection of independently deployed microservices.

This minimizes operational complexity while retaining clear internal module boundaries.

## ADR-003: PostgreSQL as metadata storage

PostgreSQL will store:

* Environments.
* Resources.
* Relationships.
* Discovery runs.
* Integration configuration metadata.
* Incident records.
* Investigation records.
* Job state.
* Audit events.
* Remediation approval records.

PostgreSQL is not intended to replace VictoriaMetrics or Loki.

High-volume metrics and logs must remain in their designated telemetry backends unless a specific operational requirement justifies otherwise.

## ADR-004: OpenTelemetry as the preferred telemetry standard

OpenTelemetry is the preferred vendor-neutral instrumentation and telemetry integration standard.

ForgeOps will support:

* Integration with existing OpenTelemetry Collectors.
* OTLP-compatible ingestion and export where required.
* OpenTelemetry resource attributes.
* Telemetry correlation with discovered infrastructure.
* Optional Collector deployment and configuration workflows.

ForgeOps must not require every infrastructure component or application to use OpenTelemetry.

Existing Prometheus exporters and other supported telemetry sources remain valid.

## ADR-005: VictoriaMetrics as a first-class metrics backend

VictoriaMetrics is a first-class metrics integration and the preferred metrics backend for a new self-hosted deployment.

ForgeOps must support:

* VictoriaMetrics single-node installations.
* VictoriaMetrics cluster installations.
* Prometheus-compatible query APIs.
* Supported OpenTelemetry metric ingestion.
* Optional `vmagent` integration.
* Backend health checks.
* Query timeouts and result limits.

VictoriaMetrics is not a mandatory dependency. Existing Prometheus-compatible systems must remain supported.

## ADR-006: Separate telemetry interfaces

Metrics, logs, and traces must have separate provider interfaces.

The implementation must not force a metrics provider to implement log or trace retrieval.

The investigation engine will consume a common evidence model that can represent observations from multiple backends.

## ADR-007: Read-only discovery

Initial discovery operations must not modify the target infrastructure.

Write operations, including changes to Collector configurations, must be implemented as separately authorized workflows.

## ADR-008: Provider-neutral AI

AI integration must be independent of the infrastructure and telemetry layers.

The platform must support an abstraction that can later accommodate:

* Locally hosted OpenAI-compatible endpoints.
* Other compatible inference endpoints.
* Approved remote AI providers.
* Deterministic investigation without AI.

The first release must remain useful if AI is disabled.

## ADR-009: Database-backed background jobs

PostgreSQL will initially store discovery and investigation job state.

A separate message broker is not required for the first release.

Job execution must support retries, timeouts, cancellation where safe, and recovery after process restarts.

## ADR-010: Optional MCP integration

Model Context Protocol may be exposed as an additional integration interface.

MCP must not replace the REST API or become a mandatory dependency of the core application.

Any MCP server must apply the same authorization and auditing rules as the REST API.

---

# 5. Repository Structure

The initial repository should use the following structure:

```text
forgeops/
├── cmd/
│   ├── forgeops-server/
│   │   └── main.go
│   ├── forgeops-agent/
│   │   └── main.go
│   └── forgeopsctl/
│       └── main.go
│
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   ├── middleware/
│   │   └── routes/
│   │
│   ├── auth/
│   ├── audit/
│   ├── config/
│   ├── jobs/
│   ├── inventory/
│   ├── discovery/
│   ├── topology/
│   ├── incidents/
│   ├── investigations/
│   ├── remediation/
│   │
│   ├── telemetry/
│   │   ├── model/
│   │   ├── queries/
│   │   ├── correlation/
│   │   ├── health/
│   │   └── providers/
│   │       ├── opentelemetry/
│   │       ├── victoriametrics/
│   │       ├── prometheus/
│   │       ├── loki/
│   │       └── traces/
│   │
│   ├── adapters/
│   │   ├── host/
│   │   ├── kubernetes/
│   │   ├── docker/
│   │   └── swarm/
│   │
│   ├── storage/
│   │   └── postgres/
│   │
│   └── ai/
│       ├── provider/
│       ├── prompts/
│       └── investigation/
│
├── migrations/
├── api/
│   └── openapi.yaml
│
├── configs/
│   ├── forgeops.example.yaml
│   └── telemetry.example.yaml
│
├── deploy/
│   ├── systemd/
│   ├── rpm/
│   ├── containers/
│   ├── kubernetes/
│   └── otel-collector/
│
├── docs/
│   ├── architecture/
│   ├── security/
│   ├── operations/
│   └── integrations/
│
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── fixtures/
│   └── acceptance/
│
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

The repository must maintain strict boundaries between infrastructure discovery, telemetry retrieval, storage, and investigation logic.

Provider-specific dependencies must not leak into domain models.

---

# 6. Core Resource Model

## 6.1 Resource entity

Every discovered infrastructure object must have a common representation.

Conceptual Go model:

```go
type Resource struct {
    ID          string
    Environment string

    Kind        string
    Provider    string
    ExternalID  string

    Name        string
    Namespace   string

    Labels      map[string]string
    Attributes  map[string]any

    Status      string

    FirstSeenAt time.Time
    LastSeenAt  time.Time

    Source      string
}
```

The actual implementation should use typed identifiers and timestamps where appropriate.

`ExternalID` identifies the resource in its originating system. `ID` is the stable ForgeOps identifier.

Resource IDs must be generated deterministically where reliable identity information is available. Resources without sufficient identity information must not be merged solely because their names match.

## 6.2 Initial resource kinds

### Infrastructure

* Environment.
* Site.
* Host.
* Network interface.
* Filesystem.
* Disk device.

### Containers

* Docker host.
* Docker container.
* Docker image.
* Docker network.
* Docker volume.

### Kubernetes

* Cluster.
* Node.
* Namespace.
* Pod.
* Deployment.
* StatefulSet.
* DaemonSet.
* ReplicaSet.
* Service.
* Ingress.
* PersistentVolume.
* PersistentVolumeClaim.
* ConfigMap.
* Secret metadata.
* Kubernetes event.

Secret values must never be collected as part of ordinary discovery.

### Docker Swarm

* Swarm cluster.
* Manager.
* Worker.
* Service.
* Task.
* Container.
* Overlay network.
* Volume.

### Observability

* Telemetry endpoint.
* OpenTelemetry Collector.
* Metrics backend.
* Log backend.
* Trace backend.
* Scrape target, where applicable.
* Telemetry integration.

Observability resources should reference infrastructure resources where identity can be established reliably.

## 6.3 Resource relationships

The topology engine must support relationships such as:

* `CONTAINS`
* `RUNS_ON`
* `DEPENDS_ON`
* `CONNECTS_TO`
* `EXPOSES`
* `MOUNTS`
* `SCHEDULED_ON`
* `MONITORED_BY`
* `EMITS_TELEMETRY_TO`
* `OBSERVED_BY`
* `RELATED_TO`

Relationships must record their source and observation timestamps.

The topology engine must distinguish confirmed relationships from inferred relationships.

For example, a pod's node assignment is an observed relationship, while an application-to-database dependency inferred from traces may be probabilistic.

---

# 7. Infrastructure Discovery

## 7.1 Discovery lifecycle

Every discovery operation must follow this lifecycle:

1. Create a discovery run.
2. Validate credentials and connectivity.
3. Determine supported discovery capabilities.
4. Retrieve resources.
5. Normalize resource identities.
6. Upsert resources and relationships.
7. Record discovery errors.
8. Mark previously discovered resources as stale where appropriate.
9. Complete the run.
10. Expose results through the API.

Discovery must support partial success.

Failure to discover one namespace or host must not invalidate the entire run.

## 7.2 Linux host discovery

Initial host discovery must retrieve, where permissions allow:

* Hostname and machine identity.
* OS distribution and version.
* Kernel version.
* CPU topology.
* Memory capacity and available memory.
* Filesystem capacity and utilization.
* Disk device inventory.
* Network interface inventory.
* IP addresses.
* Service status when explicitly enabled.
* Host uptime.
* Optional approved system metrics.

Discovery mechanisms may include:

* A lightweight ForgeOps agent.
* A supported remote execution mechanism.
* Existing infrastructure APIs.
* Existing telemetry endpoints.

The control plane must not require unrestricted SSH or root access for all discovery operations.

## 7.3 Kubernetes discovery

The Kubernetes adapter must support:

* API server connectivity checks.
* Cluster identification.
* Node inventory.
* Namespace inventory.
* Workload discovery.
* Pod status and scheduling information.
* Service and ingress inventory.
* Storage relationships.
* Relevant events.
* Resource limits and requests.
* Restart counts.
* Readiness and availability information.

The initial adapter should use the Kubernetes API and a read-only service account wherever practical.

Permissions must be configurable by capability. The application must not require cluster-admin permissions merely to perform ordinary discovery.

## 7.4 Docker Swarm discovery

The Swarm adapter must support:

* Cluster identification.
* Manager and worker inventory.
* Service specifications.
* Task state.
* Container status.
* Replica counts.
* Network relationships.
* Volume references.
* Placement constraints where accessible.

Docker socket access must be treated as highly privileged.

The preferred deployment should use the minimum practical access model and avoid mounting a writable Docker socket into the control plane.

## 7.5 Discovery scheduling

The platform must support:

* Manual discovery.
* Scheduled discovery.
* Configurable intervals.
* Per-environment scheduling.
* Retry policies.
* Execution timeouts.
* Cancellation where safe.
* Run history.
* Error reporting.

Initial scheduling can be implemented using database-backed jobs.

---

# 8. Telemetry Architecture

## 8.1 Objectives

ForgeOps telemetry integration must provide a consistent way to collect or query operational evidence without forcing operators to replace their current monitoring infrastructure.

The telemetry subsystem must support:

* Metrics.
* Logs.
* Traces.
* Resource attributes.
* Infrastructure events.
* Backend health.
* Historical queries.
* Correlation with discovered topology.

ForgeOps should distinguish **telemetry ingestion** from **telemetry querying**.

Ingestion sends or receives telemetry data. Querying retrieves existing data from a backend. These capabilities may be implemented by different providers.

## 8.2 Telemetry collection patterns

The platform must support four operational patterns.

### Pattern A: Existing observability stack

ForgeOps queries existing metrics and log backends.

It does not install duplicate agents or change existing collection settings.

This should be the preferred initial integration mode.

### Pattern B: Existing OpenTelemetry Collectors

ForgeOps integrates with an existing Collector deployment and uses configured OTLP endpoints or backend integrations.

Collector administration is optional.

### Pattern C: ForgeOps-managed telemetry collection

In a later milestone, ForgeOps may help operators deploy and configure OpenTelemetry Collectors or `vmagent`.

Configuration changes must require explicit authorization, validation, and audit logging.

### Pattern D: Mixed infrastructure

Different environments may use different telemetry sources.

For example:

* Kubernetes production clusters use OpenTelemetry Collectors.
* Bare-metal servers use existing Prometheus exporters.
* Docker Swarm uses host and container exporters.
* Application traces are stored in an existing trace backend.
* Logs remain in Loki.

The platform must support this without requiring one uniform collection strategy.

---

# 9. OpenTelemetry Integration

## 9.1 Functional requirements

ForgeOps must provide an OpenTelemetry integration module.

The integration must support:

* Existing Collector endpoints.
* OTLP-compatible transport.
* Collector health checks where an appropriate endpoint is available.
* Resource attribute mapping.
* Metric export or ingestion where configured.
* Trace correlation where a compatible tracing backend is available.
* Log correlation where a compatible log pipeline is available.
* TLS configuration.
* Authentication through supported mechanisms.
* Query and export timeouts.
* Telemetry pipeline error reporting.

Support for each signal must be explicit. Configuring an OTLP endpoint does not automatically mean that metrics, logs, and traces are all available.

## 9.2 Collector deployment patterns

ForgeOps must accommodate three Collector deployment models.

### Host-based Collector

A Collector can run as a systemd service on RHEL-compatible hosts.

Suitable for:

* Bare-metal servers.
* Standalone Docker hosts.
* Swarm nodes.
* Environments where Kubernetes is unavailable.

### Kubernetes DaemonSet

A Collector can run on each node to collect node-local telemetry and other approved sources.

The Collector's permissions must be restricted to the data it needs.

### Kubernetes Deployment

A centralized Collector gateway can receive OTLP data, apply processing, and export telemetry.

The gateway may be used alongside a DaemonSet, but both must not be deployed automatically without a reason.

## 9.3 Existing Collector mode

The initial implementation must support an existing Collector without requiring ForgeOps to manage its lifecycle.

Configuration should distinguish:

* Collector endpoint.
* Transport protocol.
* TLS settings.
* Authentication reference.
* Supported signal types.
* Health-check endpoint, if available.
* Timeout settings.

ForgeOps must not assume that every Collector endpoint exposes an administrative API.

## 9.4 Resource identity and attributes

ForgeOps should establish a stable mapping between discovered resources and telemetry resource attributes.

Relevant attributes may include:

* `service.name`
* `service.namespace`
* `service.instance.id`
* `deployment.environment.name`
* `host.name`
* `host.id`
* `k8s.cluster.name`
* `k8s.namespace.name`
* `k8s.node.name`
* `k8s.pod.name`
* `container.id`

Attribute availability depends on the instrumentation and Collector configuration.

ForgeOps must not assume that every telemetry source supplies every attribute.

When identity is ambiguous, the correlation engine must preserve uncertainty rather than merging unrelated resources.

## 9.5 Security

The OpenTelemetry integration must support:

* TLS and certificate validation.
* Authentication where supported by the endpoint.
* Secret references instead of plaintext credentials.
* Explicitly configured trusted certificate authorities.
* Bounded export retries.
* Payload and request-size limits.
* Endpoint allowlisting where required by deployment policy.

TLS verification must not be disabled by default.

## 9.6 Initial implementation boundary

The first release should primarily integrate with existing Collectors and telemetry backends.

Collector installation, upgrades, pipeline changes, and remote configuration management should be introduced only after the underlying authorization and audit framework is established.

---

# 10. VictoriaMetrics Integration

## 10.1 Objectives

VictoriaMetrics will be a first-class metrics backend.

ForgeOps must support querying existing VictoriaMetrics deployments and must not require VictoriaMetrics to be installed locally.

The preferred self-hosted deployment may use VictoriaMetrics single-node for simpler environments.

VictoriaMetrics cluster should be supported when deployment scale or availability requirements justify its additional operational complexity.

## 10.2 Supported integration modes

### Mode A: Query an existing installation

ForgeOps retrieves historical and current metrics using the supported VictoriaMetrics query APIs.

This is the initial priority.

### Mode B: OpenTelemetry metrics ingestion

An OpenTelemetry Collector exports metrics to a configured VictoriaMetrics ingestion endpoint.

For a single-node deployment, the typical OTLP/HTTP metrics endpoint is:

`/opentelemetry/v1/metrics`

The complete endpoint must be configured for the actual deployment.

For a cluster deployment, the endpoint and tenant configuration must match the deployed VictoriaMetrics architecture.

### Mode C: `vmagent`

Where appropriate, `vmagent` may scrape Prometheus-compatible targets and forward metrics to VictoriaMetrics.

ForgeOps may integrate with an existing `vmagent` deployment or support controlled deployment in a later release.

ForgeOps must not automatically create a second scraping pipeline if the required targets are already being scraped.

## 10.3 Query capabilities

The provider must implement:

* Instant metric queries.
* Range queries.
* Configurable query timeouts.
* Query result-size limits.
* Health checks.
* Backend version or capability discovery where available.
* Error normalization.
* Query metadata and execution timing.

The query layer should use Prometheus-compatible query semantics where supported.

ForgeOps must not assume every PromQL function, extension, or backend-specific feature behaves identically across all compatible systems.

## 10.4 Single-node and cluster support

The configuration must distinguish:

* Single-node VictoriaMetrics.
* Clustered VictoriaMetrics.
* Optional tenant identifiers.
* Query endpoint.
* Ingestion endpoint, if required.
* Authentication and TLS.
* Query timeout and result limits.

Cluster deployments commonly separate query, ingestion, and storage responsibilities. ForgeOps must use the appropriate endpoints rather than assume a single URL serves every purpose.

## 10.5 Metric cardinality and query protection

ForgeOps must avoid expensive or unbounded queries.

Requirements:

* Enforce query timeouts.
* Limit returned series and data points.
* Restrict default investigation time windows.
* Apply query concurrency limits.
* Avoid uncontrolled label expansion.
* Cache safe repeated queries where appropriate.
* Log query execution metadata without logging credentials.
* Expose partial results and query failures distinctly.

The system must not silently treat a failed or truncated query as a complete result.

## 10.6 Retention awareness

The metrics provider should expose available backend capabilities and, where reliably discoverable, the effective retention window.

Investigations must account for missing historical data.

If an incident occurred outside the available retention period, ForgeOps should report that evidence is unavailable rather than conclude that no anomaly occurred.

---

# 11. Prometheus Integration

Prometheus remains a supported metrics provider.

The initial adapter should support:

* Prometheus-compatible instant queries.
* Range queries.
* Health checks.
* Query timeouts.
* Error normalization.
* Basic capability detection.

ForgeOps may optionally integrate with Prometheus discovery and scrape configuration workflows in a later milestone.

Prometheus and VictoriaMetrics must implement the same internal metrics-provider contract where their capabilities overlap.

The investigation engine must not depend on either backend's specific client library or configuration format.

---

# 12. Logs and Traces

## 12.1 Logs

The initial log provider should support Loki.

Required capabilities:

* Query logs by time range.
* Filter by labels.
* Retrieve relevant entries around an incident.
* Apply query and response limits.
* Correlate log entries with hosts, services, containers and Kubernetes workloads where possible.
* Preserve query metadata and timestamps.
* Distinguish missing data from query errors.

OpenTelemetry log support should be implemented through the telemetry integration layer when a compatible ingestion or storage path is configured.

ForgeOps must not require Loki when another supported log backend is available.

## 12.2 Traces

Tracing support must be backend-neutral.

The initial architecture should define a trace-provider interface even if the first release does not implement a concrete tracing backend.

A later provider may integrate with an existing compatible trace store.

Required capabilities include:

* Lookup by trace ID.
* Query traces within a time range, where supported.
* Retrieve spans and relevant attributes.
* Identify service dependencies where the evidence supports them.
* Correlate spans with infrastructure resources.
* Preserve sampling and completeness limitations.

ForgeOps must not claim a complete request path when trace sampling or missing instrumentation makes the evidence incomplete.

---

# 13. Telemetry Interfaces in Go

## 13.1 Metrics provider

The metrics-provider interface should resemble the following conceptual contract:

```go
type MetricsProvider interface {
    QueryInstant(
        ctx context.Context,
        query string,
        at time.Time,
    ) (MetricResult, error)

    QueryRange(
        ctx context.Context,
        query string,
        start time.Time,
        end time.Time,
        step time.Duration,
    ) (MetricSeries, error)

    HealthCheck(ctx context.Context) error
}
```

This is a design sketch. The final implementation should define typed results, query limits, structured errors, and capability metadata.

## 13.2 Log provider

```go
type LogsProvider interface {
    QueryLogs(
        ctx context.Context,
        query LogQuery,
    ) (LogResult, error)

    HealthCheck(ctx context.Context) error
}
```

## 13.3 Trace provider

```go
type TracesProvider interface {
    GetTrace(
        ctx context.Context,
        traceID string,
    ) (TraceResult, error)

    HealthCheck(ctx context.Context) error
}
```

A production implementation may need additional search methods, pagination, capability discovery, and backend-specific query constraints.

## 13.4 OpenTelemetry integration

The OpenTelemetry module should provide separate responsibilities for:

* OTLP configuration.
* Export or ingestion.
* Resource attribute mapping.
* Collector health checks.
* Export failure reporting.
* Correlation metadata.

It must not implement metrics storage itself.

## 13.5 Provider registry

ForgeOps should resolve configured providers through a registry.

Example configuration:

```yaml
telemetry:
  metrics:
    provider: victoriametrics

  logs:
    provider: loki

  traces:
    provider: none
```

The registry must validate configuration at startup and report unsupported provider combinations clearly.

---

# 14. PostgreSQL Data Model

The initial database schema should contain the following entities.

## 14.1 Environments

Stores environment definitions.

Fields:

* `id`
* `name`
* `description`
* `environment_type`
* `configuration`
* `created_at`
* `updated_at`

Examples of environment types include development, staging, production, datacentre, and hybrid.

## 14.2 Resources

Stores normalized infrastructure resources.

Fields:

* `id`
* `environment_id`
* `kind`
* `provider`
* `external_id`
* `name`
* `namespace`
* `labels`
* `attributes`
* `status`
* `source`
* `first_seen_at`
* `last_seen_at`
* `stale_since`
* `created_at`
* `updated_at`

Use indexes for:

* Environment.
* Resource kind.
* Provider.
* External identity.
* Name and namespace where appropriate.
* Last-seen timestamps.

The schema must define a safe identity strategy that prevents collisions between different providers or environments.

## 14.3 Relationships

Fields:

* `id`
* `environment_id`
* `source_resource_id`
* `target_resource_id`
* `relationship_type`
* `confidence`
* `evidence_source`
* `observed_at`
* `attributes`

Relationship uniqueness must be defined according to the relationship type and identity of the evidence.

## 14.4 Discovery runs

Fields:

* `id`
* `environment_id`
* `adapter_type`
* `started_at`
* `completed_at`
* `status`
* `resources_discovered`
* `relationships_discovered`
* `errors`
* `metadata`

## 14.5 Telemetry integrations

Fields:

* `id`
* `environment_id`
* `provider_type`
* `integration_name`
* `endpoint_reference`
* `configuration`
* `enabled`
* `last_health_check_at`
* `health_status`
* `created_at`
* `updated_at`

Secrets must not be stored directly in ordinary configuration JSON.

Use a secret reference or a protected secret store.

## 14.6 Incidents

Fields:

* `id`
* `environment_id`
* `title`
* `description`
* `severity`
* `status`
* `detected_at`
* `resolved_at`
* `source`
* `resource_references`
* `attributes`

## 14.7 Investigations

Fields:

* `id`
* `incident_id`
* `status`
* `started_at`
* `completed_at`
* `hypotheses`
* `evidence_references`
* `summary`
* `confidence`
* `model_metadata`

AI outputs must be distinguishable from deterministic findings.

## 14.8 Jobs and audit events

Background jobs must record their status, execution attempts, timestamps, and errors.

Audit events must record:

* Actor.
* Action.
* Target resource.
* Timestamp.
* Outcome.
* Authorization context.
* Relevant request or correlation identifier.

Audit records must not contain passwords, tokens, or secret values.

## 14.9 Telemetry storage boundary

PostgreSQL should store telemetry configuration, query metadata, references, and investigation findings.

It should not become the primary store for high-volume time-series metrics, raw logs, or distributed traces.

---

# 15. Configuration

## 15.1 Configuration principles

Configuration must be:

* Validated at startup.
* Versioned where appropriate.
* Documented.
* Separated from secrets.
* Explicit about disabled integrations.
* Suitable for native Linux deployment.
* Suitable for containerized deployment.

A configuration change must not silently enable privileged collection or infrastructure modification.

## 15.2 Example configuration

The following is an illustrative configuration contract, not a claim that all fields are implemented.

```yaml
server:
  listen_address: "127.0.0.1:8080"

  tls:
    enabled: false

database:
  dsn_secret_ref: "forgeops/postgres-dsn"
  max_open_connections: 20
  max_idle_connections: 5

discovery:
  enabled: true
  interval: 5m

  host:
    enabled: true

  kubernetes:
    enabled: true
    read_only: true

  docker_swarm:
    enabled: true
    read_only: true

telemetry:
  enabled: true

  resource_identity:
    environment: production
    cluster_detection: true
    host_detection: true

  opentelemetry:
    enabled: true

    mode: existing_collector

    protocol: http
    endpoint: "https://otel-gateway.example.internal"

    tls:
      enabled: true
      ca_file: "/etc/forgeops/pki/ca.pem"

    auth:
      secret_ref: "forgeops/otel-auth"

  metrics:
    provider: victoriametrics

    victoriametrics:
      mode: single
      query_endpoint: "https://vm.example.internal"

      query_timeout: 15s
      max_range: 24h
      max_series: 1000

      auth:
        secret_ref: "forgeops/victoriametrics-auth"

    fallback_provider: prometheus

  logs:
    provider: loki
    endpoint: "https://loki.example.internal"

    auth:
      secret_ref: "forgeops/loki-auth"

  traces:
    provider: none

ai:
  enabled: false

  provider: local_openai_compatible

  endpoint: "http://127.0.0.1:9931"

  request_timeout: 120s

security:
  require_tls_for_remote_endpoints: true
  allow_insecure_tls: false

  remediation:
    enabled: false
    require_approval: true
```

Important implementation rules:

1. The application must reject unknown or invalid configuration where appropriate.
2. A disabled integration must not be contacted.
3. Fallback behavior must be explicit.
4. The configuration must distinguish query endpoints from ingestion endpoints.
5. Clustered VictoriaMetrics deployments must be supported by an appropriate configuration schema.
6. Secret references must be resolved through an approved secret mechanism.
7. The application must never log resolved secret values.
8. AI inference being disabled must not disable discovery or deterministic investigation.

---

# 16. REST API

The API must be versioned under `/api/v1`.

## 16.1 Environments

* `GET /api/v1/environments`
* `POST /api/v1/environments`
* `GET /api/v1/environments/{id}`
* `PATCH /api/v1/environments/{id}`
* `DELETE /api/v1/environments/{id}`

Destructive operations must have explicit authorization and audit logging.

## 16.2 Resources

* `GET /api/v1/resources`
* `GET /api/v1/resources/{id}`
* `GET /api/v1/resources/{id}/relationships`
* `GET /api/v1/resources/{id}/telemetry`

Resource queries must support pagination and bounded filtering.

## 16.3 Discovery

* `POST /api/v1/discovery/runs`
* `GET /api/v1/discovery/runs`
* `GET /api/v1/discovery/runs/{id}`
* `POST /api/v1/discovery/runs/{id}/cancel`

The cancel endpoint must return a clear status if the job cannot be safely interrupted.

## 16.4 Telemetry integrations

* `GET /api/v1/telemetry/integrations`
* `POST /api/v1/telemetry/integrations`
* `GET /api/v1/telemetry/integrations/{id}`
* `PATCH /api/v1/telemetry/integrations/{id}`
* `POST /api/v1/telemetry/integrations/{id}/health-check`

Integration modification requires authorization.

## 16.5 Telemetry queries

* `POST /api/v1/telemetry/metrics/query`
* `POST /api/v1/telemetry/metrics/query-range`
* `POST /api/v1/telemetry/logs/query`
* `GET /api/v1/telemetry/traces/{trace_id}`

All query endpoints must enforce:

* Authentication and authorization.
* Query timeouts.
* Time-range limits.
* Response-size limits.
* Provider-specific validation.
* Audit or operational logging appropriate to the request.

Raw backend query access must not provide a path around the platform's security controls.

## 16.6 Incidents and investigations

* `GET /api/v1/incidents`
* `GET /api/v1/incidents/{id}`
* `POST /api/v1/incidents`
* `POST /api/v1/incidents/{id}/investigations`
* `GET /api/v1/investigations/{id}`
* `GET /api/v1/investigations/{id}/evidence`

The API must distinguish a completed investigation from one that failed or returned incomplete evidence.

## 16.7 Remediation

* `GET /api/v1/remediations`
* `POST /api/v1/remediations/{id}/approve`
* `POST /api/v1/remediations/{id}/execute`

Remediation endpoints must remain disabled until the required authorization, approval, and audit controls are implemented.

---

# 17. Discovery and Telemetry Correlation

## 17.1 Objective

The principal differentiator of ForgeOps should be its ability to connect infrastructure state with operational telemetry.

The platform should answer questions such as:

* Which hosts run the affected workloads?
* Which Kubernetes nodes host the unhealthy pods?
* Which services share an affected node?
* Did CPU pressure increase before application latency?
* Are pod restarts correlated with node pressure?
* Are errors concentrated in one deployment or spread across a cluster?
* Are traces and logs available for the affected time window?
* Did a recent infrastructure or workload change coincide with the incident?

## 17.2 Correlation workflow

The investigation engine should follow this sequence:

1. Identify the affected service or resource.
2. Retrieve its topology relationships.
3. Determine relevant hosts, nodes, pods, containers, and dependencies.
4. Retrieve metrics for a bounded time window.
5. Retrieve relevant logs and events.
6. Retrieve traces where available.
7. Compare the incident window with a baseline.
8. Apply deterministic correlation rules.
9. Record evidence and data-quality limitations.
10. Generate hypotheses.
11. Optionally ask an AI model to interpret the evidence.
12. Return a structured investigation report.

## 17.3 Example investigation

**Scenario:** An application experiences increased request latency.

ForgeOps may discover:

* Request latency increased.
* Error rates increased.
* CPU saturation increased on a hosting node.
* Several affected pods were scheduled on that node.
* Container restarts occurred during the same period.
* Relevant logs show application errors.

The investigation should then distinguish the observations from the hypothesis.

**Observed evidence**

* Metrics show increased latency and CPU saturation.
* Kubernetes data identifies the affected pods and nodes.
* Logs show errors in the relevant time window.

**Hypothesis**

Node-level resource contention may be contributing to application latency.

**Further validation**

* Compare affected and unaffected nodes.
* Inspect CPU throttling and resource limits.
* Examine pod placement.
* Check recent deployments and configuration changes.

ForgeOps must not state that node contention is the confirmed root cause unless the evidence supports that conclusion.

## 17.4 Evidence model

Each investigation finding must reference its evidence.

An evidence record should include:

* Evidence identifier.
* Evidence type.
* Provider.
* Resource identifiers.
* Observation timestamp or time range.
* Query or retrieval metadata.
* Result summary.
* Completeness status.
* Confidence or uncertainty where appropriate.

The system should retain enough metadata to reproduce or inspect the analysis without duplicating large amounts of raw telemetry.

---

# 18. Incident Detection

Incident detection should initially be deterministic.

Supported inputs may include:

* Kubernetes events.
* Resource status transitions.
* Host health information.
* Metrics thresholds.
* Rate-of-change rules.
* Missing telemetry.
* Log patterns where configured.
* Existing alert notifications where supported.

The first implementation should prioritize useful correlation over sophisticated anomaly detection.

Later releases may introduce statistical baselines and anomaly detection.

## 18.1 Alert deduplication

The incident engine should group repeated observations that appear to describe the same underlying issue.

Deduplication must consider:

* Environment.
* Resource identity.
* Incident type.
* Time window.
* Existing incident status.

The system must preserve evidence from repeated observations rather than silently discard it.

## 18.2 Incident lifecycle

Initial statuses:

* `open`
* `investigating`
* `mitigated`
* `resolved`
* `closed`

Status transitions must be recorded and auditable.

---

# 19. AI-Assisted Investigation

## 19.1 Objectives

AI should help operators understand complex infrastructure problems, not replace deterministic discovery and monitoring.

Initial capabilities:

* Summarize incident evidence.
* Explain relevant metrics.
* Generate root-cause hypotheses.
* Recommend additional diagnostic queries.
* Explain Kubernetes and container failures.
* Correlate related observations.
* Generate incident reports.
* Recommend remediation steps.

## 19.2 Evidence-first architecture

The investigation engine must retrieve evidence before asking an AI model to interpret it.

The AI component should receive structured, bounded evidence instead of unrestricted access to all infrastructure and telemetry systems.

AI-generated statements must distinguish:

* Direct observations.
* Derived conclusions.
* Hypotheses.
* Missing information.
* Recommended actions.

## 19.3 Local inference

The design must support local OpenAI-compatible inference endpoints.

The initial integration should not require a remote AI API.

AI functionality should be optional and configurable.

A local model may be used for evidence summarization and hypothesis generation, while deterministic rules remain responsible for core detection and data retrieval.

## 19.4 AI safety

The AI model must not:

* Receive unrestricted credentials.
* Execute arbitrary shell commands by default.
* Modify infrastructure without authorization.
* Treat untrusted log content as trusted instructions.
* Invent metric values or resource relationships.
* Claim a confirmed root cause without supporting evidence.
* Bypass the remediation approval process.

Any tool invocation initiated by AI must be validated and authorized by the application.

---

# 20. Security Requirements

## 20.1 Authentication

The API must support secure authentication appropriate to the deployment.

Options may include:

* Local administrative authentication.
* OIDC integration.
* Service identities for agents.
* Mutual TLS for supported service-to-service connections.

The first implementation should avoid creating a complex identity subsystem if an established identity provider is available.

## 20.2 Authorization

Authorization must be enforced server-side.

Initial roles may include:

* Administrator.
* Operator.
* Investigator.
* Read-only viewer.

Permissions should distinguish:

* Viewing inventory.
* Running discovery.
* Querying telemetry.
* Managing integrations.
* Viewing investigation evidence.
* Approving remediation.
* Executing remediation.

## 20.3 Secrets

Credentials must be supplied through secret references or an approved secret store.

ForgeOps should be compatible with a self-hosted secret-management system.

The implementation must not:

* Commit credentials to source control.
* Store plaintext tokens in ordinary configuration files.
* Include secrets in logs.
* Expose secret values through API responses.
* Reuse a privileged credential across unrelated integrations unnecessarily.

## 20.4 Network security

The platform must support:

* TLS for remote APIs.
* Certificate verification.
* Configurable trusted certificate authorities.
* Endpoint allowlisting where required.
* Request and response limits.
* Connection timeouts.
* Rate limiting.
* Protection against server-side request forgery.

Integration endpoints must be validated to prevent an untrusted configuration from turning ForgeOps into a network access proxy.

## 20.5 Auditability

Security-sensitive operations must be auditable.

Examples:

* Integration creation and modification.
* Credential-reference changes.
* Discovery execution.
* Telemetry query access where policy requires it.
* Permission changes.
* Remediation approval.
* Remediation execution.

Audit data must be protected against unauthorized modification.

---

# 21. Observability of ForgeOps Itself

ForgeOps must be observable without depending exclusively on the observability infrastructure it manages.

## 21.1 Application logs

Use structured logs with fields such as:

* Timestamp.
* Severity.
* Component.
* Request ID.
* Job ID.
* Environment ID.
* Integration ID.
* Error classification.

Do not log credentials or unbounded raw telemetry payloads.

## 21.2 Application metrics

Expose metrics covering:

* API request count and latency.
* API errors.
* Discovery runs and failures.
* Resources discovered.
* Job queue depth.
* Job duration.
* Telemetry query count and latency.
* Telemetry query errors.
* Provider health.
* Collector export failures where available.
* Investigation duration.
* AI request latency and errors.
* Remediation outcomes.

Metrics must use bounded labels to avoid excessive cardinality.

Avoid labels containing arbitrary resource names, raw query text, user identifiers, or unbounded error strings.

## 21.3 OpenTelemetry instrumentation

The application should use OpenTelemetry-compatible instrumentation for its own metrics and traces.

Where enabled, ForgeOps may export its own telemetry to an existing Collector.

The control plane must continue operating if telemetry export fails.

Self-observability must not create a circular operational dependency in which ForgeOps cannot start or investigate incidents because its own metrics backend is unavailable.

## 21.4 Health endpoints

Provide:

* `/health/live`
* `/health/ready`
* `/health/integrations`

The endpoints must have distinct semantics.

Liveness must not fail merely because VictoriaMetrics, Loki, or an optional AI provider is temporarily unavailable.

Readiness may report required dependency failures according to the deployment configuration.

Integration health must expose per-provider status without disclosing credentials.

---

# 22. Deployment and Packaging

## 22.1 Primary deployment target

The primary production deployment target is a RHEL-compatible Linux system.

The project should support compatible distributions such as RHEL, Rocky Linux, and AlmaLinux, subject to testing against the specific versions declared in the release matrix.

## 22.2 Native deployment

Provide:

* RPM packaging.
* systemd service units.
* A dedicated non-root service account.
* Protected configuration directories.
* Protected data and log directories.
* Configurable TLS certificate locations.
* Upgrade and rollback documentation.

Suggested filesystem layout:

```text
/usr/bin/forgeops-server
/usr/bin/forgeops-agent
/usr/bin/forgeopsctl

/etc/forgeops/
/etc/forgeops/pki/

/var/lib/forgeops/
/var/log/forgeops/

/usr/lib/systemd/system/forgeops-server.service
/usr/lib/systemd/system/forgeops-agent.service
```

The exact installation layout should follow the packaging conventions of the target distributions.

## 22.3 Containerized deployment

Container deployment should also be supported.

Provide:

* A minimal application image.
* A non-root runtime.
* Read-only root filesystem where practical.
* Explicit persistent storage requirements.
* Health checks.
* Resource limits.
* Secure secret injection.
* A Compose-based development environment.

The containerized control plane must not require privileged mode by default.

## 22.4 Kubernetes deployment

A Kubernetes deployment package may be provided for organizations that want to run ForgeOps inside Kubernetes.

However, Kubernetes must not be required to install or operate ForgeOps.

---

# 23. Performance and Reliability

## 23.1 General requirements

The platform must support:

* Concurrent discovery jobs.
* Bounded telemetry query concurrency.
* Request timeouts.
* Job retries.
* Graceful shutdown.
* Database connection pooling.
* Pagination.
* Partial failure handling.
* Cancellation where safe.
* Recovery after process restarts.

## 23.2 Initial sizing

The initial deployment should target small-to-medium environments with hundreds of hosts and Kubernetes nodes, subject to workload and query complexity.

The implementation must not claim a fixed maximum scale until benchmark results are available.

Performance tests should establish:

* Discovery throughput.
* Database growth.
* Query concurrency.
* API latency.
* Investigation duration.
* Memory consumption.
* CPU consumption.

## 23.3 Failure isolation

Failure of an optional provider must not disable unrelated capabilities.

Examples:

* VictoriaMetrics unavailable: inventory and discovery continue.
* Loki unavailable: metrics-based investigation continues.
* AI provider unavailable: deterministic analysis continues.
* One Kubernetes cluster unreachable: other environments remain accessible.
* OpenTelemetry export fails: the application reports the failure and continues operating.

---

# 24. Testing Strategy

## 24.1 Unit tests

Test:

* Resource normalization.
* Identity generation.
* Relationship generation.
* Configuration validation.
* Query validation.
* Incident deduplication.
* Evidence aggregation.
* Authorization.
* Job retries.
* Error normalization.

## 24.2 Adapter tests

Each adapter must have fixtures representing:

* Normal responses.
* Empty responses.
* Partial results.
* Authentication failures.
* Timeouts.
* Malformed responses.
* Unsupported capabilities.

## 24.3 Telemetry integration tests

### VictoriaMetrics

Test:

* Successful health checks.
* Instant queries.
* Range queries.
* Authentication failures.
* Query timeouts.
* Missing metrics.
* Partial results.
* Single-node configuration.
* Cluster configuration where supported.

### OpenTelemetry

Test:

* Existing Collector configuration.
* OTLP endpoint connectivity.
* TLS validation.
* Authentication failure.
* Export failure reporting.
* Resource attribute mapping.
* Collector unavailability.
* Signal-specific configuration.

### Prometheus

Test:

* Instant and range queries.
* Backend errors.
* Query limits.
* Timeout handling.

### Loki

Test:

* Time-bounded log queries.
* Label filters.
* Query errors.
* Empty results.
* Response-size limits.

## 24.4 End-to-end tests

The end-to-end test environment should include:

* A PostgreSQL instance.
* A test Linux host or fixture.
* A test Kubernetes cluster.
* A Docker Swarm test environment where feasible.
* VictoriaMetrics.
* An OpenTelemetry Collector.
* Loki or a suitable test log backend.
* A deterministic incident scenario.

Tests must verify that ForgeOps can discover resources, retrieve telemetry, correlate evidence, and generate a structured investigation.

## 24.5 Security tests

Test:

* Unauthorized API requests.
* Privilege boundaries.
* Secret leakage.
* Malicious endpoint configurations.
* Request-size limits.
* Query abuse.
* Untrusted log content.
* Remediation authorization.
* Audit integrity.

---

# 25. Implementation Roadmap

Implementation should proceed through small, testable releases.

## M0 — Requirements and architecture

Deliverables:

* Confirm deployment targets.
* Confirm initial security model.
* Define resource identities.
* Define provider interfaces.
* Define API conventions.
* Define the configuration schema.
* Establish the release and test matrix.

**Exit criteria:** Architecture decisions and initial API contracts are documented.

## M1 — Go foundation

Deliverables:

* Go module and repository structure.
* Configuration validation.
* Structured logging.
* PostgreSQL migrations.
* API framework.
* Health endpoints.
* Basic authentication.
* systemd development deployment.

**Exit criteria:** ForgeOps starts reliably and passes initial unit and integration tests.

## M2 — Resource inventory and host discovery

Deliverables:

* Resource model.
* Relationship model.
* Discovery-run lifecycle.
* Linux host discovery.
* PostgreSQL persistence.
* CLI commands for inventory and discovery.

**Exit criteria:** The platform can discover and persist host resources without modifying the target.

## M3 — Kubernetes and Docker discovery

Deliverables:

* Kubernetes API adapter.
* Cluster and workload inventory.
* Node and pod relationships.
* Docker Swarm adapter.
* Standalone Docker support.
* Partial-failure handling.

**Exit criteria:** The platform can discover the initial supported infrastructure types and represent their relationships.

## M4 — Telemetry foundation

Deliverables:

* Common metrics, logs, and traces models.
* Provider interfaces.
* Integration configuration model.
* Secret references.
* Query timeout and response limits.
* Telemetry health checks.
* Resource identity mapping.

**Exit criteria:** The investigation engine can request telemetry without depending on a specific backend.

## M5 — VictoriaMetrics and Prometheus

Deliverables:

* VictoriaMetrics provider.
* Single-node integration.
* Cluster integration support.
* Instant and range queries.
* Query safety controls.
* Prometheus provider.
* Provider health reporting.

**Exit criteria:** ForgeOps can retrieve bounded historical metrics from supported metrics backends.

## M6 — OpenTelemetry integration

Deliverables:

* Existing Collector integration.
* OTLP endpoint configuration.
* TLS and authentication support.
* Resource attribute mapping.
* Signal-specific configuration.
* Export and connectivity failure reporting.
* Deployment examples.

**Exit criteria:** ForgeOps can integrate with an existing Collector and use the configured telemetry path without requiring it to manage Collector lifecycle.

## M7 — Logs and traces

Deliverables:

* Loki provider.
* Trace-provider contract.
* Initial compatible tracing backend integration, if selected.
* Log and trace correlation.
* Missing-data handling.

**Exit criteria:** The investigation engine can collect relevant evidence from multiple telemetry sources.

## M8 — Incident detection and correlation

Deliverables:

* Deterministic incident rules.
* Resource topology correlation.
* Metric baselines and comparisons.
* Event correlation.
* Evidence records.
* Incident lifecycle.
* Deduplication.

**Exit criteria:** A test incident produces a reproducible investigation record with linked evidence.

## M9 — AI-assisted investigation

Deliverables:

* Provider-neutral AI interface.
* Local OpenAI-compatible inference support.
* Evidence summarization.
* Root-cause hypotheses.
* Suggested diagnostic queries.
* Structured investigation reports.
* AI safety controls.

**Exit criteria:** AI-assisted reports distinguish facts from hypotheses and reference supporting evidence.

## M10 — Interfaces and operational experience

Deliverables:

* Expanded CLI.
* API documentation.
* Operational status views.
* Investigation report presentation.
* Integration configuration workflows.

A web UI may be implemented in this milestone if it is included in the product requirements.

**Exit criteria:** Operators can manage environments, inspect resources, and review investigations through the supported interfaces.

## M11 — Controlled remediation

Deliverables:

* Remediation proposal model.
* Approval workflow.
* Authorization checks.
* Dry-run support where applicable.
* Execution timeouts.
* Audit trail.
* Rollback or recovery guidance where possible.

**Exit criteria:** Only explicitly authorized and approved actions can modify infrastructure.

## M12 — Production hardening

Deliverables:

* RPM packaging.
* systemd service hardening.
* Container image hardening.
* Upgrade documentation.
* Backup and restore procedures.
* Security testing.
* Performance testing.
* Operational documentation.

**Exit criteria:** A documented release candidate passes the required functional, security, and reliability tests.

---

# 26. Initial Release Acceptance Criteria

The first release should be considered complete only when it can:

1. Run on a supported RHEL-compatible Linux distribution.
2. Start as a non-root systemd service.
3. Store metadata in PostgreSQL.
4. Discover Linux host resources.
5. Discover Kubernetes resources using restricted credentials.
6. Discover Docker Swarm resources within the supported test matrix.
7. Persist resource relationships.
8. Integrate with an existing VictoriaMetrics instance.
9. Query current and historical metrics within configured limits.
10. Integrate with an existing OpenTelemetry Collector.
11. Integrate with Prometheus-compatible metrics backends.
12. Integrate with Loki for logs.
13. Correlate telemetry with discovered infrastructure where identity is available.
14. Record investigation evidence.
15. Continue core operation when an optional telemetry or AI provider is unavailable.
16. Enforce authentication and authorization.
17. Avoid logging credentials.
18. Produce auditable discovery and investigation results.
19. Pass unit, integration, and security tests.
20. Operate without a mandatory external AI API.

AI-assisted investigation and controlled remediation may be delivered in later releases.

---

# 27. Key Risks and Mitigations

| Risk                           | Mitigation                                                            |
| ------------------------------ | --------------------------------------------------------------------- |
| Excessive implementation scope | Deliver discovery, telemetry and investigation in separate milestones |
| High-cardinality metrics       | Enforce query limits and bounded application labels                   |
| Incorrect resource correlation | Use stable identities and preserve uncertainty                        |
| AI-generated false conclusions | Require linked evidence and separate observations from hypotheses     |
| Privileged Docker access       | Minimize socket access and isolate the adapter                        |
| Kubernetes overprivilege       | Use restricted service accounts and capability-specific permissions   |
| Duplicate telemetry collection | Prefer existing monitoring infrastructure                             |
| Observability backend outage   | Isolate providers and preserve discovery functionality                |
| Missing historical data        | Report retention limitations and incomplete evidence                  |
| Credential leakage             | Use secret references and redact sensitive values                     |
| Excessive telemetry queries    | Apply concurrency, timeout, range and response-size limits            |
| Vendor lock-in                 | Keep backend-specific code behind provider interfaces                 |
| Unsafe remediation             | Require explicit authorization and auditable approval                 |

---

# 28. Final Architectural Decisions

The following decisions define the initial ForgeOps implementation.

1. **Language:** Go.
2. **Architecture:** Modular-monolith control plane with optional host agent.
3. **Metadata:** PostgreSQL.
4. **Deployment:** Native RHEL-compatible Linux deployment as the primary target; container deployment supported.
5. **Discovery:** Read-only by default.
6. **Infrastructure:** Linux, Kubernetes, Docker Swarm, and standalone Docker.
7. **Telemetry standard:** OpenTelemetry.
8. **Preferred metrics backend:** VictoriaMetrics.
9. **Alternative metrics backend:** Prometheus-compatible systems.
10. **Initial log backend:** Loki.
11. **Tracing:** Provider-neutral interface, with concrete backend support selected for a subsequent milestone.
12. **AI:** Optional, provider-neutral, with local inference supported.
13. **Incident analysis:** Deterministic evidence collection before AI interpretation.
14. **Remediation:** Disabled by default and implemented separately from investigation.
15. **Security:** Least privilege, protected secrets, TLS, authorization, and auditability.
16. **Extensibility:** Provider interfaces for infrastructure, metrics, logs, traces, and AI.
17. **Operational independence:** Core discovery must remain functional without VictoriaMetrics, Loki, OpenTelemetry export, or AI.
18. **Implementation order:** Foundation → discovery → telemetry → correlation → AI → controlled remediation → production hardening.

---

# 29. Recommended Next Step

The next step is to establish the repository and implement **M0–M2** before introducing AI or automated remediation.

The first working vertical slice should:

1. Start the Go control plane.
2. Connect to PostgreSQL.
3. Discover a Linux host.
4. Persist resources and relationships.
5. Expose inventory through the REST API.
6. Report discovery status and errors.
7. Run as a non-root Linux service.

Once this is reliable, add VictoriaMetrics querying and OpenTelemetry integration against test instances.

This order establishes the resource identity and data model that telemetry correlation will depend on later. It also avoids prematurely building AI workflows on top of an unproven discovery layer.

**The architectural objective is to make ForgeOps an evidence-driven operations platform, not simply an AI wrapper around infrastructure APIs or a replacement for existing monitoring software.**
