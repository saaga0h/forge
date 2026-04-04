# Forge

*GPU Compute Orchestrator*

Forge is a daemon that bridges MQTT clients and Nomad-scheduled compute workers. Clients publish job requests over MQTT, Forge dispatches the appropriate Nomad parameterized batch job, and relays results back to the client — all asynchronously over MQTT.

## Architecture

```
Client
  │  publish compute/request/{client_id}/{correlation_id}
  │  subscribe compute/response/{client_id}/{correlation_id}
  ↓
Forge (raw_exec daemon on GPU node)
  │  publish retained compute/jobs/{job_id}/params
  │  dispatch Nomad parameterized batch job (operation = job name)
  ↓
Nomad → Compute Worker (one-shot batch)
  │  subscribe compute/jobs/{job_id}/params
  │  publish compute/jobs/{job_id}/status
  │  publish compute/jobs/{job_id}/logs
  │  publish compute/jobs/{job_id}/result
  ↓
Forge
  │  receives result, matches by job_id
  ↓
Client
  receives compute/response/{client_id}/{correlation_id}
```

## Client Protocol

### Submitting a job

Subscribe to your response topic first, then publish the request:

```
Subscribe:  compute/response/{client_id}/{correlation_id}
Publish:    compute/request/{client_id}/{correlation_id}
```

**Request payload:**
```json
{
  "operation": "semantic-n-body",
  "payload": {
    "anchors": [...]
  }
}
```

- `operation` — Nomad job name to dispatch (maps 1:1 to the compute worker)
- `payload` — arbitrary fields, merged with `job_id` and forwarded to the worker as-is
- `client_id` — self-declared client identity, part of the topic
- `correlation_id` — client-generated ID to match request with response

Multiple jobs can be in flight simultaneously from the same client — each `correlation_id` is independent.

### Response

**Success:**
```json
{
  "correlation_id": "my-req-1",
  "job_id": "uuid-generated-by-forge",
  "success": true,
  "result": { ... },
  "duration_ms": 1234,
  "worker_id": "nomad-abc123"
}
```

**Failure** (Nomad dispatch error, worker error, or timeout):
```json
{
  "correlation_id": "my-req-1",
  "job_id": "uuid-generated-by-forge",
  "success": false,
  "error": "reason"
}
```

## MQTT Topics

| Topic | Direction | Description |
|-------|-----------|-------------|
| `compute/request/{client_id}/{correlation_id}` | client → forge | Job request |
| `compute/response/{client_id}/{correlation_id}` | forge → client | Job result or error |
| `compute/jobs/{job_id}/params` | forge → worker | Job parameters (retained) |
| `compute/jobs/{job_id}/status` | worker → forge | Status updates |
| `compute/jobs/{job_id}/result` | worker → forge | Computation result |
| `compute/jobs/{job_id}/logs` | worker → forge | Log output |

## Worker Protocol

Forge publishes a **retained** message to `compute/jobs/{job_id}/params` before dispatching the Nomad job. The worker reads `NOMAD_META_job_id` from its environment, subscribes to the matching params topic, processes the job, and publishes the result.

**Params** (QoS 1, retained):
```json
{"job_id": "uuid", ...payload_fields_from_request}
```

**Result** (QoS 1):
```json
{
  "job_id": "uuid",
  "success": true,
  "result": { ... },
  "worker_id": "nomad-abc123",
  "timestamp": "2026-04-04T10:00:00Z"
}
```

`error` (string), `duration_ms` (int), and `metrics` are optional. Workers must send `success: bool`.

## Deployment

Forge runs as a Nomad `service` job on the GPU node. Deployed automatically on push to `main` via Gitea CI.

```bash
# Manual deploy
NOMAD_ADDR=<addr> ARTIFACT_BASE=<url> make deploy
```

**Vault secrets** at `secret/data/nomad/forge`:

| Key | Description |
|-----|-------------|
| `MQTT_BROKER` | Broker URL, e.g. `tcp://mqtt.example.com:1883` |
| `MQTT_USER` | Broker username |
| `MQTT_PASSWORD` | Broker password |
| `LOG_LEVEL` | `info` or `debug` |

**Gitea repo variables:** `NOMAD_ADDR`, `ARTIFACT_BASE`

## Build

```bash
make build      # → bin/orchestrator
make fmt        # go fmt + go vet
make lint       # golangci-lint
```

Go 1.23 required.

## Testing

```bash
make test-unit          # unit tests, race detector, coverage
make test-integration   # spins up docker-compose.test.yaml stack, runs integration suite
make test-all           # both
```

Integration tests use `//go:build integration` and require Docker Compose.
Test stack ports are offset from dev: MQTT 11883, Nomad 14646, Orchestrator 18080.

## Troubleshooting

### Check Forge logs
```bash
nomad alloc logs -f <alloc-id>
```

### Monitor all job traffic
```bash
mosquitto_sub -h <broker> -u <user> -P <pass> -t 'compute/#' -v
```

### Nomad dispatch fails
```bash
nomad job status semantic-n-body
nomad alloc status <alloc-id>
```
