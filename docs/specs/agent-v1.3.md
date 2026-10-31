<!-- SPDX-License-Identifier: MIT -->
# Status: Current agent design specification

This is the current design specification for the agent. It does not imply that every specified component is implemented or released.

# UptimeFoundry Agent — Technical Specification v1.3

Oct 24, 2026 · @Vitalii

## 1. Purpose, scope, and changes from v1.2

`uptimefoundry-agent` is a host-resident component that gathers discovery data, network-flow topology and optional zero-code telemetry from Linux hosts and reports it to the UptimeFoundry control plane. It is the highest-privilege and most widely deployed part of the platform, so v1.3 treats it as a security boundary first and a feature set second.

### 1.1 Goals

1. Collect host inventory, workload identity and flow topology with no inbound listeners on the host.
2. Run with the minimum Linux capabilities per function, split across separate processes.
3. Never hold credentials for PostgreSQL, VictoriaMetrics, Loki or any other backend.
4. Degrade predictably across kernels, cgroup versions and policy restrictions instead of failing to start.
5. Stay inside a fixed, measured CPU, memory and disk budget, and shed work before breaching it.
6. Keep working through control-plane outages, with bounded local buffering and explicit loss accounting.
7. Remain read-only. Anything that changes a host is a separate, optional binary (section 3.4).

### 1.2 Non-goals

- Replacing node-level collectors such as an OpenTelemetry Collector for general log and metric pipelines. The agent may embed collector components but does not compete with a full pipeline.
- Reimplementing eBPF protocol instrumentation that the upstream OpenTelemetry eBPF Instrumentation (OBI) project already provides.
- Remote shell, arbitrary command execution or file retrieval.
- Self-updating binaries.
- Windows, macOS or non-Linux hosts.

### 1.3 Changes from v1.2

| Area | v1.2 | v1.3 |
| --- | --- | --- |
| Process model | One agent process hosting eBPF and discovery | Supervisor plus separate discovery, ebpf and (later) actuator processes |
| Backend access | Agent writes to PostgreSQL and streams OTLP to VictoriaMetrics | Agent talks only to the control plane over mTLS gRPC; metrics go through a relay |
| Capabilities | `CAP_BPF`, `CAP_NET_ADMIN` | Per-process sets including `CAP_PERFMON`, `CAP_SYS_PTRACE` or `CAP_DAC_READ_SEARCH` as needed, plus a shipped SELinux policy |
| eBPF code | Custom HTTP, gRPC and SQL parsers and TLS uprobes in `internal/ebpf` | Wrap and supervise upstream OBI; custom eBPF limited to flow tracking |
| TLS inspection | On for all processes when `monitor_tls: true` | Off by default; per-process opt-in with on-host redaction |
| `traceparent` injection | Part of the OBI feature set | Removed from the baseline; experimental and separately authorized only |
| Flow edges | Per-event upserts | Host-side flow table with 10 to 30 second deltas and edge TTLs |
| Enrollment | Not specified | Bootstrap token, short-lived mTLS certificate, rotation and revocation |
| Failure handling | Fallback only for kernels older than 4.18 | Four-tier degradation ladder driven by a capability report |
| Local control | Not specified | Root-owned policy ceiling that remote configuration can narrow but not widen |

Safeguards that v1.2 dropped from v1.1 and that apply to the agent again: TLS is mandatory for every remote endpoint, insecure TLS is off by default, and the failure of any optional provider must not stop discovery.

## 2. Threat model and trust boundaries

The agent's design assumption is that any single component can be compromised, and that the blast radius must stay at one host and one function. Observed hosts are also untrusted: everything the agent parses came from a process or network peer it does not control.

### 2.1 Assets

| Asset | Why it matters |
| --- | --- |
| Host root and kernel | The agent holds BPF and ptrace-class capabilities |
| Plaintext application traffic | TLS uprobes can see tokens, cookies and SQL literals |
| Agent identity key | Lets an attacker impersonate a host to the control plane |
| Control-plane integrity | A hostile control plane could direct every agent |
| Backend credentials | PostgreSQL, VictoriaMetrics and Loki must never reach a host |
| Topology data | Reveals internal architecture and attack paths |

### 2.2 Adversaries

1. **Hostile workload on an observed host.** Sends crafted HTTP, gRPC, SQL, DNS or log content to exploit parsers or poison topology and metrics.
2. **Local unprivileged user.** Tries to read agent buffers, sockets or keys, or to trigger actions through local IPC.
3. **Compromised agent process.** Attempts to reach other host processes, other agents or backends.
4. **Network attacker.** Intercepts or replays agent traffic, or impersonates the control plane.
5. **Compromised control plane.** Pushes configuration or tasks to widen collection or run actions on hosts.
6. **Malicious content aimed at AI analysis.** Log lines or headers written to manipulate downstream LLM reasoning.

### 2.3 Trust zones

- **Zone 0, kernel and host root.** Trusted. Only the supervisor and the local policy file are authoritative here.
- **Zone 1, agent processes.** Partially trusted. Each process holds only its own capabilities and cannot call another's privileged functions.
- **Zone 2, observed workloads.** Untrusted input. All parsing happens in the ebpf process, which has no network egress and no write access outside its own state directory.
- **Zone 3, control plane.** Trusted for configuration within the local policy ceiling and not beyond it.
- **Zone 4, backends.** Never reachable from the agent.

### 2.4 Required properties

| Threat | Mitigation | Section |
| --- | --- | --- |
| Parser exploit from workload data | Separate process, seccomp, no egress, fuzzing | 3, 15 |
| Secrets captured by TLS uprobes | Per-process opt-in, on-host redaction | 8 |
| Stolen agent credentials | Short-lived certificates, keys in memory or TPM where available, revocation | 4 |
| Replay or impersonation | mTLS, sequence numbers, per-message authentication context | 4, 5 |
| Compromised control plane widens collection | Root-owned policy ceiling, closed set of signed task types | 9 |
| Agent compromised pivots to backends | No backend credentials on hosts | 5 |
| Resource exhaustion by hostile traffic | Probe budgets, cardinality caps, shedding | 11 |
| Prompt injection through captured content | Captured text is labelled untrusted data and never forwarded as instructions | 8 |

### 2.5 Explicit assumptions

- Root on a host defeats the agent's local protections. The design limits what root on one host yields elsewhere, not what it yields locally.
- The control-plane operator is trusted to configure fleet-wide intent, but cannot exceed what each host's policy file permits.

## 3. Process model and privilege separation

The agent is four cooperating processes, not one. The only process that opens network connections holds no BPF privileges, and the only process that holds BPF privileges has no network access.

&#91;embedded content: process model · 4 processes, one with network egress\]

Discovery and eBPF hand schema-checked records to the supervisor over local sockets, and the supervisor alone talks to the control plane. Backends are never reachable from the host.

### 3.1 Processes

| Process | Runs as | Responsibility | Network |
| --- | --- | --- | --- |
| `supervisor` | `uptimefoundry-agent` user, no capabilities | Enrollment, mTLS stream, policy intersection, WAL, watchdog, child lifecycle | Outbound to the control-plane endpoint only |
| `discovery` | `uptimefoundry-agent-disc` user, no capabilities | Host inventory from procfs, sysfs and D-Bus | None |
| `ebpf` | `uptimefoundry-agent-ebpf` user, narrow capability set | Flow tracking, supervised OBI child, parsers, redaction | None; `AF_UNIX` and `AF_NETLINK` only |
| `actuator` | Separate unit and package | Approved host actions (A5 and later) | Local only; receives instructions through the supervisor |

