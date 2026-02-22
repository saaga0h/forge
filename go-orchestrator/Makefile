.PHONY: help build run test clean docker-build docker-push deps

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

test: ## Run tests
	go test -v -race -coverprofile=coverage.out ./...

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