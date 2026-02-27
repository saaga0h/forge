# GPU Compute Orchestrator

Distributed GPU compute orchestration: HTTP API → MQTT dispatch → Nomad scheduling → async results.

## Architecture

```
┌──────────────┐
│ HTTP Client  │
└──────┬───────┘
       │ POST /compute
       ↓
┌──────────────────────────┐
│ Go Orchestrator          │ (Docker Container)
│ - Validates request      │
│ - Publishes MQTT params  │
│ - Dispatches Nomad job   │
└────────┬─────────────────┘
         │
    ┌────┴────┐
    ↓         ↓
┌────────┐  ┌────────┐
│  MQTT  │  │ Nomad  │
│ Broker │  │ Server │
└────┬───┘  └───┬────┘
     │          │ Schedules on GPU node
     │          ↓
     │     ┌─────────────────────┐
     │     │ Nomad Client (GPU)  │
     │     │  Compute Worker     │
     │     │  (any language,     │
     │     │   any container)    │
     │     └──────────┬──────────┘
     │                │
     └────────────────┘
          MQTT Results
```

## Components

- **Go Orchestrator** — HTTP API, MQTT client, Nomad dispatcher. Go 1.23, Docker container.
- **MQTT Broker** — Eclipse Mosquitto. Async job parameters and results. Ports 1883/9001.
- **Nomad** — HashiCorp Nomad. Container scheduling on GPU nodes. Port 4646.
- **Compute Workers** — Independently deployed GPU containers, dispatched via Nomad, communicate over MQTT. Workers live in separate repositories.

## Prerequisites

**Development machine:** Docker & Docker Compose, Go 1.23+, Make

**GPU node:** Ubuntu 24.04 LTS, AMD ROCm 7.0+, Nomad client, AMD Radeon AI Pro R9700 (RDNA 4)

## Quick Start

### 1. Build

```bash
make build          # binary → bin/orchestrator
make docker-build   # Docker image
```

### 2. Start Dev Stack

```bash
docker-compose up -d
docker-compose ps
```

Starts MQTT broker, Nomad server (with `gpu-compute` job pre-registered), and the orchestrator.

### 3. Submit a Job

```bash
curl -X POST http://localhost:8080/compute \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "my_operation",
    "payload": {
      "size": 512
    }
  }'
```

## API

### `POST /compute`
Submit async job. Returns immediately with job ID and `dispatched` status.

**Request:**
```json
{
  "operation": "my_operation",
  "payload": { "size": 512 },
  "priority": "normal"
}
```

**Response (202):**
```json
{
  "id": "abc-123",
  "status": "dispatched",
  "created_at": "2026-02-27T12:00:00Z",
  "nomad_eval_id": "eval-xyz"
}
```

### `POST /compute/sync`
Submit and wait for result (blocks until complete or timeout).

### `GET /jobs`
List all jobs with count.

### `GET /jobs/{job_id}`
Get job details and result.

### `POST /jobs/cancel?job_id={id}`
Cancel a running job.

### `GET /health`
Health check — returns status, version, time.

## MQTT Topics

```
compute/jobs/{job_id}/params    # Job parameters (retained message)
compute/jobs/{job_id}/status    # Status updates from worker
compute/jobs/{job_id}/result    # Computation result from worker
compute/jobs/{job_id}/logs      # Log output from worker
```

### Worker Protocol

**Params** (orchestrator → worker, QoS 1, retained):
```json
{"job_id": "uuid", ...payload_fields}
```
The `payload` from the HTTP request is merged with `job_id` at the top level.

**Result** (worker → orchestrator, QoS 1):
```json
{
  "job_id": "uuid",
  "success": true,
  "result": { ... },
  "worker_id": "nomad-abc123",
  "timestamp": "2026-02-21T10:00:00Z"
}
```
`error` (string) and `metrics` are optional. `success: false` marks the job as failed.

## Configuration

Environment variables for the orchestrator container:

| Variable | Default | Description |
|----------|---------|-------------|
| `MQTT_BROKER` | `tcp://mqtt:1883` | MQTT broker address |
| `NOMAD_ADDR` | `http://nomad:4646` | Nomad API address |
| `NOMAD_JOB_NAME` | `gpu-compute` | Nomad parameterized job name |
| `PORT` | `8080` | HTTP listen port |

## Testing

```bash
make test-unit          # unit tests, race detector, coverage (no external deps)
make test-integration   # spins up test stack, runs integration suite, tears down
make test-all           # both
make test-coverage      # HTML coverage report
```

Integration tests use `//go:build integration` and are invisible to plain `go test ./...`.
Test stack runs on offset ports so it can coexist with the dev stack:

| Service | Dev | Test |
|---------|-----|------|
| MQTT | 1883 | 11883 |
| Nomad | 4646 | 14646 |
| Orchestrator | 8080 | 18080 |

## GPU Node Setup

```bash
# Copy Nomad client config
sudo cp nomad-jobs/nomad-client-gpu.hcl /etc/nomad.d/client.hcl
sudo systemctl restart nomad

# Verify GPU detection
nomad node status -self

# Register the compute job (first time only)
cd nomad-jobs
./setup-and-deploy.sh
```

## Monitoring

| Service | URL |
|---------|-----|
| Orchestrator + Web UI | http://localhost:8080 |
| Nomad UI | http://localhost:4646 |
| Consul UI | http://localhost:8500 (enhanced stack only) |

## Troubleshooting

### GPU not detected
```bash
rocm-smi
ls -la /dev/kfd /dev/dri/
nomad node status -self -verbose
```

### MQTT issues
```bash
mosquitto_sub -h localhost -t 'compute/jobs/#' -v
docker-compose logs mqtt
```

### Nomad dispatch fails
```bash
nomad job status gpu-compute
nomad alloc status <alloc-id>
nomad node eligibility -self
```

## License

MIT
