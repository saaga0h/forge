.PHONY: help build run test test-unit test-integration test-all clean docker-build docker-push deps deploy

APP_NAME := gpu-compute-orchestrator
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
REGISTRY := localhost:5000
IMAGE := $(REGISTRY)/$(APP_NAME)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Download dependencies
	go mod download
	go mod tidy

build: ## Build binary
	CGO_ENABLED=0 go build -ldflags="-w -s -X main.version=$(VERSION)" -o bin/orchestrator main.go

run: ## Run locally
	go run main.go

test: ## Run unit tests with race detector and coverage
	go test -v -race -coverprofile=coverage.out ./pkg/... ./...

test-unit: ## Run unit tests only (no external dependencies)
	go test -v -race -coverprofile=coverage.out ./pkg/...

test-integration: ## Run integration tests (requires docker-compose.test.yaml stack)
	docker compose -f docker-compose.test.yaml up -d --wait
	go test -tags integration -v -timeout 60s ./integration_test/ ; \
	EXIT_CODE=$$? ; \
	docker compose -f docker-compose.test.yaml down ; \
	exit $$EXIT_CODE

test-all: test-unit test-integration ## Run all tests

test-coverage: test ## Show test coverage
	go tool cover -html=coverage.out

clean: ## Clean build artifacts
	rm -rf bin/
	rm -f coverage.out

docker-build: ## Build Docker image
	docker build -t $(IMAGE):$(VERSION) -t $(IMAGE):latest .

docker-push: ## Push Docker image
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

docker-run: ## Run Docker container locally
	docker run --rm \
		-p 8080:8080 \
		-e MQTT_BROKER=tcp://host.docker.internal:1883 \
		-e NOMAD_ADDR=http://host.docker.internal:4646 \
		$(IMAGE):latest

fmt: ## Format code
	go fmt ./...
	go vet ./...

lint: ## Run linter
	golangci-lint run ./...

dev: ## Development mode with hot reload (requires air)
	air

deploy: ## Deploy Nomad jobs (requires NOMAD_ADDR and ARTIFACT_BASE env vars)
	for hcl in deploy/nomad/*.hcl; do \
		envsubst '$${ARTIFACT_BASE}' < "$$hcl" | nomad job run -; \
	done