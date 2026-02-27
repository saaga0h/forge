# GPU Compute Orchestrator

Distributed GPU compute orchestration using **Go** for the API and dispatch layer, **MQTT** for async messaging, and **Nomad** for container scheduling on GPU nodes.

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
│ - Aggregates patterns    │
└────────┬─────────────────┘
         │
    ┌────┴────┐
    ↓         ↓
┌────────┐  ┌────────┐
│  MQTT  │  │ Nomad  │
│ Broker │  │ Server │
└────┬───┘  └───┬────┘
     │          │
     │          │ Schedules on GPU node
     │          ↓
     │     ┌─────────────────────┐
     │     │ Nomad Client (GPU)  │
     │     │  GPU Compute Worker │
     │     │  (any language,     │
     │     │   any container)    │
     │     └──────────┬──────────┘
     │                │
     └────────────────┘
          MQTT Results
```

## Components

### 1. Go Orchestrator
- **Language**: Go 1.23
- **Function**: HTTP API server, MQTT client, Nomad dispatcher, LLM pattern aggregation
- **Container**: Docker (multi-stage build)
- **Location**: Project root (`go.mod`, `main.go`, `pkg/`, `cmd/`, `Makefile`, `Dockerfile`)

### 2. MQTT Broker
- **Service**: Eclipse Mosquitto
- **Function**: Async message passing for job parameters and results
- **Ports**: 1883 (MQTT), 9001 (WebSocket)

### 3. Nomad Cluster
- **Service**: HashiCorp Nomad
- **Function**: Container orchestration and GPU scheduling
- **Port**: 4646 (HTTP API)

### 4. Compute Workers
Workers are independently deployed GPU containers registered via Nomad. They receive
job parameters via MQTT and publish results back. Workers can be written in any language
and run in any container format (Docker, Singularity, raw binary).

The semantic-n-body worker (Julia) lives in its own repository:
`https://git.home.federation.fi/lavernea/semantic-n-body`

## Prerequisites

### Development Machine
- Docker & Docker Compose
- Go 1.23+
- Make
- Git

### GPU Node
- Ubuntu 24.04 LTS
- AMD ROCm 7.0+ drivers installed
- Nomad client configured
- Singularity/Apptainer installed (optional, for containerized workers)
- AMD Radeon AI Pro R9700 (RDNA 4)

## Quick Start

### 1. Build Go Orchestrator

```bash
make build
# Or build Docker image
make docker-build
```

### 2. Start Local Development Stack

```bash
# Start MQTT, Nomad, Consul, and Go orchestrator
docker-compose up -d

# Check services
docker-compose ps
```

### 3. Start Full Stack (with PostgreSQL + LLM)

```bash
# Includes pattern storage DB and Ollama LLM integration
docker-compose -f docker-compose.enhanced.yaml up -d
```

### 4. Configure GPU Node

```bash
# On GPU node, configure Nomad client
sudo cp nomad-jobs/nomad-client-gpu.hcl /etc/nomad.d/client.hcl
sudo systemctl restart nomad

# Verify GPU detection
nomad node status -self
```

### 5. Register Nomad Job

```bash
cd nomad-jobs
chmod +x setup-and-deploy.sh
./setup-and-deploy.sh
```

### 6. Submit Test Job

**Via API:**
```bash
curl -X POST http://localhost:8080/compute \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "semantic_diffusion",
    "parameters": {
      "max_iterations": 250
    }
  }'
```

**Manual Dispatch:**
```bash
cd nomad-jobs
chmod +x test-dispatch.sh
./test-dispatch.sh
```

## Testing

The repository includes unit tests and integration tests. Unit tests run quickly without external dependencies. Integration tests spin up an isolated Docker Compose stack automatically.

### Unit Tests

Run unit tests for the `pkg/` package tree:

```bash
make test-unit
```

This runs `go test -v -race ./pkg/...` with the race detector enabled. No Docker or external services needed.

### Integration Tests

Run integration tests against a full test stack:

```bash
make test-integration
```