### 3.2 Capabilities per process

| Process | Capabilities | Why |
| --- | --- | --- |
| `supervisor` | None | Uses ordinary sockets and files under its own directory |
| `discovery` | None | Reads world-readable procfs and sysfs; D-Bus access is granted by policy |
| `ebpf` | `CAP_BPF`, `CAP_PERFMON` | Load programs; attach kprobes, tracepoints and uprobes (kernel 5.8 and later) |
| `ebpf` | `CAP_SYS_PTRACE` or `CAP_DAC_READ_SEARCH` | Read other processes' `/proc/<pid>/ns`, `exe` and `cgroup`; `CAP_DAC_READ_SEARCH` is preferred where it is enough |
| `ebpf` | `CAP_SYS_RESOURCE` | Raise the memlock limit on kernels before 5.11 |
| `ebpf` | `CAP_NET_ADMIN` | Only for features that need it, such as conntrack queries or traffic-control attach; off in the default profile |
| `actuator` | Per action, none by default | Defined by each approved action type |

On kernels where `CAP_BPF` does not exist (RHEL 8's 4.18 base), the `ebpf` process may need a broader privilege or the host runs at Tier 3. This is an open question in section 17. The set must be re-verified for each supported RHEL minor release, and any host where the full set is not granted reports the missing capabilities in its capability report.

### 3.3 Inter-process communication

- Each child connects to the supervisor over a Unix domain socket in a directory owned by the agent group. The supervisor checks peer credentials with `SO_PEERCRED` and refuses unexpected users.
- Messages are length-prefixed Protobuf with a maximum frame size and a schema per message class. The supervisor validates every record and drops any that fail validation, counting them.
- No shared memory and no file hand-off carries data between processes.
- Children are started by the supervisor with the capabilities and seccomp filter for their role, and with `no_new_privs` set. A crashed child is restarted with exponential backoff and its crash reason is included in the next heartbeat.
- A crash or compromise of the `ebpf` process cannot reach the identity key, which only the supervisor can read.

### 3.4 The actuator

Remediation is deliberately not part of this binary. The `actuator` is a separate package and systemd unit, disabled by default, with its own SELinux domain and capability set per action type. It accepts only signed, approved actions routed by the supervisor and listed in the local policy. It is specified with the approval and audit framework (platform M9) and is introduced in roadmap step A5; until then no host action code exists on a host running the agent.

## 4. Enrollment, identity and credentials

Every agent proves a stable host identity with a short-lived mTLS certificate that the control plane issues after a one-time enrollment. No long-lived secret is stored on the host after enrollment completes.

### 4.1 Enrollment flow

1. An administrator creates an **enrollment token** in the control plane, scoped to one environment, with a maximum use count, an expiry (default 24 hours) and optional labels to apply.
2. The token is delivered to the host through the package configuration, a Kubernetes Secret or a secret manager. It is read once and deleted from disk after use.
3. On first start the agent generates an asymmetric key pair locally (ECDSA P-256 or Ed25519). The private key never leaves the host.
4. The agent connects to the control plane over TLS, verifying the server against the configured CA bundle or a pinned certificate fingerprint, and sends a certificate signing request, the token and a signed capability report (section 6).
5. The control plane validates the token, records the host, issues a client certificate and returns it with the current agent policy.
6. The agent stores the key and certificate under `/var/lib/uptimefoundry-agent/identity/` with mode `0600`, owned by the agent user, and discards the token.

An enrollment token is never accepted over an unverified TLS connection, and the agent refuses to enroll if the server certificate cannot be validated.

### 4.2 Identity model

| Field | Source | Notes |
| --- | --- | --- |
| `agent_id` | Control plane, UUIDv7 | Stable across restarts and certificate renewals |
| `host_fingerprint` | Hash of `machine-id`, DMI product UUID when readable, and primary MAC set | Used to detect clones and hardware changes, not as the identity |
| `environment_id` | Enrollment token | Cannot be changed by the agent |
| Certificate subject | `agent_id` only | No hostnames in the subject, since they change |
| Certificate lifetime | Default 24 hours | Renewed at 50 percent of lifetime |

### 4.3 Rotation and revocation

- The agent renews its certificate over the existing authenticated stream, with a new key pair on every renewal when the host has a TPM or on a configurable schedule (default every 7 days) otherwise.
- Revocation is enforced by short lifetime plus an explicit deny list on the control plane. A revoked agent's next renewal is refused and its stream is closed.
- If the control plane is unreachable, the agent keeps using its certificate until expiry, then enters offline mode (section 10). For a renewal grace period after expiry (default 7 days) it may request a new certificate by proving possession of its existing private key over a server-authenticated TLS connection. After the grace period it needs re-enrollment with a new token. It never falls back to a weaker credential.
- An operator can force re-enrollment with a new token. The old `agent_id` is retired and its data retained according to the retention policy.

### 4.4 Cloned machines and identity changes

- Duplicate `machine-id` values, common after VM cloning, are detected by the control plane when two live streams claim the same `agent_id`, or when `host_fingerprint` changes sharply. The newer connection is refused and an alert is raised.
- The package post-install step warns when `/etc/machine-id` is empty, uninitialized or matches a known template value.
- Hostname changes update the resource record but never the identity.
- Where a TPM 2.0 is present, the private key may be sealed to it. This is optional and reported in the capability report.

### 4.5 Kubernetes enrollment

In a cluster the DaemonSet can use a projected service account token with a dedicated audience instead of a bootstrap token. The control plane validates it against the cluster's token review API, then issues the same short-lived certificate. The node name and pod identity are recorded as attributes, not as the identity.

## 5. Agent to control plane protocol

The agent always dials out. It opens one long-lived bidirectional gRPC stream to the control plane over mTLS and multiplexes every message type on it. The agent exposes no inbound network listener; local IPC uses Unix domain sockets with filesystem permissions and peer-credential checks.

### 5.1 Transport rules

- gRPC over HTTP/2 with TLS 1.3 only. Client certificate required. The server certificate is validated against a configured CA bundle, with optional fingerprint pinning.
- Egress goes through one configurable endpoint, with optional HTTP CONNECT proxy support and a secondary endpoint list for control-plane high availability.
- Keepalive pings every 30 seconds. Reconnect with exponential backoff and full jitter, from 1 second up to 5 minutes.
- Maximum message size 4 MiB, with larger payloads chunked. Stream-level compression is optional and off for already-compressed payloads.
- The agent never receives backend URLs or credentials. Metrics, logs and flow data go to the control plane (or its relay) and nowhere else.

### 5.2 Message types

| Direction | Message | Purpose | Delivery |
| --- | --- | --- | --- |
| Agent to server | `Register` | Capability report, version, host fingerprint, stream resume token | Once per connection |
| Server to agent | `Policy` | Effective configuration, already intersected with the local ceiling | On connect and on change |
| Agent to server | `Heartbeat` | Health, probe states, drop counters, buffer depth | Every 15 seconds |
| Agent to server | `InventoryDelta` | Host and workload resource changes | At-least-once, sequenced |
| Agent to server | `InventorySnapshot` | Full state for resync | On request and after long outages |
| Agent to server | `FlowDelta` | Aggregated edges with counters and RTT | At-least-once, sequenced |
| Agent to server | `MetricBatch` | OTLP-compatible metric data for relay | At-least-once, may be dropped under load |
| Agent to server | `EventBatch` | Selected log and journal events, systemd state changes | At-least-once, bounded |
| Server to agent | `Task` | One signed, typed task from the closed set in section 9.4 | Acknowledged |
| Agent to server | `TaskResult` | Outcome and bounded output | Acknowledged |
| Server to agent | `CertRenew` / `CertRenewResult` | Rotation over the live stream | Acknowledged |

### 5.3 Sequencing and resync

1. Every data message carries `agent_id`, a `stream_epoch` (incremented per connection), and a monotonic `sequence` number per message class.
2. The server acknowledges the highest contiguous sequence. The agent keeps unacknowledged messages in its write-ahead log (section 10) and resends them after a reconnect.
3. The server deduplicates by `(agent_id, class, sequence)`, so redelivery is safe.
4. If the server detects a gap it cannot recover (for example the agent dropped data under backpressure), it sends `ResyncRequest`. The agent replies with an `InventorySnapshot` or a full flow-table dump and the server replaces the previous state for that agent.
5. Each delta states the interval it covers, so the server can age out edges that stop appearing.

### 5.4 Versioning

- Messages are Protobuf. Fields are only added, never reused or renumbered.
- `Register` carries `protocol_version` and `agent_version`. The server supports the current and previous protocol version (N and N-1). Older agents receive a clear `UpgradeRequired` error and continue to send heartbeats if they can.
- Unknown fields are ignored. Unknown task types are refused, never guessed at.

### 5.5 Metrics relay

Metrics reach VictoriaMetrics through the control plane or a dedicated relay service that authenticates agents with the same mTLS identity and writes with backend credentials the agent never sees. Sites that prefer a local OpenTelemetry Collector can enable an optional local OTLP exporter in the policy, restricted to a loopback or Unix-socket endpoint; this is a deliberate exception and is recorded in the capability and policy reports.

## 6. Capability report and degradation ladder

At every start the agent probes what the host actually allows, reports it, and selects the highest working tier. The control plane uses the report to show operators what each host can and cannot do, and never assumes eBPF is available.

### 6.1 Capability report fields

| Group | Fields |
| --- | --- |
| Kernel | Version, architecture, distribution release, `/sys/kernel/btf/vmlinux` present, lockdown mode, `kernel.unprivileged_bpf_disabled`, `bpf_jit_enable`, available BPF program and map types, memlock limit |
| Security | SELinux mode and agent domain, seccomp support, Secure Boot state, granted capabilities per process, `no_new_privs` |
| Containers | cgroup version (v1, v2 or hybrid), container runtimes detected, Kubernetes or Swarm membership, `hostPID` and `hostNetwork` availability |
| Observability | Detected exporters, running OpenTelemetry Collector or vmagent, journald access, conntrack availability |
| Identity | `machine-id` health, DMI product UUID readable, TPM 2.0 present |
| Runtime | Agent and protocol versions, CPU count, memory, filesystem free space for the buffer path |
| Probe status | Per probe: `active`, `disabled_by_policy`, `unsupported`, `denied`, `degraded`, with a machine-readable reason |

The report is signed with the agent key and re-sent whenever any value changes, for example after a kernel upgrade or an SELinux mode change.

### 6.2 Degradation tiers

The agent starts at the highest tier its policy and host allow, and moves down automatically when a probe cannot attach, is denied, or is shed by the watchdog. Moving up requires a restart or an explicit policy change, to avoid flapping.

| Tier | Name | Requires | Provides | Loses |
| --- | --- | --- | --- | --- |
| 1 | Full eBPF | BTF, BPF-capable kernel, capabilities granted, SELinux domain allows | OBI RED metrics for opted-in processes, flow table with RTT and retransmits, DNS timing, host discovery | Nothing |
| 2 | Reduced eBPF | Tier 1 with some probes failing or shed | Flow table and socket-level counters, host discovery; no protocol parsing or TLS inspection | RED metrics, TLS, SQL visibility |
| 3 | Procfs sampling | Read access to `/proc/net`, `ss` or netlink, optional conntrack | Periodic connection sampling and approximate edges, host discovery | Accurate RTT, short-lived connections, retransmit detail |
| 4 | Discovery only | Read access to `/proc`, `/sys` and systemd over D-Bus | Inventory, systemd state, filesystem and interface data | All flow and traffic evidence |

Every tier change is reported in the next heartbeat with its reason. Evidence produced at a lower tier is labelled so investigations never present a Tier 3 sample as a complete record.

### 6.3 Platform matrix and verification points

| Platform | Kernel line | cgroup default | eBPF approach |
| --- | --- | --- | --- |
| RHEL 8 and compatible | 4.18 with vendor backports | v1 | Detect features at runtime rather than by version number; `CAP_BPF` is not available on a stock 4.18 base, so confirm which privilege model the target minor release supports |
| RHEL 9 and compatible | 5.14 with backports | v2 | `CAP_BPF`, `CAP_PERFMON`, BTF expected |
| RHEL 10 and compatible | 6.12 | v2 | As RHEL 9, with newer helpers available |
| aarch64 variants | Same lines | Same | Separate build and test lane; verify uprobe and BTF behaviour per architecture |

The feature probe, not the version string, decides what runs. Version numbers are used only for documentation and the test matrix.

### 6.4 Fallback rules

- A missing capability or SELinux denial for one probe disables that probe only, with an explicit `denied` status and the audit message captured in the diagnostics bundle.
- Lockdown mode or a disabled BPF subsystem moves the agent to Tier 3 at startup.
- If the BPF verifier rejects an object on a given kernel, the object is marked `unsupported` for that kernel build and not retried until the kernel or agent version changes.
- Tier 3 and Tier 4 are fully supported modes, not error states, and are tested in the release matrix.

## 7. Collection modules

Each module is independently enabled, has its own privilege requirements, and reports its own status. A module failure never stops the others.

### 7.1 Host discovery (unprivileged)

Runs in the `discovery` process as an unprivileged user with read access to procfs, sysfs and the systemd D-Bus API.

- Machine identity, OS release, kernel, architecture, uptime, boot parameters and eBPF compatibility flags.
- CPU topology and throttling counters, memory breakdown, hugepages.
- Block devices, partitions, filesystems with capacity and mount options.
- Network interfaces, MTU, addresses, bonding and bridge topology.
- systemd units: active and sub state, restart counts, failed units. Unit enumeration is restricted by the policy to named units or patterns.
- Container and process inventory through cgroup paths and the runtime's read-only socket where the policy allows it.

The DMI product UUID is readable only by root on most systems. When unreadable, the agent falls back to `machine-id` and reports the reduced identity confidence. It does not widen its privileges to read it.

Inventory is sent as deltas. A full snapshot is produced on first start, on `ResyncRequest`, and at a low-frequency interval (default 6 hours) as a consistency check.

### 7.2 Upstream OBI wrapper

The agent supervises the upstream OpenTelemetry eBPF Instrumentation component rather than carrying its own protocol parsers.

- The wrapper launches OBI as a managed child with the capabilities and seccomp profile the `ebpf` process holds, its configuration generated from the effective policy.
- Instrumentation is selected per process, using executable path, cgroup or container label, listening port or Kubernetes workload selector. The default selection is empty.
- RED metrics (request rate, errors and duration) for HTTP, gRPC and SQL are collected only for selected processes.
- The pinned OBI version, build and supported kernel features are recorded in the agent release notes and the capability report. Upgrading OBI is a normal agent release with the full test matrix.
- Where OBI changes its packaging or configuration surface, only the wrapper changes. The control-plane protocol is unaffected.
- `traceparent` injection is not part of the baseline. If offered later, it requires an explicit per-service policy entry, a separate authorization, and a documented statement that it modifies traffic.

### 7.3 Flow table topology (custom eBPF)

The one custom eBPF component is a compact connection tracker that yields workload-to-workload edges.

- Hooks: `connect`, `accept` and TCP state transitions, with socket-level `srtt`, retransmit and drop counters read from kernel structures at sample time.
- Flows are keyed on **(source workload, destination workload, destination port, protocol)**. Ephemeral source ports are never part of the key.
- Workload identity is resolved on the host from cgroup ID, container ID and Kubernetes pod metadata, falling back to executable and systemd unit.
- The flow table is updated in the kernel or in the `ebpf` process and flushed as a `FlowDelta` every 10 to 30 seconds (default 15). A delta carries counts, bytes, RTT summary (minimum, mean and a high percentile), retransmits and first and last seen times.
- Service virtual IPs are resolved to backing endpoints at flush time using conntrack and the endpoint map, so edges point at workloads rather than ClusterIPs (see section 12).
- Edges that do not reappear within their TTL (default 10 minutes) are closed by the control plane, which keeps churn low in the graph store.
- IPv6, UDP, loopback and sidecar traffic are included as separate flow classes and can be disabled by policy.

### 7.4 DNS observation

DNS latency and failure are common root causes and are collected explicitly.

- Resolver query count, response codes and latency per client workload and per resolver, aggregated into histograms.
- Query names are recorded only as a registered domain or an allowlisted suffix by default. Full names require a policy entry.
- Dropped or timed-out lookups are exposed as a distinct signal rather than merged with NXDOMAIN.

### 7.5 Host metrics and logs

The agent does not duplicate an existing collector.

- When a site already runs node exporters or an OpenTelemetry Collector, the agent only records their presence in inventory and relies on them for host metrics.
- When none is present, an optional bundled host-metrics module provides CPU, memory, disk, filesystem, network and pressure-stall data at a fixed interval (default 30 seconds).
- Selected journal and file logs can be read through an optional module with an explicit source allowlist, line-length limit and rate limit. Log content is treated as untrusted (section 8).
- Where a full pipeline is needed, the agent can be built as a custom OpenTelemetry Collector distribution that embeds these receivers, giving batching, retries and persistent queues. This is an implementation option, not a change to the protocol.

## 8. Redaction and data protection

The v1.2 statement that TLS uprobes give "zero credential exposure" is replaced by an enforced model: sensitive data may pass through agent memory, but redaction happens on the host before anything is buffered, written to disk or sent, and nothing is inspected unless a policy entry selects it.

### 8.1 Principles

1. **Opt-in by process.** TLS inspection and SQL parsing are off for every process until the effective policy names it. There is no global `monitor_tls: true`.
2. **Allowlist, not blocklist.** Only named fields leave the parser. Everything else is discarded in the same function that read it.
3. **Redact in the agent.** The control plane is not relied on to scrub data it should never have received.
4. **No raw payload storage.** Payload bytes are never written to the WAL, logs or diagnostics bundles.
5. **Bounded memory.** Capture buffers have fixed size, are zeroed after use and are never swapped (locked or excluded by `MADV_DONTDUMP` and an agent-wide `mlock` budget where practical).

### 8.2 Field policy

| Protocol | Kept | Never kept |
| --- | --- | --- |
| HTTP | Method, route template, status class and code, duration, request and response size, protocol version | Query strings, header values (including `Authorization`, `Cookie`, `Set-Cookie`), bodies |
| gRPC | Service and method, status code, duration, message sizes | Metadata values, message contents |
| SQL | Operation verb, normalized statement hash, optional normalized text with literals replaced, duration, rows affected, error class | Literal values, bind parameters, raw statement text leaving the host |
| DNS | Registered domain or allowlisted suffix, response code, latency | Full query names unless allowlisted, client payloads |
| TLS | Library and version, handshake outcome, cipher class | Keys, master secrets, session tickets, certificate private material |

SQL normalization strips literals and collapses `IN` lists before hashing. The hash and the normalized form are the only representations that leave the `ebpf` process.

Route templating (for example `/users/:id`) uses a bounded rule set and a fallback of `/*` for unknown high-cardinality paths, so identifiers never become labels.

### 8.3 Enforcement points

- The parser runs in the `ebpf` process, which has no network egress and cannot write outside its state directory. It hands only schema-validated records to the supervisor over a Unix socket.
- The supervisor refuses any record that does not match the allowed schema for its class.
- Policy entries that enable TLS inspection or full DNS names are logged to the control-plane audit trail with the actor, scope and time.
- A host-local switch (`uptimefoundry-agentctl disable tls-inspection`) can turn inspection off immediately, independent of the control plane.

### 8.4 Untrusted content and AI analysis

Log lines, headers, error strings and SQL text all originate from workloads the platform does not control.

- Every record that can contain free text carries a `content_trust: untrusted` marker.
- The control plane passes such text to any language model as quoted data inside a clearly delimited block, never merged with instructions, and strips or flags control sequences before display.
- Free-text fields have a hard length limit and character-class filtering at the agent, so one hostile line cannot become an unbounded payload.

### 8.5 Data at rest on the host

- Buffer and state directories are `0700`, owned by the agent user, on a filesystem path chosen at install time.
- The WAL stores only already-redacted records. Optional encryption at rest can be enabled with a key held in the kernel keyring or TPM.
- Core dumps are disabled for agent processes. The diagnostics bundle (section 14) excludes keys, tokens and payload content by construction.

## 9. Local policy ceiling and configuration

A compromised control plane must not be able to turn every agent into a fleet-wide root-equivalent tool. Each host therefore carries a root-owned **local policy** that defines the maximum the agent may ever do. The control plane supplies a **remote policy** that can only narrow it.

### 9.1 Effective policy

```
effective = local_policy  INTERSECT  remote_policy
```

- A remote setting that asks for something the local policy does not allow is ignored and reported in the next heartbeat as `policy_conflict`.
- If the remote policy is unavailable, the agent runs on the last accepted remote policy, or on a safe default (discovery only) if none exists.
- Setting a boolean or list to a value absent from the ceiling is not an error that stops the agent; it is a reported conflict.
- The local policy file is `/etc/uptimefoundry-agent/policy.yaml`, mode `0640`, owner `root`, group `uptimefoundry-agent`. The agent cannot modify it.

### 9.2 Local policy keys

| Key | Meaning | Default |
| --- | --- | --- |
| `modules.<name>.allowed` | Which modules may run | discovery only |
| `ebpf.max_tier` | Highest permitted tier (1 to 4) | 3 |
| `ebpf.tls_inspection.allowed_processes` | Executable paths or cgroup selectors allowed for TLS inspection | empty |
| `ebpf.sql_parsing.allowed_processes` | As above for SQL | empty |
| `ebpf.traceparent_injection` | Experimental header injection | false |
| `dns.full_names` | Allow recording full query names | false |
| `logs.sources` | Allowlisted journal units and file paths | empty |
| `tasks.allowed_types` | Permitted task types from the closed set | none |
| `egress.endpoints` | Control-plane endpoints, with CA bundle or pins | required |
| `limits.cpu_quota_percent`, `limits.memory_max_mb`, `limits.buffer_max_mb` | Hard resource ceilings | 5, 256, 512 |
| `kill_switch.file` | Path whose presence disables non-discovery modules immediately | `/etc/uptimefoundry-agent/disable` |

### 9.3 Example local policy

```yaml
version: 1
modules:
  discovery: { allowed: true }
  flows:     { allowed: true }
  obi:       { allowed: true }
  hostmetrics: { allowed: false }
  logs:      { allowed: false }
ebpf:
  max_tier: 1
  tls_inspection:
    allowed_processes:
      - exe: /usr/sbin/nginx
  sql_parsing:
    allowed_processes:
      - exe: /usr/pgsql-16/bin/postgres
  traceparent_injection: false
dns:
  full_names: false
tasks:
  allowed_types: []
egress:
  endpoints:
    - address: uptimefoundry.example.internal:8443
      ca_file: /etc/uptimefoundry-agent/pki/ca.pem
limits:
  cpu_quota_percent: 5
  memory_max_mb: 256
  buffer_max_mb: 512
```

### 9.4 Closed task set

Remote tasks are a short list of typed, parameterized operations. There is no shell, no arbitrary file read and no free-form command string. Each task is signed by the control plane's task-signing key, carries an expiry and a unique ID, and is rejected if the agent's local policy does not list its type.

| Task type | Purpose | Side effects |
| --- | --- | --- |
| `collect_diagnostics` | Produce a bounded, redacted diagnostics bundle | Writes to the agent's own state directory only |
| `resync_inventory` | Send a full snapshot | None |
| `set_probe_state` | Enable or disable a named probe within the ceiling | Changes agent behaviour only |
| `capture_flow_sample` | Bounded, time-limited flow sampling for one workload | None on the host |
| `restart_module` | Restart one agent process | Agent processes only |

Remediation actions on the host (restarting services, draining nodes and similar) are not tasks of this agent. They belong to the separate actuator in section 3.4, which is off by default and not part of the observation binary.

### 9.5 Remote configuration

Operational tuning that does not change privilege travels in the remote policy: sampling intervals, flush intervals, probe selections inside the ceiling, route-template rules, cardinality caps and log filters. Every remote change is versioned, signed, applied atomically, and reverted to the previous version if the agent fails health checks within 60 seconds of applying it.

## 10. Buffering, delivery and backpressure

The agent survives control-plane and network outages with a bounded on-disk write-ahead log (WAL), and it accounts for every record it drops.

### 10.1 Message classes and priority

| Class | Priority | Buffering | Under pressure |
| --- | --- | --- | --- |
| `Register`, `Heartbeat`, `TaskResult` | Highest | Memory, small | Never dropped |
| `InventoryDelta` | High | WAL | Coalesced into a newer snapshot rather than dropped |
| `FlowDelta` | Medium | WAL | Oldest deltas merged, then dropped with a counter |
| `EventBatch` | Medium | WAL, rate limited | Dropped by severity, lowest first |
| `MetricBatch` | Low | WAL, short retention | Dropped oldest first |

### 10.2 WAL rules

- The WAL lives under `/var/lib/uptimefoundry-agent/wal/`, split into fixed-size segments (default 16 MiB) with a checksum per record.
- Total size is capped by `limits.buffer_max_mb` (default 512). A separate free-space floor (default 10 percent of the filesystem) stops writing before the agent can fill the disk.
- Segments are deleted once the server acknowledges every sequence in them. A maximum age (default 24 hours, per class) applies even if unacknowledged.
- On crash recovery the agent replays unacknowledged segments, relying on server-side deduplication by sequence number.
- Corrupt records are skipped and counted, never fatal.

### 10.3 Aggregation and delta policy

| Data | Interval | Method |
| --- | --- | --- |
| Flows | 10 to 30 seconds, default 15 | Delta with counters, resync fallback after any gap |
| Inventory | On change, minimum spacing 5 seconds | Delta, full snapshot every 6 hours |
| Heartbeat | 15 seconds | Small fixed payload |
| Host metrics | 30 seconds | Batched, relayed |
| RED metrics | Pre-aggregated histograms over 15 seconds | Never one event per request |

Raw events are aggregated on the host. The system does not transmit per-request or per-connection events.

### 10.4 Backpressure

1. A slow or unreachable server fills the WAL toward its cap.
2. At 70 percent of the cap, low-priority classes switch to coarser aggregation intervals.
3. At 90 percent, `MetricBatch` and low-severity events are dropped and counted per class.
4. At 100 percent, only high-priority classes are written and flow data is reduced to periodic full snapshots.
5. When the server returns, the agent drains oldest-first within a rate limit, so a mass reconnect does not overload the control plane.

Every drop is recorded in `uptimefoundry_agent_dropped_total{class,reason}` and summarised in the next heartbeat. The control plane marks any investigation window that overlaps a recorded loss as incomplete, rather than treating the gap as quiet.

### 10.5 Offline mode

If the stream is down beyond certificate expiry, the agent stays in offline mode: it keeps collecting within the budget above, does not accept any unauthenticated instructions, and retries renewal and reconnection with capped backoff within the grace period defined in section 4.3. It does not discard its identity.

## 11. Resource governance and self-protection

v1.2 gave two different CPU figures (under 2 percent of a core, and under 1.5 percent at 100,000 events per second). v1.3 replaces them with one budget per process that is enforced by systemd and verified by test.

### 11.1 Budgets

The figures below are design targets until the benchmark lane in section 15 has produced measured values. They are then replaced by tested numbers in the release notes.

| Process | CPU (steady) | CPU (ceiling) | Memory (RSS) | Disk |
| --- | --- | --- | --- | --- |
| `supervisor` | 0.5 percent of one core | 2 percent | 64 MiB | WAL up to `buffer_max_mb` |
| `discovery` | 1 percent | 5 percent during a scan | 64 MiB | State under 50 MiB |
| `ebpf` (flows only) | 2 percent | 5 percent | 128 MiB | State under 50 MiB |
| `ebpf` (flows plus OBI) | 5 percent | 10 percent | 384 MiB | State under 100 MiB |

Ceilings are enforced with systemd `CPUQuota`, `MemoryHigh` and `MemoryMax`. In Kubernetes the same values become resource requests and limits.

### 11.2 Watchdog and shedding

The supervisor samples each child's CPU, memory, ring-buffer drops and event rate every 5 seconds. When a threshold is crossed it sheds load in a fixed order and reports each step.

1. Stop optional modules: host metrics, log reading.
2. Raise flush intervals and sampling ratios.
3. Turn off protocol parsing and TLS inspection (Tier 2).
4. Reduce the flow table to top-N flows by bytes.
5. Detach flow probes and fall back to procfs sampling (Tier 3).

Recovery is deliberately slow: the agent returns to a higher state only after 10 minutes below threshold, and at most one step at a time.

### 11.3 Kill switches

| Level | Mechanism | Effect |
| --- | --- | --- |
| Per probe | Policy entry or `uptimefoundry-agentctl probe disable <name>` | Detaches that probe |
| Per module | Policy or control-plane command within the ceiling | Stops the module |
| Whole agent, non-discovery | Presence of the kill-switch file | Detaches all eBPF and optional modules within 5 seconds |
| Whole agent | `systemctl stop uptimefoundry-agent` | Full stop; BPF programs are detached on process exit |

All eBPF programs are loaded without pinning by default, so they disappear when the owning process exits. Pinning to `/sys/fs/bpf` is an optional mode with a startup check that removes stale pins.

### 11.4 Cardinality control

- Metric labels are limited to a fixed schema: workload, route template, method, status class, protocol and port. Raw paths, user identifiers and arbitrary header values are never labels.
- Each metric has a series cap per agent (default 5,000). Series beyond the cap are folded into an `other` bucket and counted.
- Route templates are learned only from a bounded rule set. Unknown paths map to `/*`.
- High-volume paths can be sampled by policy, with the sampling rate recorded in the metric so rates can be corrected downstream.
- Flow keys exclude ephemeral ports and are limited to a configurable number of distinct destinations per source (default 2,000).

### 11.5 Ring buffer and event loss

Ring-buffer drops are counted in-kernel where supported and reported as `uptimefoundry_agent_ebpf_dropped_events_total{probe}`. A sustained drop rate above 1 percent for 60 seconds triggers shedding step 2 or 3 automatically. Loss counters are always included in the heartbeat so the control plane can mark affected evidence as incomplete.

## 12. Kubernetes, OpenShift and Swarm specifics

Container platforms change what the agent sees and what it must be allowed to do. This section replaces the v1.2 DaemonSet description, which mounted `/sys/kernel/debug` and did not state the pod security requirements.

### 12.1 DaemonSet requirements

| Requirement | Reason | Notes |
| --- | --- | --- |
| `hostPID: true` | Map PIDs and cgroups to containers; attach uprobes to other processes | Needed only by the `ebpf` container |
| `hostNetwork: true` | Observe node-level and host-namespace sockets | Optional; flows for pod network namespaces work through the host's BPF hooks |
| Capabilities | `BPF`, `PERFMON`, `SYS_RESOURCE` (older kernels), `SYS_PTRACE` or `DAC_READ_SEARCH`, `NET_ADMIN` only if a feature needs it | Dropped for the discovery container |
| `/sys/fs/bpf` mount | Only if pinning is enabled | Off by default |
| `/sys/kernel/btf` | Read-only, for BTF | Usually already readable on the host |
| `/sys/kernel/debug` | Not mounted | Removed from v1.2; BTF and tracefs rarely need it |
| Host paths | `/proc` and `/sys` read-only, runtime sockets read-only where allowed | Never the writable Docker socket |
| `readOnlyRootFilesystem` | Required | Writable only on the state and WAL volumes |
| Resource requests and limits | From section 11 budgets | Set in the chart |

The DaemonSet runs as two containers in one pod, one unprivileged for discovery and one with the narrow capability set for eBPF, sharing a Unix socket through an `emptyDir` volume. The seccomp profile for the `ebpf` container is shipped with the chart and is not `Unconfined`.

### 12.2 Pod security and OpenShift

- The target namespace must permit the privileged or custom pod security level the DaemonSet needs, and the chart documents the exact label or admission exception.
- For OpenShift, the chart ships a dedicated `SecurityContextConstraints` object granting only the capabilities and host access listed above, and binds it to the agent's service account.
- RBAC for the agent's service account is read-only: `get`, `list` and `watch` on pods, nodes, namespaces, services and endpoints (or EndpointSlices). It has no access to Secrets or ConfigMap contents.

### 12.3 Service virtual IP resolution

A `connect()` call sees the ClusterIP, not the backing pod. Without resolution, `CONNECTS_TO` edges would point at Services and hide the real dependency.

1. Maintain a local map of Service IP and port to endpoint set, from EndpointSlices received through the control plane, so the agent needs no direct Kubernetes API credential beyond what the chart grants.
2. At flush time, resolve the destination for each flow through conntrack (the post-NAT tuple) first, and the endpoint map second.
3. Record both the Service and the resolved workload. The edge targets the workload and carries the Service as an attribute.
4. If resolution is ambiguous (for example a headless Service with multiple endpoints), emit one edge per endpoint with a lower confidence value instead of picking one.

Swarm's virtual IPs and ingress routing mesh have the same issue. The Swarm resolver uses the task and VIP data from the Docker API (read-only) and the host's conntrack table in the same order.

### 12.4 PID, cgroup and container mapping

- Identify containers by cgroup ID and path, not by PID, since PIDs are reused and differ between namespaces.
- Support both cgroup v1 (RHEL 8) and cgroup v2 (RHEL 9 and 10). The mapping layer is tested against both, including systemd-managed cgroup drivers.
- Resolve cgroup to container, then container to pod or Swarm task, through the runtime's read-only API (CRI or the Docker socket where allowed). Fall back to cgroup path parsing when the runtime socket is not available.
- Record network namespace and PID namespace IDs so that localhost and sidecar traffic can be attributed correctly.

### 12.5 Identity in clusters

Pod and node names are attributes of a workload. Workload identity uses owner references (Deployment, StatefulSet, DaemonSet) and namespace, so that pod churn does not create new topology nodes. The agent does not trust labels supplied by workloads for security decisions.

## 13. Packaging, SELinux, updates and version skew

### 13.1 Packages and layout

| Path | Content |
| --- | --- |
| `/usr/bin/uptimefoundry-agent` | Supervisor |
| `/usr/libexec/uptimefoundry-agent/` | `discovery`, `ebpf` and (separate package) `actuator` helpers; embedded BPF objects with hash verification |
| `/etc/uptimefoundry-agent/policy.yaml` | Local policy ceiling, root-owned |
| `/etc/uptimefoundry-agent/pki/` | CA bundle and optional pins |
| `/var/lib/uptimefoundry-agent/` | Identity, WAL and state, owned by the agent user |
| `/usr/lib/systemd/system/uptimefoundry-agent.service` | Supervisor unit |

The `actuator` ships as its own RPM (`uptimefoundry-agent-actuator`) with its own unit, disabled by default, so a host that does not install it has no remediation code on disk.

### 13.2 systemd hardening

The unit for each process sets the following where compatible with its function:

- `User=uptimefoundry-agent`, `NoNewPrivileges=yes`, `ProtectSystem=strict`, `ProtectHome=yes`, `PrivateTmp=yes`, `ProtectKernelModules=yes`, `ProtectControlGroups=read-only` where a function allows it.
- `CapabilityBoundingSet` and `AmbientCapabilities` set to the per-process list in section 3.2 and nothing else.
- `SystemCallFilter` allowlist per process, with `SystemCallArchitectures=native`.
- `ReadWritePaths=/var/lib/uptimefoundry-agent`, `RestrictAddressFamilies` (the `ebpf` process gets only `AF_UNIX` and `AF_NETLINK`; only the supervisor gets `AF_INET` and `AF_INET6`).
- `MemoryDenyWriteExecute=yes`, `LockPersonality=yes`, `RestrictNamespaces=yes` for the parts that do not load BPF programs. BPF loading requires verification that these options do not block it on each supported kernel.
- `CPUQuota`, `MemoryHigh` and `MemoryMax` from section 11.

### 13.3 SELinux

RHEL runs SELinux in enforcing mode, so the agent ships a policy module rather than relying on `unconfined_service_t`.

- Separate domains: `uptimefoundry_agent_t` (supervisor), `uptimefoundry_agent_discovery_t` and `uptimefoundry_agent_ebpf_t`, with `uptimefoundry_agent_actuator_t` in its own package.
- The `ebpf` domain is allowed `bpf`, `perfmon` and the specific `/proc` reads it needs; it is denied network socket creation.
- The supervisor domain is the only one allowed outbound TCP to the control-plane port type.
- File contexts are defined for the state, WAL and identity directories.
- Denials are collected during testing with `audit2allow` only to inform review; no blanket `allow` rules are merged.
- Booleans or optional modules grant additional access where a site enables them, for example reading a runtime socket.

The capability report includes the SELinux mode and the domain each process is running in, and any AVC denial affecting a probe is reported in the diagnostics bundle.

### 13.4 Signing, provenance and updates

- RPMs are signed with the project's GPG key; the repository metadata is signed. Container images are signed (cosign or equivalent) and referenced by digest in the chart.
- Each release publishes an SBOM and build provenance, and the BPF object hashes are listed in the release notes.
- Updates arrive only through DNF and the package system, or through the Helm chart. The agent never downloads or replaces its own binaries.
- Package scripts restart the supervisor, which re-reads the capability report so changed kernels or policies are reflected immediately.

### 13.5 Version skew

| Pair | Policy |
| --- | --- |
| Agent and control plane | Control plane supports agents at the current and previous protocol version (N and N-1) |
| Agent processes among themselves | Same package version required; the supervisor refuses a mismatched helper |
| Agent and OBI | Pinned per agent release |
| Agent and chart | Chart pins the image digest; mixed versions during a rollout are tolerated within N and N-1 |

A control plane upgrade that would drop N-1 support must be announced in the release notes at least one minor release in advance.

## 14. Agent self-observability

The agent must be diagnosable without access to the observability stack it feeds, and without exposing a network listener.

### 14.1 Local interfaces

- `uptimefoundry-agentctl status` reports tier, probe states, policy version, last control-plane contact, buffer depth and drop counters over the supervisor's Unix socket. Access requires membership in the `uptimefoundry-agent` group or root.
- `uptimefoundry-agentctl policy show` prints the effective policy and any conflicts between local and remote settings.
- `uptimefoundry-agentctl diagnostics` creates the bundle described below.
- No TCP port is opened for health or metrics by default. A Prometheus-format endpoint on a Unix socket or loopback can be enabled by local policy for sites that scrape agents.

### 14.2 Agent metrics

All metrics use bounded labels. Probe, class and reason values come from fixed enumerations.

| Metric | Labels | Meaning |
| --- | --- | --- |
| `uptimefoundry_agent_tier` | none | Current degradation tier (1 to 4) |
| `uptimefoundry_agent_probe_state` | `probe`, `state` | Probe status per the capability report |
| `uptimefoundry_agent_dropped_total` | `class`, `reason` | Records dropped before delivery |
| `uptimefoundry_agent_ebpf_dropped_events_total` | `probe` | Kernel ring-buffer losses |
| `uptimefoundry_agent_wal_bytes` | none | Current WAL size |
| `uptimefoundry_agent_unacked_messages` | `class` | Messages awaiting acknowledgement |
| `uptimefoundry_agent_stream_connected` | none | 1 when the control-plane stream is up |
| `uptimefoundry_agent_cert_expiry_seconds` | none | Time until certificate expiry |
| `uptimefoundry_agent_process_cpu_seconds_total`, `uptimefoundry_agent_process_resident_bytes` | `process` | Per-process resource use |
| `uptimefoundry_agent_shed_level` | none | Current watchdog shedding step (0 to 5) |
| `uptimefoundry_agent_policy_conflicts` | none | Count of remote settings rejected by the ceiling |

The same values are included in the heartbeat, so the control plane has them even when no scraper reaches the host.

### 14.3 Logs

Structured logs go to the journal with fields for component, severity, request or task ID and error class. Logs never contain payload content, header values, SQL text, tokens or private key material. Repeated errors are rate-limited with a suppression counter.

### 14.4 Health semantics

| State | Meaning |
| --- | --- |
| `healthy` | Stream connected, expected probes active, no recent loss |
| `degraded` | Running at a lower tier, shedding, or losing data, but collecting |
| `disconnected` | Collecting and buffering without a control-plane stream |
| `blocked` | Cannot start a required component, for example expired identity beyond the grace period |

A temporary control-plane or backend outage never causes the agent to exit, and a failure of one module never takes down the supervisor.

### 14.5 Diagnostics bundle

The bundle is a compressed archive written to the agent's state directory, containing the capability report, effective policy with secrets masked, probe states, recent redacted logs, AVC denial summaries, BPF verifier errors, buffer statistics and version information. It excludes keys, tokens, payloads and any captured text. It is transferred only when an operator copies it or approves a `collect_diagnostics` task upload.

## 15. Testing strategy and acceptance criteria

### 15.1 Test lanes

| Lane | What it covers | Environment |
| --- | --- | --- |
| Unit | Redaction, route templating, SQL normalization, policy intersection, WAL, sequencing, flow-table aggregation | CI, any Linux |
| Fuzz | Every parser that handles workload-controlled bytes: HTTP, gRPC framing, PostgreSQL and MySQL wire formats, DNS, journal and log lines, policy YAML, task messages | Go native fuzzing, nightly with a persisted corpus |
| Kernel matrix | Probe attachment, verifier acceptance and tier selection | QEMU or `vmtest` plus real hosts: RHEL 8.x, RHEL 9.x, RHEL 10, Rocky, Alma; x86\_64 and aarch64 |
| Privilege | Each process runs with only its listed capabilities and syscalls, under enforcing SELinux | RHEL VMs with enforcing mode and AVC capture |
| Protocol | Enrollment, renewal, revocation, resync, duplicate delivery, version skew N and N-1 | Control plane test harness |
| Fault injection | Network loss, slow server, disk full, clock skew, corrupt WAL, process kill, mass reconnect | Chaos harness |
| Overhead | CPU, memory and loss under defined load | Dedicated benchmark hosts |
| Container platforms | Kubernetes (cgroup v1 and v2), OpenShift SCC, Swarm VIP resolution, ClusterIP and headless Services | Test clusters |
| Security | Credential leakage, local IPC abuse, replay, hostile workload traffic, prompt-injection strings in logs | Red-team scripts and review |

### 15.2 Security-specific tests

- Seed a workload with known secrets in headers, cookies, SQL literals and query strings. Assert none appear in the WAL, logs, diagnostics bundle or any message sent to the control plane.
- Attempt to widen collection from the control plane beyond the local policy. Assert that the agent reports `policy_conflict` and makes no change.
- Send task messages that are unsigned, expired, replayed or of an unlisted type. Assert refusal and audit.
- Clone a VM image after enrollment. Assert the second agent is refused and an alert is raised.
- Feed oversized, malformed and truncated protocol traffic to every parser under fuzzing and in the integration lane.
- Place instruction-like text in logs and headers. Assert it is marked untrusted end to end.

### 15.3 Measurable acceptance criteria

Criteria use fixed workload profiles that are written into the repository. Numeric targets below are provisional until the first benchmark run, and then become gating thresholds.

| # | Criterion | Target |
| --- | --- | --- |
| 1 | Agent starts and reaches a defined tier on every kernel in the matrix | 100 percent, no crashes |
| 2 | Steady-state CPU, flows only, 10,000 connections per second | At most 2 percent of one core |
| 3 | Steady-state CPU, flows plus OBI on opted-in processes, 5,000 requests per second | At most 5 percent of one core |
| 4 | Memory under the same loads | Within the section 11 RSS budgets |
| 5 | Event loss at the stated load | Below 0.1 percent, and always counted |
| 6 | Reconnect after a 30-minute control-plane outage | No data loss within the WAL cap; drain completes without exceeding the rate limit |
| 7 | Secret leakage test | Zero seeded secrets observed anywhere off the host |
| 8 | Policy ceiling test | Zero actions outside the local policy |
| 9 | Fuzzing | Zero crashes or hangs after the agreed corpus and run time |
| 10 | Enrollment through renewal and revocation | All state transitions behave as in section 4 |
| 11 | Tier 3 and Tier 4 fallback | Correct labelling, no parse errors, no missing heartbeat |
| 12 | SELinux enforcing | No unreviewed AVC denials during the full test run |

### 15.4 Release gate

A release candidate must pass every lane on every supported platform, with the benchmark results recorded in the release notes. A regression in any measured target blocks the release unless the change is documented and approved.

## 16. Roadmap A1 to A5 with exit criteria

The agent is delivered in five steps that map onto platform milestones M2 to M9. Each step must pass its gate before the next begins, and the actuator is the last step because it needs the approval and audit framework.

&#91;embedded content: agent roadmap · 5 steps, 4 exit gates\]

| Step | Deliverables | Exit criteria (section 15.3 numbers) |
| --- | --- | --- |
| A1 Foundation | Supervisor and process split, enrollment, mTLS stream, renewal and revocation, heartbeat, capability report, unprivileged host discovery, WAL buffering, `uptimefoundry-agentctl status` | 1, 10 and 11 pass; a 30-minute outage loses no data inside the WAL cap (6) |
| A2 OBI | Supervised upstream OBI for RED metrics on opted-in processes, redaction pipeline, SQL normalization, route templating, fuzzing of every parser | 7 and 9 pass; cardinality caps hold under the benchmark profile |
| A3 Topology | Flow table with delta sync, Service and Swarm VIP resolution, cgroup v1 and v2 mapping, DNS observation, edge TTLs | 2 and 5 pass; flow edges in Kubernetes and Swarm tests target workloads, not virtual IPs |
| A4 Hardening | SELinux policy module, systemd hardening, DaemonSet and OpenShift SCC, full degradation ladder, watchdog and shedding, signed RPMs, SBOM | 3, 4, 11 and 12 pass on every platform in the kernel matrix |
| A5 Actuator | Separate actuator package, signed and approved action types, dry-run, audit trail | 8 passes for actions; platform M9 approval and audit framework is complete and reviewed |

A1 is the first vertical slice and should be built alongside platform M2, so the control plane's ingestion API and the agent's protocol are tested together from the start.

## 17. Required changes to the platform spec and open questions

### 17.1 Changes to v1.2 outside the agent

| v1.2 location | Change |
| --- | --- |
| Section 3 and 9, agent writes to PostgreSQL | Replace with the control-plane ingestion API; the control plane owns all database writes |
| Section 10, agent streams OTLP to a local VictoriaMetrics endpoint | Replace with the metrics relay (section 5.5) and keep `127.0.0.1:8428` only as an optional local-collector exception |
| Section 15 configuration, `ebpf.monitor_tls: true` and `capture_sql: true` | Move to the local and remote policy model in section 9 and default both off |
| Section 20 security, `CAP_BPF` and `CAP_NET_ADMIN` only | Replace with the per-process capability table and SELinux requirements |
| Section 22, DaemonSet mounts `/sys/kernel/debug` | Remove the mount and adopt section 12 |
| Section 16, `POST /topology/cypher` | Run under a read-only database role with statement timeout and a query allowlist, since the API alone cannot guarantee read-only behaviour |
| Section 23 and 9.5, two CPU figures | Replace with the tested budgets in section 11 |
| Section 26, latency and similarity criteria | Replace with measured service-level targets and retrieval quality metrics (Recall@K, MRR) on a labelled set |
| Ports | Resolve the clash between MCP SSE and the embedding endpoint (both 8081) and the control plane (8080) with one documented port plan |
| Binary and endpoint names | Standardize `uptimefoundry-server` or `uptimefoundry`, `uptimefoundry-cli` or `uptimefoundryctl`, and `/health/*` or `/healthz` across all documents |
| Section 20, "cryptographic authorization" | Define signing keys, key holders, signed content and verification for tasks, policy and remediation approval |
| v1.1 safeguards | Restore TLS-required remote endpoints, no insecure TLS by default, SSRF protection, partial-result handling, retention awareness and provider failure isolation |
| Graph store | Consider recursive queries on the relational `relationships` table for the first release and add Apache AGE only if multi-hop performance requires it |

### 17.2 Open questions

1. **RHEL 8 privilege model.** Which exact RHEL 8 minor release and kernel will be supported for eBPF, and does it require full `CAP_SYS_ADMIN` for BPF? This decides whether Tier 1 on RHEL 8 is offered or whether RHEL 8 is Tier 3 only.
2. **OBI packaging.** Which upstream OBI release and distribution form (library, binary or Collector component) will be pinned, and what are its licence and kernel requirements?
3. **Collector-based agent.** Is the agent built as a custom OpenTelemetry Collector distribution or as a standalone Go supervisor that embeds selected components?
4. **Metrics relay.** Is the relay part of the control plane or a separately deployable service for large sites?
5. **Key custody.** Who holds the task-signing and policy-signing keys, and are they kept in an HSM or a secrets manager?
6. **TPM use.** Is TPM sealing a supported option in the first release or a later hardening item?
7. **Windows and non-Linux hosts.** Confirmed out of scope, or to be reconsidered?
8. **Air-gapped sites.** What is the offline distribution process for RPMs, images and BPF objects?
9. **Retention of agent-originated data.** What retention applies to flow deltas and inventory history in the control plane?
10. **Remediation actuator.** Which first actions does the actuator support, and who approves each class?

Before implementation the facts in sections 6.3 and 13.3 about specific kernel releases, capability availability and SELinux behaviour should be re-verified on the target RHEL minor versions, since vendor backports change what a version number implies.
