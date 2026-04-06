# Forge — Concepts

> _A thin, stateless broker between clients that need GPU compute and workers that can deliver it._

**Date of initial implementation**: February - April 2026.

---

## Table of Contents

1. [Problem Statement](#1-problem-statement)
2. [The Role of Forge](#2-the-role-of-forge)
3. [Request Identity — Client ID and Correlation ID](#3-request-identity--client-id-and-correlation-id)
4. [The Job Lifecycle](#4-the-job-lifecycle)
5. [Workers Are External](#5-workers-are-external)
6. [Nomad as the Dispatch Layer](#6-nomad-as-the-dispatch-layer)
7. [MQTT as the Communication Fabric](#7-mqtt-as-the-communication-fabric)
8. [The Params Pattern — Retained Messages as Worker Bootstrap](#8-the-params-pattern--retained-messages-as-worker-bootstrap)
9. [Statelessness and Its Limits](#9-statelessness-and-its-limits)
10. [Design Decisions and Roads Not Taken](#10-design-decisions-and-roads-not-taken)

---

## 1. Problem Statement

### What This Solves

GPU compute is expensive, stateful, and slow to start. A client that needs a computation result — an inference run, a numerical simulation, a model training step — should not need to know which machine will execute it, how to schedule the work, or how to track the hardware. It should be able to say: "run this operation with these parameters" and later receive the result.

The problem is the gap between that simple request and the real machinery: a fleet of GPU nodes managed by Nomad, workers that run as parameterized batch jobs, and the need to correlate responses back to the original requester across an asynchronous, message-passing system.

Forge closes that gap.

### Why the Approach is Non-Obvious

The non-obvious move is to treat the entire pipeline — request, dispatch, execution, result delivery — as a message flow over MQTT rather than a synchronous HTTP call chain. This means no persistent connections between client and orchestrator, no polling, and no shared state between components except what is encoded in messages and the job store.

A second non-obvious move: Forge itself does not execute compute. It does not even know what a given operation does. It only knows how to dispatch a Nomad job with a given operation name and a job ID, and how to route the result back to the client that requested it. The semantics of the computation live entirely in the worker.

---

## 2. The Role of Forge

Forge is an orchestrator in the narrow sense: it manages the lifecycle of compute jobs without participating in the computation. Its responsibilities are:

1. **Receive requests** from clients via MQTT (`compute/request/{client_id}/{correlation_id}`)
2. **Publish job parameters** to a retained MQTT topic so workers can bootstrap themselves
3. **Dispatch a Nomad job** for the requested operation, passing the job ID as metadata
4. **Track the in-flight job** in memory
5. **Receive the result** from the worker via MQTT (`compute/jobs/{job_id}/result`)
6. **Route the response** back to the originating client via MQTT (`compute/response/{client_id}/{correlation_id}`)

Forge does not persist jobs. If Forge restarts, in-flight jobs are lost (from Forge's perspective — the Nomad allocation may still complete, but the result will be received by a handler with no corresponding in-memory job). This is an explicit design choice, described further in §9.

---

## 3. Request Identity — Client ID and Correlation ID

Every request carries two identifiers extracted from the MQTT topic, not from the payload:

- **Client ID**: identifies the requesting client. Used to route the response back.
- **Correlation ID**: identifies this specific request within the client's session. Allows a client to have multiple in-flight requests simultaneously and match each response to the correct one.

Both are extracted from the topic: `compute/request/{client_id}/{correlation_id}`. They are not trusted user-supplied fields in the payload — they are structural properties of the message routing. This is load-bearing: it means the response topic `compute/response/{client_id}/{correlation_id}` is deterministic from the request topic alone.

A client that subscribes to `compute/response/{client_id}/+` before publishing a request will always receive its responses, even if Forge restarts between request and response delivery (assuming the Nomad job completes and a new Forge instance is running).

---

## 4. The Job Lifecycle

```
Client publishes → compute/request/{client_id}/{correlation_id}
                        │
                        ▼
              Forge receives, generates job_id (UUID)
                        │
              ┌─────────┴──────────┐
              │                    │
              ▼                    ▼
   Publish retained params   Dispatch Nomad job
   compute/jobs/{job_id}/params   (operation=req.Operation, meta.job_id=job_id)
              │
              ▼
   Nomad schedules allocation on GPU node
              │
              ▼
   Worker starts, reads retained params from MQTT
              │
              ▼
   Worker publishes progress (optional):
   compute/jobs/{job_id}/status
   compute/jobs/{job_id}/logs
              │
              ▼
   Worker publishes result:
   compute/jobs/{job_id}/result
              │
              ▼
   Forge receives result, matches to in-memory job
              │
              ▼
   Forge publishes response:
   compute/response/{client_id}/{correlation_id}
```

The job ID is a UUID generated by Forge at request time. It is the key that connects the Nomad dispatch, the MQTT params topic, and the result message into a single coherent job.

---

## 5. Workers Are External

Compute workers are not part of this repository. They live in their own repositories and are deployed independently as Nomad parameterized batch jobs. Forge only knows about workers through:

1. The **operation name** — the string identifier of the Nomad job to dispatch (e.g., `gpu-compute`). This is configured in the job spec as `NOMAD_JOB_NAME`.
2. The **params topic** — `compute/jobs/{job_id}/params`, where Forge publishes a retained JSON message containing `job_id` and all payload fields from the client request.
3. The **result topic** — `compute/jobs/{job_id}/result`, where Forge expects to receive the result.

Workers must comply with the protocol:
- Subscribe to `compute/jobs/{job_id}/params` (or receive `job_id` from Nomad metadata) to get their parameters
- Publish result to `compute/jobs/{job_id}/result` with `{"job_id": "...", "success": bool, ...}`
- Optionally publish to `status` and `logs` topics during execution

The `success` field (boolean) is required. The older `status: string` format is not supported.

---

## 6. Nomad as the Dispatch Layer

Forge uses Nomad's parameterized batch job mechanism. When a request arrives, Forge calls `jobs.Dispatch(operationName, meta, ...)` where `meta` contains the `job_id`. Nomad handles:

- Scheduling the allocation on an appropriate node (constrained to GPU nodes)
- Downloading the worker binary from the artifact server
- Starting the process
- Restarting on failure (per the job's restart policy)
- Cleaning up after completion

Forge does not manage the worker process directly. It receives the `EvalID` and `DispatchedJobID` from Nomad's dispatch response and stores them on the job for observability, but does not poll Nomad for job status. The result signal comes from the worker via MQTT.

The Nomad job name used for dispatch is passed to Forge at runtime via the `NOMAD_JOB_NAME` environment variable, not hardcoded. This allows the same Forge binary to dispatch to different worker jobs without recompilation.

---

## 7. MQTT as the Communication Fabric

All coordination between clients, Forge, and workers happens over MQTT. The choice reflects a deliberate preference for loose coupling: no component holds a direct reference to any other, and components can be restarted, replaced, or scaled independently without coordination.

The communication model has three distinct flows:

**Client → Forge**: `compute/request/{client_id}/{correlation_id}` — synchronous from the client's perspective (it expects a response), but implemented as two independent pub/sub operations.

**Forge → Worker**: `compute/jobs/{job_id}/params` — a retained message. Workers can arrive late (after Nomad scheduling delay) and still receive their parameters.

**Worker → Forge**: `compute/jobs/{job_id}/result|status|logs` — the worker pushes its output back. Forge is subscribed to wildcard patterns at startup and routes messages by extracting the job ID from the topic.

**Forge → Client**: `compute/response/{client_id}/{correlation_id}` — the response is sent by Forge after receiving and processing the worker's result.

QoS 1 is used throughout (at-least-once delivery). This means duplicate messages are possible in theory; Forge's in-memory job store provides idempotent result handling (the result channel is buffered at capacity 1 with a non-blocking send).

---

## 8. The Params Pattern — Retained Messages as Worker Bootstrap

When Forge publishes job parameters, it uses MQTT retained messages. This is a deliberate design choice with a specific rationale: there is a window between Nomad dispatch and worker startup during which the worker does not yet exist to receive the message. Without retention, the message would be lost.

The retained params message is published to `compute/jobs/{job_id}/params` before Nomad dispatch is called. When the worker eventually starts and subscribes to this topic (or when Nomad passes the job ID as metadata and the worker constructs the topic), the broker delivers the most recently retained message immediately.

The params payload is a flat JSON merge of the client's request payload with the Forge-generated `job_id` added:

```json
{"job_id": "uuid", ...payload_fields_from_request}
```

The worker receives all the information it needs to execute the job in a single message. There is no second round-trip to Forge or any other service.

---

## 9. Statelessness and Its Limits

Forge holds in-flight jobs in a `sync.Map` in memory. There is no database, no persistent queue, no durable state. This is a deliberate constraint: Forge is designed to be simple and restartable without migration concerns.

The consequence: if Forge restarts while a job is in flight, the job is orphaned from Forge's perspective. The Nomad allocation may complete and the worker may publish its result to MQTT, but Forge will have no in-memory job entry to match it against. The result will be logged as an error (`unknown job ID`) and discarded. The client will time out.

This is an acceptable tradeoff at the current scale and use pattern. Workers are expected to complete within the job timeout (default 5 minutes), and Forge restarts are rare. If this becomes a problem, the fix is to persist the job store to an external database — but that adds operational complexity that is not yet warranted.

The timeout mechanism (default 5 minutes) is implemented as a goroutine per job that selects on either the result channel or `time.After`. When the timeout fires, a timeout error response is sent to the client. The in-memory job entry remains in the map (it is never explicitly deleted) — this is a known memory growth issue for very high job volumes, but negligible at current scale.

---

## 10. Design Decisions and Roads Not Taken

### No HTTP API

The current implementation exposes no HTTP API. All client interaction is via MQTT. An HTTP API was present in earlier versions but was removed. The reasoning: adding an HTTP layer would require a second protocol to secure, document, and maintain, while MQTT already provides the necessary semantics (async request/response via pub/sub, client identity via topic structure).

### Operation Name as Nomad Job Name

The `operation` field in a client request is used directly as the Nomad job name for dispatch. This means operations are Nomad jobs, one-to-one. Adding a new operation type means deploying a new parameterized batch job. This is intentionally explicit — there is no operation registry or routing table in Forge. The available operations are exactly the parameterized batch jobs registered in Nomad.

### No Worker Discovery

Forge does not know which workers are available or whether a given operation is registered in Nomad. If a client requests an operation that does not correspond to a Nomad job, the dispatch call fails and Forge sends an error response. This is fail-fast behavior: the error surfaces immediately rather than silently queuing.

### Timeout as the Failure Mode

Forge has no mechanism for detecting worker crashes beyond the timeout. If a worker starts, crashes immediately, and publishes no result, Forge waits the full timeout duration before responding with an error. Nomad's restart policy may attempt to restart the worker within that window, which is the intended behavior. The timeout is the safety net, not the primary error detection path.
