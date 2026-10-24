.PHONY: build test lint clean docker-up docker-down

BINARY_NAME=bin/forgeops
CLI_NAME=bin/forgeops-cli
AGENT_NAME=bin/forgeops-agent
AGENTCTL_NAME=bin/forgeops-agentctl

build:
	@mkdir -p bin
	go build -o $(BINARY_NAME) ./cmd/forgeops
	go build -o $(CLI_NAME) ./cmd/forgeops-cli
	go build -o $(AGENT_NAME) ./cmd/forgeops-agent
	go build -o $(AGENTCTL_NAME) ./cmd/forgeops-agentctl

test:
	go test -v -race ./...

lint:
	go vet ./...

docker-up:
	docker compose -f deploy/docker-compose/docker-compose.dev.yml up -d

docker-down:
	docker compose -f deploy/docker-compose/docker-compose.dev.yml down

clean:
	rm -rf bin/
