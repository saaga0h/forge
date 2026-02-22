# CLAUDE.md — GPU Compute Orchestrator

## What This Repository Is

A **Go-based orchestration service** for dispatching GPU compute jobs to Nomad-managed
worker containers. Jobs arrive via HTTP API, are published to MQTT for workers to pick
up, and are dispatched via the Nomad scheduler to GPU nodes.

This repository does **not** contain compute implementations. Workers live in their
own repositories and are deployed independently via Nomad + Singularity/Docker.

## History

This project started as **FORGE** — a proof-of-concept GPU compute platform that
included Fortran/Rust/Julia compute implementations in this repository. Those have
all been removed:

- **Fortran/Rust** (`fortran-compute/`, `compute/diffusion-nbody/`) — removed; was POC only
- **Julia semantic-n-body** (`compute/semantic-n-body/`) — moved to its own repository:
  `https://git.home.federation.fi/lavernea/semantic-n-body`
- **Python benchmarks** (`python-benchmark/`) — removed; benchmarked old compute

The remaining codebase is the orchestration layer only.

## Key Components

```
go-orchestrator/          # Main Go service
├── main.go               # HTTP server entry point
├── pkg/
│   ├── anchors/          # Parse CSV anchor data, submit diffusion jobs
│   ├── compute/          # Job lifecycle management (submit, track, cancel)
│   ├── database/         # PostgreSQL pattern storage
│   ├── mqtt/             # MQTT client (publish params, subscribe results)
│   ├── nomad/            # Nomad job dispatcher
│   └── patterns/         # LLM-assisted semantic pattern aggregation (key package)
├── cmd/
│   └── test-patterns/    # CLI tool: test LLM pattern aggregation
├── configs/
│   ├── config.yaml       # Service configuration
│   └── llm.yaml          # LLM/Ollama configuration
└── migrations/
    └── 001_create_patterns.sql

nomad-server/             # Nomad server config for local dev
nomad-jobs/               # Nomad job definitions (.hcl)
configs/                  # Mosquitto MQTT config
docker-compose.yaml       # Basic dev stack (MQTT + Nomad + orchestrator)
docker-compose.enhanced.yaml  # Full stack (adds PostgreSQL + Ollama LLM)
```

## The `pkg/patterns/` Package

This is the most sophisticated part of the codebase. It:
1. Receives semantic diffusion results (chains of behavioral anchors)
2. Aggregates them using signature hashing and temporal bucketing
3. Uses a local LLM (Ollama) to generate human-readable pattern descriptions
4. Stores patterns in PostgreSQL with caching

## Go Development

```bash
cd go-orchestrator

# Build
make build                  # → bin/orchestrator
make docker-build           # → Docker image

# Run
make run                    # run locally
docker-compose up -d        # full dev stack

# Test
make test                   # go test -v -race ./...
go test ./pkg/patterns/...  # test specific package

# Other
make fmt                    # go fmt + go vet
make lint                   # golangci-lint
```

**Go version**: 1.23

## MQTT Topic Convention

```
gpu/jobs/{job_id}/params    # Job parameters (retained message)
gpu/jobs/{job_id}/status    # Status updates from worker
gpu/jobs/{job_id}/result    # Computation result from worker
gpu/jobs/{job_id}/logs      # Log output from worker
```

## Compute Job Operations

The orchestrator handles two MQTT payload formats depending on operation:

- **`semantic_diffusion` / `diffusion`**: Publishes raw CSV anchor data directly
  (compute worker expects CSV with Jeeves Anchor Export header format)
- **All other operations**: Publishes JSON `JobParams` structure

## Nomad Job Structure

Workers are Nomad parameterized batch jobs. The orchestrator dispatches them
via the Nomad HTTP API, passing `job_id` as metadata so the worker knows which
MQTT topics to use.

Job definitions live in `nomad-jobs/`. GPU scheduling uses Nomad's device
fingerprinting with AMD ROCm support.

## Local Dev Stack

```bash
# Basic (MQTT + Nomad + orchestrator)
docker-compose up -d

# Full (adds PostgreSQL + Ollama LLM)
docker-compose -f docker-compose.enhanced.yaml up -d
```

**Note**: The enhanced stack expects Ollama running on the host at `localhost:11434`.
The orchestrator connects via `host.docker.internal:11434`.

## GPU Node Requirements

- Ubuntu 24.04 LTS
- AMD ROCm 7.0+
- Nomad client with GPU fingerprinting enabled
- Singularity/Apptainer (for containerized workers)
- AMD Radeon AI Pro R9700 (RDNA 4)

## Notes from Cleanup (2026-02-22)

Pre-existing compilation errors were found and fixed in `go-orchestrator/pkg/anchors/handler.go`:

- Wrong field name `Params` corrected to `Parameters` on `compute.Request`
- Wrong field name `DurationMs` corrected to `DurationMS` on `compute.Job`
- `FormatForFortran()` method renamed to `FormatAsCSV()` to remove Fortran-specific naming
