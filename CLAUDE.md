# CLAUDE.md — GPU Compute Orchestrator

## Build & Test
```bash
make build          # → bin/orchestrator
make test           # go test -v -race ./...
make fmt            # go fmt + go vet
make lint           # golangci-lint
```
Go 1.23 required. Run from project root (go.mod is at root).

## Dev Stack
```bash
docker-compose up -d                                # basic (MQTT + Nomad + orchestrator)
docker-compose -f docker-compose.enhanced.yaml up -d  # full (adds PostgreSQL + Ollama)
```
Enhanced stack expects Ollama on host at localhost:11434. Orchestrator connects via host.docker.internal:11434.
Enhanced stack builds a clean Nomad dev image (`nomad-server/Dockerfile`) that auto-registers `nomad-jobs/gpu-compute-dev.nomad.hcl` as a dispatch placeholder (raw_exec sleep; real workers live in compute repos).

`configs/` holds both the MQTT broker config (`mosquitto.conf`) and the orchestrator app configs (`config.yaml`, `llm.yaml`). Docker Compose mounts the whole directory into the orchestrator container at `/app/configs`.

## MQTT Topics
```
gpu/jobs/{job_id}/params    # Job parameters (retained message)
gpu/jobs/{job_id}/status    # Status updates from worker
gpu/jobs/{job_id}/result    # Computation result from worker
gpu/jobs/{job_id}/logs      # Log output from worker
```

## Constraints
- Compute workers are NOT in this repo — they live in their own repositories and deploy via Nomad + Singularity/Docker
- `semantic_diffusion` / `diffusion` jobs publish raw CSV (Jeeves Anchor Export header format); all other operations publish JSON `JobParams`
- Nomad workers are parameterized batch jobs, dispatched via Nomad HTTP API with `job_id` as metadata
- GPU nodes: Ubuntu 24.04 LTS, AMD ROCm 7.0+, AMD Radeon AI Pro R9700 (RDNA 4), Nomad client with GPU fingerprinting