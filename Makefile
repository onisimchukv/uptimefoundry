# SPDX-License-Identifier: MIT

# UptimeFoundry Makefile
# SPDX-License-Identifier: MIT
# Copyright (c) 2026 UptimeFoundry Contributors

.PHONY: build test lint vulncheck sbom clean docker-up docker-down

BINARY_NAME=bin/uptimefoundry
CLI_NAME=bin/uptimefoundry-cli
AGENT_NAME=bin/uptimefoundry-agent
AGENTCTL_NAME=bin/uptimefoundry-agentctl
ACTUATOR_NAME=bin/uptimefoundry-agent-actuator

build:
	@mkdir -p bin
	go build -o $(BINARY_NAME) ./cmd/uptimefoundry
	go build -o $(CLI_NAME) ./cmd/uptimefoundry-cli
	go build -o $(AGENT_NAME) ./cmd/uptimefoundry-agent
	go build -o $(AGENTCTL_NAME) ./cmd/uptimefoundry-agentctl
	go build -o $(ACTUATOR_NAME) ./cmd/uptimefoundry-agent-actuator

test:
	go test -v -race ./...

lint:
	go vet ./...

vulncheck:
	govulncheck ./...

sbom:
	go run scripts/generate-sbom.go > uptimefoundry-sbom.json

docker-up:
	docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d

docker-down:
	docker compose -f deploy/docker-compose/docker-compose.dev.yml down

clean:
	rm -rf bin/ uptimefoundry-sbom.json
