# Contributing to UptimeFoundry

<!-- SPDX-License-Identifier: MIT -->

Thank you for your interest in contributing to **UptimeFoundry** — an open-source,
self-hosted AIOps platform for autonomous SRE operations.

## Table of Contents

1. [Code of Conduct](#code-of-conduct)
2. [How to Contribute](#how-to-contribute)
3. [Development Setup](#development-setup)
4. [Code Standards](#code-standards)
5. [Testing Requirements](#testing-requirements)
6. [Commit & Sign-off](#commit--sign-off)
7. [Pull Request Process](#pull-request-process)
8. [Security Issues](#security-issues)

---

## Code of Conduct

All participants in the UptimeFoundry project are expected to follow our
[Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

---

## How to Contribute

| Type | Where |
|---|---|
| Bug report | [GitHub Issues → Bug report](.github/ISSUE_TEMPLATE/bug_report.md) |
| Feature request | [GitHub Issues → Feature request](.github/ISSUE_TEMPLATE/feature_request.md) |
| Security vulnerability | [GitHub Private Vulnerability Reporting](https://github.com/onisimchukv/uptimefoundry/security/advisories/new) — **not public issues** |
| Code contribution | Fork → branch → PR against `master` |
| Documentation | Same PR process |

---

## Development Setup

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.27+ (`go version`) |
| Docker + Compose | For integration test stack |
| `golangci-lint` | Latest — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| `govulncheck` | Latest — `go install golang.org/x/vuln/cmd/govulncheck@latest` |

### First-time setup

```bash
git clone https://github.com/onisimchukv/uptimefoundry.git uptimefoundry
cd uptimefoundry
go mod download

# Copy example config
cp configs/uptimefoundry.example.yaml configs/uptimefoundry.local.yaml
# Edit configs/uptimefoundry.local.yaml — fill in your local PostgreSQL DSN

# Start dev dependencies (Postgres, VictoriaMetrics, Loki)
docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d
```

### Building

```bash
make build           # builds all 5 binaries into ./bin/
make test            # go test -race ./...
make lint            # golangci-lint run
make vulncheck       # govulncheck ./...
```

---

## Code Standards

### License headers

All new `.go` files **must** include an SPDX header as the very first comment:

```go
// SPDX-License-Identifier: MIT
```

### Formatting

- `gofmt` / `goimports` — run before committing.
- `golangci-lint` with `gosec`, `govet`, `errcheck`, `staticcheck` enabled.
- No `panic()` in library code — return errors.
- No `log.Fatal` outside of `main()`.

### Error handling

- Wrap errors with context: `fmt.Errorf("component: operation: %w", err)`.
- Use sentinel errors (`var ErrFoo = errors.New(...)`) for errors callers must inspect.

### Sensitive data

- **Never** hardcode secrets, tokens, passwords, or private keys in source files.
- Use environment variables or the config file (which is excluded from git via `.gitignore`).
- New example config values must be clearly labeled `# CHANGE ME — dev only`.

---

## Testing Requirements

| Requirement | Command |
|---|---|
| All tests pass | `go test -race ./...` |
| Coverage ≥ 60% for new packages | `go test -cover ./...` |
| No data races | `-race` flag (enabled above) |
| No new lint errors | `golangci-lint run` |
| No known vulnerabilities in new deps | `govulncheck ./...` |

---

## Commit & Sign-off

UptimeFoundry uses the **Developer Certificate of Origin (DCO)**.
Every commit must be signed off with:

```bash
git commit -s -m "feat(agent): add circuit breaker to WAL flush"
```

This adds:
```
Signed-off-by: Your Name <you@example.com>
```

### Commit message format

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <short summary>

[optional body]

[optional footer(s)]
Signed-off-by: ...
```

Types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `ci`, `perf`, `security`

---

## Pull Request Process

1. Fork the repository and create a branch from `master`.
2. Make your changes, add tests, run `make test lint vulncheck`.
3. Fill out the [PR template](.github/PULL_REQUEST_TEMPLATE.md) completely.
4. Ensure all CI checks pass (they run automatically on PR open).
5. A project maintainer will review within **5 business days**.
6. Squash-merge is preferred for feature branches; rebase for small fixes.

---

## Security Issues

**Do not file public issues for security vulnerabilities.**

Use [GitHub Private Vulnerability Reporting](https://github.com/onisimchukv/uptimefoundry/security/advisories/new)
or email the address listed in [SECURITY.md](SECURITY.md).
