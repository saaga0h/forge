# Plan: Generic Orchestration — Remove Domain-Specific Residue
## Created: 2026-02-27
## Complexity: sonnet
## Recommended implementation model: sonnet

## Context

The orchestrator has residual domain-specific code from when computation lived in this repo:
- `manager.go` has a hard-coded branch for `diffusion`/`semantic_diffusion` that publishes raw
  CSV anchor data to MQTT instead of JSON — this belongs in the compute worker, not here.
- `JobParams` struct wraps params in `{job_id, operation, parameters, timestamp}` — but the
  worker protocol expects `{job_id, ...payload_fields}` flat (job_id merged with the payload).
- `JobResult` uses `Status string` ("completed"/"failed") — but workers send `success: bool`
  + `worker_id`. The orchestrator should derive its internal status from `success`.
- `Request.Parameters` / `Job.Parameters` uses the old field name — rename to `Payload` to
  reflect that it's a generic pass-through to the compute worker.
- `static/index.html` hardcodes a diffusion CSV upload UI — replace with a generic debug
  console for any operation.

**Worker protocol (fixed contract):**

Params topic (orchestrator → worker, retained):
```json
{"job_id": "uuid", ...payload_fields_from_request}
```

Result topic (worker → orchestrator):
```json
{
  "job_id": "uuid",
  "success": true,
  "result": { ...arbitrary... },
  "worker_id": "nomad-abc123",
  "timestamp": "2026-02-21T10:00:00.000Z"
}
```
`error` string is optional. `duration_ms` and `metrics` are optional orchestrator extensions
a worker may include.

**What does NOT change:** MQTT topic names, `TopicBuilder`, `JobStatus`, `JobLog`,
`Metrics`, `Dispatcher`, `main.go` HTTP handlers, `Config` types, docker-compose files.

## Prerequisites
- [x] Plans 01–05 complete (codebase is pure orchestrator with no patterns/LLM code)
- [x] `go build ./...` passes
- [x] `go test -v -race ./pkg/...` passes (10 tests)

## Tasks

### [x] 1. Update pkg/mqtt/types.go
- **File**: `pkg/mqtt/types.go`
- **Action**:
  - Delete the entire `JobParams` struct. Manager will build the MQTT params message as a
    `map[string]interface{}` directly — no named struct needed.
  - Update `JobResult` struct:
    - Remove `Status string` field
    - Add `Success bool   \`json:"success"\``
    - Add `WorkerID string \`json:"worker_id,omitempty"\``
    - Keep `JobID`, `Result`, `Error`, `DurationMS`, `Timestamp`, `Metrics` unchanged
  - All other types (`JobStatus`, `JobLog`, `Metrics`, `TopicBuilder`, topic constants)
    stay exactly as-is.
- **Test**: `go build ./pkg/mqtt/` must pass
- **Notes**: `client.go` handles `map[string]interface{}` via the `default: json.Marshal`
  branch in `Publish()` — no changes needed there.

### [x] 2. Update pkg/compute/types.go
- **File**: `pkg/compute/types.go`
- **Action**:
  - In `Request`: rename field `Parameters map[string]interface{}` → `Payload map[string]interface{}`
    with JSON tag `json:"payload,omitempty"`
  - In `Job`: rename field `Parameters map[string]interface{}` → `Payload map[string]interface{}`
    with JSON tag `json:"payload,omitempty"`
  - Everything else in the file stays the same (no logic change, just field rename).
- **Test**: `go build ./pkg/compute/` must pass
- **Notes**: `resultChan chan *mqtt.JobResult` still compiles because `mqtt.JobResult` still
  exists (just with updated fields).

