# Forge

*Flexible Orchestration Runtime for GPU Execution*

Forge is a daemon that bridges MQTT clients and Nomad-scheduled GPU compute workers. Clients publish job requests over MQTT, Forge dispatches the appropriate Nomad parameterized batch job, and routes results back to the client — all asynchronously over MQTT.

→ [CONCEPTS.md](CONCEPTS.md) — what it does and why  
→ [ARCHITECTURE.md](ARCHITECTURE.md) — how it works, data structures, deployment details

---

## Client Protocol

Subscribe to your response topic first, then publish the request:

```
Subscribe:  compute/response/{client_id}/{correlation_id}
Publish:    compute/request/{client_id}/{correlation_id}
```

**Request payload:**
```json
{
  "operation": "gpu-compute",
  "payload": { ... }
}
```

- `operation` — Nomad job name to dispatch (maps 1:1 to the compute worker)
- `payload` — arbitrary fields merged with `job_id` and forwarded to the worker
- `client_id` / `correlation_id` — part of the topic; multiple jobs in flight per client are supported

**Success response:**
```json
{
  "correlation_id": "...",
  "job_id": "...",
  "success": true,
  "result": { ... },
  "duration_ms": 1234,
  "worker_id": "..."
}
```

**Failure response** (dispatch error, worker error, or timeout):
```json
{
  "correlation_id": "...",
  "job_id": "...",
  "success": false,
  "error": "reason"
}
```

See [ARCHITECTURE.md — MQTT Topic Map](ARCHITECTURE.md#4-mqtt-topic-map) for the full topic listing.

---

## Worker Protocol

Forge publishes a **retained** message to `compute/jobs/{job_id}/params` before dispatching the Nomad job:

```json
{"job_id": "uuid", ...payload_fields_from_request}
```

Workers must publish a result to `compute/jobs/{job_id}/result` with `success: bool`. Status and log topics are optional. See [ARCHITECTURE.md — Key Data Structures](ARCHITECTURE.md#5-key-data-structures) for the full wire format.

---

## Build

```bash
make build      # → bin/orchestrator
make fmt        # go fmt + go vet
make lint       # golangci-lint
```

Go 1.23 required.

---

## Testing

```bash
make test-unit          # unit tests, race detector, coverage
make test-integration   # spins up docker-compose.test.yaml stack, runs integration suite
make test-all           # both
```

Integration tests use `//go:build integration` and require Docker Compose.  
Test stack ports are offset from dev: MQTT 11883, Nomad 14646, Orchestrator 18080.

---

## Deployment

Forge runs as a Nomad `service` job on the GPU node. Deployed automatically on push to `main` via Gitea CI.

```bash
# Manual deploy
NOMAD_ADDR=<addr> ARTIFACT_BASE=<url> make deploy
```

**Vault secrets** at `secret/data/nomad/forge`: `MQTT_BROKER`, `MQTT_USER`, `MQTT_PASSWORD`, `LOG_LEVEL`  
**Gitea repo variables:** `NOMAD_ADDR`, `ARTIFACT_BASE`

See [ARCHITECTURE.md — Deployment](ARCHITECTURE.md#9-deployment) for Nomad job details and CI pipeline.

---

## Troubleshooting

```bash
# Forge logs
nomad alloc logs -f <alloc-id>

# Monitor all job traffic
mosquitto_sub -h <broker> -u <user> -P <pass> -t 'compute/#' -v

# Nomad dispatch failures
nomad job status <operation>
nomad alloc status <alloc-id>
```
