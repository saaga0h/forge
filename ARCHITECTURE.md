# Forge — Architecture

> _MQTT-native, stateless, Go. Dispatches GPU compute jobs via Nomad._

**As of**: April 2026

---

## Table of Contents

1. [System Overview](#1-system-overview)
2. [Component Inventory](#2-component-inventory)
3. [Data Flow](#3-data-flow)
4. [MQTT Topic Map](#4-mqtt-topic-map)
5. [Key Data Structures](#5-key-data-structures)
6. [Package Structure](#6-package-structure)
7. [Request Handling](#7-request-handling)
8. [Result Routing](#8-result-routing)
9. [Deployment](#9-deployment)
10. [Invariants](#10-invariants)
11. [Known Constraints and Tradeoffs](#11-known-constraints-and-tradeoffs)

---

## 1. System Overview

Forge is a single long-running service that bridges MQTT-based client requests to GPU compute workers managed by Nomad. It is stateless except for an in-memory job map and has no external database dependency.

```
Client
  │  publish: compute/request/{client_id}/{correlation_id}
  ▼
MQTT Broker ──────────────────────────────────────────────────────────┐
  │  subscribe: compute/request/+/+                                   │
  ▼                                                                   │
Forge (orchestrator)                                                  │
  ├── publish retained: compute/jobs/{job_id}/params                  │
  ├── dispatch: Nomad API → parameterized batch job                   │
  │                                                                   │
  │   Nomad schedules worker on GPU node                              │
  │                                                                   │
  │   Worker starts, reads params from MQTT                           │
  │   Worker publishes: compute/jobs/{job_id}/status|logs|result      │
  │                                                ▲                  │
  └── subscribe: compute/jobs/+/result|status|logs ┘                  │
  │                                                                   │
  └── publish: compute/response/{client_id}/{correlation_id} ─────────┘
                                                                      │
Client                                                                │
  │  subscribe: compute/response/{client_id}/+   ◄────────────────────┘
```

---

## 2. Component Inventory

### Long-Running Service

| Binary | Role | Trigger |
|--------|------|---------|
| `orchestrator` | Receives client requests, dispatches Nomad jobs, routes results back to clients | MQTT subscriptions at startup |

### Subscriptions at Startup

| Topic Pattern | Handler |
|---------------|---------|
| `compute/request/+/+` | `handleRequest` — parses client ID + correlation ID from topic, dispatches job |
| `compute/jobs/+/result` | `handleResult` — routes result to waiting goroutine, publishes response |
| `compute/jobs/+/status` | `handleStatus` — updates in-memory job status and progress |
| `compute/jobs/+/logs` | `handleLog` — logs worker output to stdout |

---

## 3. Data Flow

### Request to Dispatch

```
Client
  │ publish JSON: {"operation": "gpu-compute", "payload": {...}}
  │ topic: compute/request/{client_id}/{correlation_id}
  ▼
Manager.handleRequest
  │ extract client_id, correlation_id from topic
  │ unmarshal Request{Operation, Payload}
  │ generate job_id (UUID)
  ▼
Manager.submit
  │ build params: payload + job_id (flat merge)
  │ publish retained params → compute/jobs/{job_id}/params
  │ call Nomad.Dispatch(operation, job_id, meta)
  │ store Job{} in sync.Map keyed by job_id
  │ return Job with resultChan (buffered 1)
  ▼
goroutine: select { resultChan | time.After(timeout) }
  │ on result: publish response → compute/response/{client_id}/{correlation_id}
  │ on timeout: publish error response
```

### Worker to Result

```
Worker (external, Nomad batch job)
  │ publish JSON: {"job_id": "...", "success": true, "result": {...}, ...}
  │ topic: compute/jobs/{job_id}/result
  ▼
Manager.handleResult
  │ unmarshal JobResult
  │ lookup Job in sync.Map by job_id
  │ update Job fields: Status, Result, Error, CompletedAt, DurationMS, Metrics
  │ non-blocking send to job.resultChan
  ▼
waiting goroutine (from handleRequest) unblocks
  │ build response payload with correlation_id + job_id + result
  │ publish → compute/response/{client_id}/{correlation_id}
```

---

## 4. MQTT Topic Map

| Topic | Direction | Producer | Consumer | Notes |
|-------|-----------|----------|----------|-------|
| `compute/request/{client_id}/{correlation_id}` | inbound | client | Forge | `+/+` wildcard; client_id and correlation_id extracted from topic |
| `compute/jobs/{job_id}/params` | outbound | Forge | worker | **Retained**; published before Nomad dispatch so late-starting workers receive it |
| `compute/jobs/{job_id}/status` | inbound | worker | Forge | Optional; updates in-memory job progress |
| `compute/jobs/{job_id}/logs` | inbound | worker | Forge | Optional; logged to stdout |
| `compute/jobs/{job_id}/result` | inbound | worker | Forge | Required; triggers response to client |
| `compute/response/{client_id}/{correlation_id}` | outbound | Forge | client | Final response; success or error |

All messages are JSON. QoS 1 throughout (at-least-once delivery).

---

## 5. Key Data Structures

### Request (inbound from client, via MQTT payload)

```go
type Request struct {
    Operation     string                 `json:"operation"`
    Payload       map[string]interface{} `json:"payload,omitempty"`
    ClientID      string                 `json:"-"` // extracted from topic
    CorrelationID string                 `json:"-"` // extracted from topic
}
```

`ClientID` and `CorrelationID` are not trusted from the payload — they come from the topic structure.

### Job (in-memory, Forge-internal)

```go
type Job struct {
    ID            string         // UUID generated by Forge
    Operation     string
    ClientID      string
    CorrelationID string
    Status        string         // pending → dispatched → starting → computing → completed|failed|timeout
    Progress      float64        // 0.0–1.0, from worker status messages
    Result        interface{}
    Error         string
    CreatedAt     time.Time
    CompletedAt   *time.Time
    DurationMS    int64
    NomadEvalID   string
    NomadJobID    string
    Metrics       *mqtt.Metrics
    resultChan    chan *mqtt.JobResult  // buffered 1; not serialized
}
```

Jobs are stored in a `sync.Map` and never deleted. Terminal state is set by `handleResult` but the entry remains in memory for the lifetime of the process.

### JobResult (inbound from worker, via MQTT)

```go
type JobResult struct {
    JobID      string      `json:"job_id"`
    Success    bool        `json:"success"`        // required; old status:string format not supported
    Result     interface{} `json:"result,omitempty"`
    Error      string      `json:"error,omitempty"`
    WorkerID   string      `json:"worker_id,omitempty"`
    DurationMS int64       `json:"duration_ms,omitempty"`
    Timestamp  time.Time   `json:"timestamp"`
    Metrics    *Metrics    `json:"metrics,omitempty"`
}
```

### Params (outbound to worker, retained MQTT message)

```json
{
  "job_id": "<uuid>",
  "<key>": "<value>",
  ...
}
```

Flat merge of `Request.Payload` with `job_id` injected. Workers receive all inputs in one message.

### Response (outbound to client, via MQTT)

Success:
```json
{
  "correlation_id": "...",
  "job_id": "...",
  "success": true,
  "result": {...},
  "duration_ms": 1234,
  "worker_id": "..."
}
```

Failure:
```json
{
  "correlation_id": "...",
  "job_id": "...",
  "success": false,
  "error": "..."
}
```

---

## 6. Package Structure

```
forge/
├── main.go                      # Entry point: wires MQTT client, Nomad dispatcher, Manager
├── pkg/
│   ├── mqtt/
│   │   ├── client.go            # MQTT client wrapper (connect, publish, subscribe)
│   │   ├── topics.go            # Topic constants and constructor functions
│   │   └── types.go             # JobResult, JobStatus, JobLog, Metrics structs
│   ├── compute/
│   │   ├── manager.go           # Core orchestration logic: handleRequest, submit, handleResult
│   │   └── types.go             # Request, Job structs
│   └── nomad/
│       ├── dispatcher.go        # Nomad API wrapper: Dispatch, GetJobStatus, ListNodes, etc.
│       └── types.go             # JobStatus, AllocationStatus, EvalStatus, NodeInfo
├── deploy/
│   ├── nomad/
│   │   └── forge-daemons.hcl    # Nomad service job definition (raw_exec, Vault secrets)
│   └── vault/
│       └── forge-policy.hcl     # Vault policy for secret access
├── .gitea/
│   └── workflows/
│       └── build.yml            # CI: build amd64+arm64, publish to artifact server, deploy
├── configs/
│   ├── config.yaml              # Orchestrator app config
│   └── mosquitto.conf           # MQTT broker config (dev stack)
└── integration_test/            # Integration tests (build tag: integration)
```

---

## 7. Request Handling

`Manager.handleRequest` in [pkg/compute/manager.go](pkg/compute/manager.go):

1. Splits topic on `/` to extract `client_id` (index 2) and `correlation_id` (index 3). Rejects topics that do not produce exactly 4 parts.
2. Unmarshals the payload as `Request`. Rejects missing `operation`.
3. Calls `submit()` which generates a UUID job ID, publishes retained params, calls Nomad dispatch, and stores the job.
4. Launches a goroutine to wait for the result or timeout.

The goroutine is the unit of per-job state. It holds a reference to the `Job` struct and its `resultChan`. It publishes the response (success or timeout error) and then exits. No cleanup of the job map entry occurs.

Error responses are published via `sendError()`, which publishes to the response topic and returns an `error`. The error is logged by the MQTT handler wrapper in `client.go` but does not propagate further.

---

## 8. Result Routing

`Manager.handleResult` in [pkg/compute/manager.go](pkg/compute/manager.go):

1. Unmarshals payload as `JobResult`. Requires `job_id` and `success` fields.
2. Looks up the job in `sync.Map` by `job_id`. Logs and returns error if not found (orphaned result — job not in memory, typically because Forge restarted).
3. Updates job fields in-place (no lock — `Job` struct fields are written by this handler and the result goroutine, but they write different fields and the channel provides the synchronization point).
4. Non-blocking send to `job.resultChan`:
   ```go
   select {
   case job.resultChan <- &result:
   default:
   }
   ```
   The `default` branch silently drops duplicate results. This can occur if a worker publishes the result more than once (e.g., due to QoS 1 redelivery).

---

## 9. Deployment

### CI Pipeline (`.gitea/workflows/build.yml`)

1. Build `amd64` and `arm64` binaries with `CGO_ENABLED=0 GOOS=linux`
2. Copy binaries to artifact server at `/var/lib/packer-builds/binaries/forge/{arch}/orchestrator` (runner is co-located with nginx)
3. Compute `BUILD_SHA` from `$GITEA_SHA` and inject into `meta.build` via `envsubst`
4. Submit job: `envsubst '$ARTIFACT_BASE $NOMAD_ADDR $BUILD_SHA' < forge-daemons.hcl | nomad job run -`

`meta.build` changes on every commit, making the job spec differ from the previous version and forcing Nomad to replace the allocation and re-fetch the binary.

**Important**: `envsubst` must be called with an explicit variable list (`$ARTIFACT_BASE $NOMAD_ADDR $BUILD_SHA` — `$VAR` form, not `${VAR}`). Without the list, `envsubst` substitutes all environment variables and will clobber Nomad runtime variables like `${NOMAD_TASK_DIR}` that happen to be set in the CI runner environment (the runner itself runs as a Nomad allocation).

### Nomad Job (`deploy/nomad/forge-daemons.hcl`)

- **Type**: `service` (long-running, restarted on failure)
- **Driver**: `raw_exec` (binary on host filesystem, not in a container)
- **Constraints**: `meta.gpu = true` and `meta.rocm = true` — runs only on GPU nodes
- **Artifact**: downloaded from artifact server using `${ARTIFACT_BASE}/${attr.cpu.arch}/orchestrator`; `${attr.cpu.arch}` is resolved by Nomad at placement time, enabling the same job spec to run on both amd64 and arm64 nodes
- **Secrets**: Vault template renders `secrets/forge.env` with `MQTT_BROKER`, `MQTT_USER`, `MQTT_PASSWORD`, `LOG_LEVEL`
- **Restart policy**: 5 attempts, 15s delay, within 5m window

### Environment Variables

| Variable | Source | Description |
|----------|--------|-------------|
| `MQTT_BROKER` | Vault `secret/data/nomad/forge` | MQTT broker URL (e.g. `tcp://mqtt:1883`) |
| `MQTT_USER` | Vault | MQTT username |
| `MQTT_PASSWORD` | Vault | MQTT password |
| `LOG_LEVEL` | Vault | Log verbosity |
| `NOMAD_ADDR` | `NOMAD_META_nomad_addr` (job meta) | Nomad API address for dispatch calls |
| `NOMAD_JOB_NAME` | `NOMAD_META_worker_job` (job meta) | Nomad job name to dispatch for compute requests |

### Local Dev Stack

```bash
docker-compose up -d                                # MQTT + Nomad + orchestrator
docker-compose -f docker-compose.enhanced.yaml up -d  # adds Consul
```

---

## 10. Invariants

1. **Client ID and Correlation ID come from the topic, not the payload.** A client cannot forge another client's response routing by manipulating payload fields.

2. **Params are published before Nomad dispatch.** The retained message must exist on the broker before the worker can start. Publishing after dispatch creates a race where the worker starts before its params are available.

3. **`success` is a boolean, not a string.** Workers must publish `"success": true|false`. The older `"status": "completed"` format is not handled.

4. **`job_id` in the params payload matches the Nomad metadata `job_id`.** Workers receive the same ID via both channels (retained MQTT message and Nomad dispatch metadata). They must use this ID when publishing to status, logs, and result topics.

5. **QoS 1 for all subscriptions and publishes.** Downgrading to QoS 0 would allow result messages to be lost in transit, leaving the waiting goroutine to time out.

6. **`envsubst` variable list uses `$VAR` form, not `${VAR}`.** The `${VAR}` form is not valid in `envsubst`'s variable list argument. The Makefile recipe uses `$$VAR` (Make-escaped `$VAR`).

---

## 11. Known Constraints and Tradeoffs

### Job Map Never Pruned

Completed jobs remain in the `sync.Map` for the lifetime of the process. At low job volumes this is negligible. At high volumes over long runtimes, this will grow without bound. The fix (delete after terminal state + small grace period) has not been implemented.

### No Persistence Across Restarts

In-flight jobs are lost if Forge restarts. There is no mechanism to recover a job that was dispatched but whose result arrives after a restart. This is acceptable for the current use pattern but would require a persistent job store (e.g., Redis or Postgres) to fix.

### No Operation Validation

Forge does not validate whether a requested operation exists as a Nomad job before dispatching. An unknown operation name results in a Nomad API error, which Forge surfaces as a job submission failure to the client. Nomad is the source of truth for available operations.

### MQTT Client ID is Random per Start

The MQTT client ID is generated as `go-orch-<uuid>` on each startup (when not explicitly configured). This means Forge does not resume a previous MQTT session after restart. With `CleanSession: false` and a random client ID, this is functionally equivalent to `CleanSession: true` — no queued messages are delivered on reconnect. If message queuing across restarts is needed, a stable client ID must be configured.

### Single Nomad Job Name for All Operations

`NOMAD_JOB_NAME` is a single environment variable. All operations dispatched by this Forge instance use the same Nomad job name. This means operation routing is by Nomad job name only — there is no per-operation job name mapping. If multiple worker job types are needed, either multiple Forge instances are required or a routing table must be added.