### [x] 3. Rewrite pkg/compute/manager.go
- **File**: `pkg/compute/manager.go`
- **Action**: Three surgical changes:

  **A. Replace params publishing (in Submit()):**
  Remove the entire `if req.Operation == "diffusion" || req.Operation == "semantic_diffusion"`
  block AND the `else` block that follows it. Replace with a single unified publish:
  ```go
  // Build params: job_id merged with payload fields
  params := make(map[string]interface{})
  for k, v := range req.Payload {
      params[k] = v
  }
  params["job_id"] = jobID

  if err := m.mqttClient.Publish(topics.Params(), params, true); err != nil {
      m.jobs.Delete(jobID)
      return nil, fmt.Errorf("mqtt publish params failed: %w", err)
  }
  log.Printf("Published params for job %s (%d payload fields)", jobID, len(req.Payload))
  ```

  **B. Simplify Nomad metadata (in Submit()):**
  Replace the current metadata building block (which extracts numeric `operation` and `size`
  from parameters) with:
  ```go
  meta := make(map[string]string)
  if req.Operation != "" {
      meta["operation"] = req.Operation
  }
  if req.Priority != "" {
      meta["priority"] = req.Priority
  }
  ```
  Keep the existing `dispatchResult, err := m.nomad.Dispatch(jobID, meta)` call unchanged
  (Dispatch() already adds `job_id` to meta internally).

  **C. Update handleResult() to use Success bool:**
  Replace:
  ```go
  job.Status = result.Status
  ```
  With:
  ```go
  if result.Success {
      job.Status = "completed"
  } else {
      job.Status = "failed"
  }
  ```
  Keep all other lines in handleResult() unchanged (job.Result, job.Error, job.CompletedAt,
  job.DurationMS, job.Metrics, resultChan notify).

  After edits verify imports: still needs `context`, `encoding/json`, `fmt`, `log`, `sync`,
  `time`, `gpu-compute-orchestrator/pkg/mqtt`, `gpu-compute-orchestrator/pkg/nomad`,
  `github.com/google/uuid`. The `encoding/json` import may now be unused — remove it if so.
- **Test**: `go build ./pkg/compute/` and `go build .` must pass
- **Notes**: The `Dispatch()` call signature is `Dispatch(jobID string, meta map[string]string)`
  and internally adds `meta["job_id"] = jobID` (see nomad/dispatcher.go:53). So we do NOT
  set `meta["job_id"]` in manager — that's handled by Dispatcher.

### [x] 4. Update pkg/mqtt/types_test.go
- **File**: `pkg/mqtt/types_test.go`
- **Action**:
  - Delete `TestJobParams_JSONRoundTrip` entirely (the `JobParams` struct is gone).
  - Update `TestJobResult_JSONRoundTrip`:
    - Replace `Status: "completed"` with `Success: true`
    - Replace the assertion `if got.Status != original.Status` with
      `if got.Success != original.Success`
    - Remove `DurationMS` from the test (keep `Metrics` check, keep `JobID` check)
    - Add `WorkerID: "test-worker-1"` to the original struct and assert it round-trips
- **Test**: `go test -v -race ./pkg/mqtt/` — all remaining tests must pass
- **Notes**: `TestTopicBuilder_*` and `TestTopicPatternConstants` and `TestJobLog_JSONRoundTrip`
  are unchanged.

### [x] 5. Update pkg/compute/types_test.go
- **File**: `pkg/compute/types_test.go`
- **Action**:
  - In `TestRequest_Defaults`: replace `_ = r.Parameters` with `_ = r.Payload`, and replace
    the assertion `if r.Parameters != nil` with `if r.Payload != nil`
  - No other changes needed (Job tests don't reference Parameters/Payload).
- **Test**: `go test -v -race ./pkg/compute/` must pass

### [x] 6. Update integration tests
- **Files**: `integration_test/api_test.go`, `integration_test/flow_test.go`
- **Action**:

  **api_test.go** — update every request body string that contains `"parameters"`:
  - `"parameters":{"test":true}` → `"payload":{"test":true}`
  - `"parameters":{}` → `"payload":{}`
  (There are ~5 occurrences across TestSubmitJob_Async, TestListJobs_AfterSubmit,
  TestGetJob_Existing, TestCancelJob_Success, TestFullFlow tests.)
  Also fix the timeout field in flow_test.go request: `"timeout":10000000000` is a
  `time.Duration` in nanoseconds (10s) — keep as-is since the Request struct field is unchanged.

  **flow_test.go** — update `FakeWorker` to publish the new worker result format:
  - Change `FakeWorker.results` map type from `map[string]string` to `map[string]bool`
    (maps `"*"` → `true` for success, `false` for failure)
  - Change `SetResult(status string)` → `SetResult(success bool)`
  - Update `publishResult(jobID, status string)` → `publishResult(jobID string, success bool)`:
    ```go
    func (fw *FakeWorker) publishResult(jobID string, success bool) {
        result := map[string]interface{}{
            "job_id":    jobID,
            "success":   success,
            "worker_id": "fake-worker",
            "timestamp": time.Now().UTC().Format(time.RFC3339),
        }
        if success {
            result["result"] = map[string]bool{"test": true}
        } else {
            result["error"] = "compute failed (simulated)"
        }
        // ... publish unchanged
    }
    ```
  - Update `onParams` to call `publishResult(jobID, success)` with the bool value
  - Update `TestFullFlow_JobCompletion` call: `fw.SetResult("completed")` → `fw.SetResult(true)`
  - Update `TestFullFlow_JobFailure` call: `fw.SetResult("failed")` → `fw.SetResult(false)`
  - The assertions `job["status"].(string) == "completed"` and `status != "failed"` stay
    valid because the orchestrator still derives `job.Status` from `result.Success`.

- **Test**: Integration tests compile with `go test -tags integration -run ^$ ./integration_test/`
  (compile-only check, no stack needed)

### [x] 7. Replace static/index.html with generic debug console
- **File**: `static/index.html`
- **Action**: Replace entirely with a minimal generic job submission and monitoring UI:
  - Title: "GPU Compute Orchestrator"
  - Two input fields: operation name (text), payload (JSON textarea)
  - Submit button → `POST /compute` with `{operation, payload: parsed_json}`
  - Error display for invalid JSON in payload textarea
  - Auto-refreshing job list (2s interval) — show: job_id (truncated), operation, status,
    duration, created_at
  - Click on a job row → show full job JSON in a detail panel (pretty-printed)
  - Basic dark theme, minimal CSS — no external dependencies
  - Remove all: CSV upload, anchor data, pattern cards, LLM-related display, "Semantic Anchor
    Diffusion" subtitle
- **Test**: Open http://localhost:8080 in browser (visual verification only; or check file
  has no references to `diffusion`, `anchor`, `pattern`, `csv` with grep)