This command:
1. Starts `docker-compose.test.yaml` with isolated services
2. Runs all tests marked with `//go:build integration`
3. Tears down the stack

Integration tests are invisible to plain `go test ./...` due to the build tag. The test stack runs on different ports than the dev stack, allowing both to run simultaneously:

| Service      | Dev Port | Test Port |
|--------------|----------|-----------|
| MQTT         | 1883     | 11883     |
| Nomad        | 4646     | 14646     |
| Orchestrator | 8080     | 18080     |

### All Tests

Run both unit and integration tests:

```bash
make test-all
```

This runs `test-unit` followed by `test-integration`.

### Coverage

Generate an HTML coverage report:

```bash
make test-coverage
```

## API Endpoints

### POST `/compute`
Submit async compute job.

**Request:**
```json
{
  "operation": "semantic_diffusion",
  "parameters": {
    "max_iterations": 250,
    "use_gpu": true
  },
  "priority": "normal"
}
```

**Response:**
```json
{
  "job_id": "abc-123-def",
  "status": "dispatched",
  "created_at": "2025-11-07T12:00:00Z",
  "nomad_eval_id": "eval-xyz"
}
```

### POST `/compute/sync`
Submit and wait for result.

### GET `/jobs`
List all jobs.

### GET `/jobs/{job_id}`
Get job details and result.

### POST `/jobs/cancel?job_id={id}`
Cancel running job.

### GET `/health`
Health check.

## MQTT Topics

- `gpu/jobs/{job_id}/params` - Job parameters (retained)
- `gpu/jobs/{job_id}/status` - Status updates
- `gpu/jobs/{job_id}/result` - Computation results
- `gpu/jobs/{job_id}/logs` - Log messages

## Configuration

### Go Orchestrator
Environment variables:
- `MQTT_BROKER` - MQTT broker address (default: `tcp://mqtt:1883`)
- `NOMAD_ADDR` - Nomad API address (default: `http://nomad:4646`)
- `NOMAD_JOB_NAME` - Job name (default: `gpu-compute`)
- `PORT` - HTTP port (default: `8080`)

### Enhanced Stack (docker-compose.enhanced.yaml)
Additional environment variables:
- `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME` - PostgreSQL for pattern storage
- `OLLAMA_URL` - Ollama LLM endpoint (default: `http://host.docker.internal:11434`)
- `OLLAMA_MODEL` - LLM model (default: `qwen2.5:7b`)
- `LLM_CACHE_ENABLED` - Enable LLM result caching
- `LLM_CACHE_TTL` - Cache TTL (default: `720h`)

## Monitoring

### Nomad UI
```bash
open http://localhost:4646
```

### MQTT Explorer
```bash
open http://localhost:4000
```

### Job Logs
```bash
# Via Nomad
nomad alloc logs -f <alloc-id>

# Via Go API
curl http://localhost:8080/jobs/{job_id}
```

### GPU Status (on GPU node)
```bash
rocm-smi
watch -n 1 rocm-smi
```

## Troubleshooting

### GPU Not Detected
```bash
# Check ROCm installation
rocm-smi

# Check device nodes
ls -la /dev/kfd /dev/dri/

# Check Nomad fingerprinting
nomad node status -self -verbose
```

### MQTT Connection Issues
```bash
# Test MQTT connectivity
mosquitto_sub -h mqtt -t 'gpu/jobs/#' -v

# Check MQTT logs
docker-compose logs mqtt
```

### Nomad Scheduling Issues
```bash
# Check job status
nomad job status gpu-compute

# Check allocation
nomad alloc status <alloc-id>

# Check node eligibility
nomad node eligibility -self
```

## Production Deployment

### Security Considerations
1. Enable MQTT authentication
2. Configure Nomad ACLs
3. Use TLS for all services
4. Restrict network access
5. Enable audit logging

### High Availability
1. Run multiple Nomad servers
2. Use HA MQTT broker (clustered)
3. Add multiple GPU nodes
4. Configure load balancing

### Monitoring
1. Prometheus metrics export
2. Grafana dashboards
3. Log aggregation (ELK/Loki)
4. Alert rules

## License

MIT
