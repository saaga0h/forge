# CLAUDE.md — GPU Compute Orchestrator

## Build & Test
```bash
make build          # → bin/orchestrator
make test           # go test -v -race ./...
make test-unit      # unit tests only (./pkg/...) with race detector + coverage
make test-integration  # spins up docker-compose.test.yaml, runs integration tests, tears down
make test-all       # test-unit + test-integration
make fmt            # go fmt + go vet
make lint           # golangci-lint
```
Go 1.23 required. Run from project root (go.mod is at root).

Integration tests live in `integration_test/` and use `//go:build integration`. They are invisible to plain `go test ./...` — the Makefile passes `-tags integration` automatically. Test stack ports are offset from dev: MQTT 11883, Nomad 14646, Orchestrator 18080.

## Dev Stack
```bash
docker-compose up -d                                # basic (MQTT + Nomad + orchestrator)
docker-compose -f docker-compose.enhanced.yaml up -d  # full (adds Consul for service discovery)
```
Both stacks build a custom Nomad dev image (`nomad-server/Dockerfile`) that auto-registers the `gpu-compute` parameterized batch job via `entrypoint.sh`.

`configs/` holds the MQTT broker config (`mosquitto.conf`) and the orchestrator app config (`config.yaml`). Docker Compose mounts the whole directory into the orchestrator container at `/app/configs`.

## MQTT Topics
```
compute/jobs/{job_id}/params    # Job parameters (retained message)
compute/jobs/{job_id}/status    # Status updates from worker
compute/jobs/{job_id}/result    # Computation result from worker
compute/jobs/{job_id}/logs      # Log output from worker
```

## Constraints
- Compute workers are NOT in this repo — they live in their own repositories and deploy via Nomad + Singularity/Docker
- `semantic_diffusion` / `diffusion` jobs publish raw CSV (Jeeves Anchor Export header format); all other operations publish JSON `JobParams`
- Nomad workers are parameterized batch jobs, dispatched via Nomad HTTP API with `job_id` as metadata
- GPU nodes: Ubuntu 24.04 LTS, AMD ROCm 7.0+, AMD Radeon AI Pro R9700 (RDNA 4), Nomad client with GPU fingerprinting