- **Notes**: The existing 566-line index.html is entirely replaced. No need to preserve any
  existing UI logic — the new page is simpler.

### [x] 8. Verify build and unit tests
- **Action**:
  - `go build ./...` — must succeed
  - `go test -v -race ./pkg/...` — all tests must pass
  - `grep -ri "diffusion\|anchor_data\|semantic\|JobParams\|Parameters\b" pkg/` — should
    return no matches in non-test Go source files (test files may have `Parameters` in
    historic comments but not in code)
- **Notes**: `encoding/json` import in manager.go — check if still used after removing the
  JSON marshal of `JobParams`. If handleResult still uses `json.Unmarshal`, it stays.
  Actually `handleResult`, `handleStatus`, `handleLog` all use `json.Unmarshal` so the import
  stays.

### [x] 9. Update CLAUDE.md and README.md
- **Files**: `CLAUDE.md`, `README.md`
- **Action**:

  **CLAUDE.md** — in the Constraints section:
  - Remove the line: `` `semantic_diffusion` / `diffusion` jobs publish raw CSV ... ``
  - Replace with: `All jobs publish \`{"job_id": "...", ...payload_fields}\` to params topic.
    Workers return \`{"job_id", "success": bool, "result": {...}, "worker_id", "timestamp"}\`.`

  **README.md** — update the API Request example:
  - Change `"parameters"` → `"payload"` in the POST /compute example
  - Add a brief "Worker Protocol" subsection under MQTT Topics showing the params and result
    JSON shapes (copy from plan Context above)

- **Test**: `grep "parameters" README.md` — should not appear in code examples (only in prose
  if at all)

## Completion Criteria
- [ ] `go build ./...` passes with no errors
- [ ] `go test -v -race ./pkg/...` passes (≥9 tests; one test deleted from mqtt)
- [ ] `grep -r "diffusion\|anchor_data\|semantic_diffusion\|JobParams" pkg/` returns nothing
- [ ] `grep "\"parameters\"" pkg/compute/types.go` returns nothing (field is now `payload`)
- [ ] `grep "\"parameters\"" integration_test/` returns nothing
- [ ] `static/index.html` has no references to `diffusion`, `anchor`, `csv`, `pattern`
- [ ] Worker result format (`success: bool`) is handled in manager.go

## New Constraints or Gotchas
- **MQTT params format changed**: All operations now publish `{"job_id": "...", ...payload_fields}`
  (flat merge). Workers that previously expected the old `JobParams` wrapper struct
  `{job_id, operation, parameters: {...}}` will need updating in their repos.
- **Worker result format**: Workers must now send `"success": true/false` (bool), not
  `"status": "completed"/"failed"` (string). The orchestrator no longer accepts the old
  string-based status in the result message.
- **HTTP API field rename**: Clients calling `POST /compute` must send `"payload"` not
  `"parameters"`. This is a breaking change to the HTTP API.
