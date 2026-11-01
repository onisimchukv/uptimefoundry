<!-- SPDX-License-Identifier: MIT -->
# UptimeFoundry

**Self-hosted SRE and operations tooling in Go**

UptimeFoundry is in design and early implementation. The repository contains working foundations for a Go control-plane process, PostgreSQL persistence, host discovery, telemetry query clients, MCP transports, and an agent process split. The full platform described by the specifications—including production eBPF collection, end-to-end mTLS enrollment, and safe autonomous remediation—is not yet a released, production-ready product.

## What works in this repository

- The control-plane binary starts and serves `/readyz` and `/api/v1/status`.
- PostgreSQL storage, migrations, discovery repositories, and graph fallbacks are implemented in the codebase; configure a database to use the persistent path.
- Host inventory collectors and VictoriaMetrics and Loki query clients are present.
- MCP stdio is wired into the control-plane binary; the SSE transport handler and MCP client exist, but separate SSE listener startup is not yet integrated.
- Agent, discovery, eBPF-worker, and actuator command entry points exist. Their presence does not mean the complete v1.3 agent design or production eBPF probe set is implemented.

GraphRAG, eBPF/OBI coverage, Kubernetes discovery, signed remediation, and the v1.3 agent architecture should be treated as design or early implementation until the relevant behavior is verified and released. See [the specifications](docs/specs/) for the design status and version history.

## Development quick start

This quick start is for local development only. It exposes the control-plane health endpoints over plain HTTP on loopback. Do not use this configuration beyond local development. Any non-local deployment must use TLS with certificate verification; agent-to-control-plane communication must follow the mTLS requirements in the agent specification.

### Requirements

- Go 1.27.3 or newer (the repository toolchain is Go 1.27.3)
- Docker and Docker Compose

### Start the local data services

Set a throwaway development-only database password in your shell, then start the compose stack. For PowerShell:

```powershell
$env:UPTIMEFOUNDRY_DEV_DB_PASSWORD = 'replace-with-a-local-development-password'
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d
```

For a POSIX shell:

```sh
export UPTIMEFOUNDRY_DEV_DB_PASSWORD='replace-with-a-local-development-password'
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d
```

The database, VictoriaMetrics, and Loki ports are published only on `127.0.0.1`. The password is required and has no committed fallback. This stack is not hardened for production.

### Run the control plane in a container

The control plane has a repository-root Docker build context. From the repository root, build it with:

```sh
docker build --target control-plane -t uptimefoundry/control-plane:dev .
```

The development Compose file builds and starts the control plane together with PostgreSQL, VictoriaMetrics, and Loki. It publishes the control-plane port on loopback only. Provide the same development-only database password shown above, then run:

```sh
docker compose -f deploy/docker-compose/docker-compose.dev.yml up --build
```

The container listens on its internal network interface so Docker can forward traffic; the host port remains bound to `127.0.0.1`. Plain HTTP and the development database configuration are strictly for local use.

### Configure and run the control plane

Copy `configs/uptimefoundry.example.yaml` to an ignored local file such as `configs/uptimefoundry.local.yaml`. Set `database.dsn` to `postgres://opsfoundry:<your-password>@127.0.0.1:5432/opsfoundry?sslmode=disable`, keep the server bound to `127.0.0.1:8080`, and leave TLS disabled only for this local development setup.

```sh
go build -o bin/uptimefoundry ./cmd/uptimefoundry
./bin/uptimefoundry -config configs/uptimefoundry.local.yaml
```

In another terminal, check the local endpoints:

```sh
curl --fail http://127.0.0.1:8080/readyz
curl --fail http://127.0.0.1:8080/api/v1/status
```

The HTTP examples above are development-only. For any deployment accessible beyond the local machine, configure TLS and certificate verification before exposing endpoints.

## Repository layout

```text
cmd/                 Control plane, agent, CLI, and worker binaries
configs/             Example configuration
deploy/              Development Compose, systemd, Kubernetes, RPM, SELinux
docs/specs/           Versioned design specifications
internal/             Agent, control plane, storage, graph, MCP, and telemetry code
migrations/           PostgreSQL schema migrations
scripts/              Release-supporting utilities
```

## Security

Please report vulnerabilities privately. See [SECURITY.md](SECURITY.md). Do not put credentials in example configs or commit local config files. The project is not yet a supported production release; review deployment manifests and the current implementation against your security requirements before use.

## License

MIT. Source files carry the SPDX identifier `MIT` in their comment syntax (for example, `// SPDX-License-Identifier: MIT` in Go, `-- SPDX-License-Identifier: MIT` in SQL, and `# SPDX-License-Identifier: MIT` in deployment files). Markdown uses an HTML comment. Any future eBPF C program must also declare a GPL-compatible kernel program licence in its BPF `license` section, independently of its source-file SPDX header. The root [LICENSE](LICENSE) is authoritative.
