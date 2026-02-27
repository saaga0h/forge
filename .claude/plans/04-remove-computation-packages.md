# Plan: Remove Computation Packages — Restore Pure Orchestrator
## Created: 2026-02-27
## Complexity: opus
## Recommended implementation model: haiku

## Context

All computational code (LLM pattern transcoding, semantic diffusion, PostgreSQL pattern
storage, anchor CSV handling) has moved to a separate repository. This repo's sole purpose
is **pure GPU compute orchestration**: receive HTTP job requests → dispatch to Nomad via
MQTT → track status → return results. Nothing else.

During an earlier refactoring, computational code was not removed but instead _embedded_
into the orchestrator: `pkg/compute/manager.go` now imports `pkg/patterns`, initialises
an LLM client and pattern cache in `NewManager()`, and calls `processSemanticChains()`
after every completed job. Additionally, `cmd/test-patterns/` was added as a developer
CLI tool that belongs in the other repo.

`main.go` at root is already correct — it matches the original orchestrator design
exactly. It stays at root (single-binary repo; `cmd/` adds no value here).

**What must be deleted:**
- `cmd/test-patterns/` — dev CLI, moved to other repo
- `pkg/patterns/` — LLM pattern transcoding, moved to other repo
- `pkg/database/` — PostgreSQL pattern storage, moved to other repo
- `pkg/anchors/` — semantic anchor CSV parsing and DiffusionManager helper (moved)
- `configs/llm.yaml` — Ollama config, no longer relevant
- `testdata/` — sample diffusion JSON used by test-patterns CLI
- `migrations/` — SQL migrations for pattern tables (moved to other repo's DB)

**What must change:**
- `pkg/compute/manager.go` — surgically remove patterns import and all LLM/pattern
  processing code while keeping the pure job dispatch/tracking logic intact
- `docker-compose.enhanced.yaml` — remove PostgreSQL and Ollama services
- `go.mod` / `go.sum` — run `go mod tidy` to drop `github.com/lib/pq` (and any other
  deps that become orphaned)

**What must NOT change:**
- `main.go` — already correct, stays at root
- `pkg/compute/types.go`, `pkg/compute/types_test.go`
- `pkg/mqtt/` — all files
- `pkg/nomad/` — all files
- `static/` — web UI
- `docker-compose.yaml`, `docker-compose.test.yaml`
- `configs/config.yaml`, `configs/mosquitto.conf`
- `nomad-server/`, `nomad-jobs/`
- `integration_test/` — all integration tests (they only use HTTP + Paho MQTT)
- `Dockerfile`

## Prerequisites
- [x] `go build ./...` currently passes (it does, minus the patterns cross-dependency)
- [x] No uncommitted work that would be lost by these deletions

## Tasks

### 1. Strip patterns/LLM code from pkg/compute/manager.go
- **File**: `pkg/compute/manager.go`
- **Action**: Remove the following, leaving the pure job management logic intact:
  - Remove `"gpu-compute-orchestrator/pkg/patterns"` from imports
  - Remove `"os"` from imports (only used for `os.Getenv("OLLAMA_URL"/"OLLAMA_MODEL")`)
  - Remove fields from `Manager` struct: `llmClient *patterns.LLMClient`, `patternCache *patterns.Cache`
  - In `NewManager()`: remove the entire `// Initialize pattern processing components`
    block (lines that create `cache`, read `ollamaURL`/`ollamaModel`, create `llmClient`)
  - In `handleResult()`: remove the `// Process semantic chains if job completed` block
    (the `if result.Status == "completed" && result.Result != nil` goroutine call)
  - Delete entire function `processSemanticChains()`
  - Delete entire function `parseSemanticResult()`
  - Delete entire function `createMockChainStatistics()`
  - After edits, verify remaining imports are: `context`, `encoding/json`, `fmt`, `log`,
    `sync`, `time`, `gpu-compute-orchestrator/pkg/mqtt`, `gpu-compute-orchestrator/pkg/nomad`,
    `github.com/google/uuid`
- **Pattern**: The resulting Manager struct should have only 4 fields: `mqttClient`,
  `nomad`, `jobs`, `config` — matching the simple design in the pasted `main.go`
- **Test**: `go build ./pkg/compute/` — must compile without errors
- **Notes**: `uuid` is used in `Submit()` to generate job IDs — keep it. The
  `subscribeToTopics()` function stays (it wires up MQTT result/status/log handlers).

### 2. Delete cmd/test-patterns/
- **File**: `cmd/test-patterns/main.go` (and `cmd/` directory if now empty)
- **Action**: Delete `cmd/test-patterns/main.go`. Then delete the `cmd/` directory
  entirely (it will be empty after removing test-patterns).
- **Test**: `ls cmd/` should fail (directory gone) or be empty
- **Notes**: Use `rm -rf cmd/`

### 3. Delete pkg/patterns/
- **File**: `pkg/patterns/` directory (aggregator.go, cache.go, llm_client.go,
  signature.go, types.go, and all *_test.go files)
- **Action**: `rm -rf pkg/patterns/`
- **Test**: `ls pkg/patterns/` should fail
- **Notes**: This also removes the unit tests written for patterns (aggregator_test.go,
  cache_test.go, signature_test.go, llm_client_test.go). That is intentional — the tests
  belong with the code in the other repo.

### 4. Delete pkg/database/
- **File**: `pkg/database/` directory (patterns.go)
- **Action**: `rm -rf pkg/database/`
- **Test**: `ls pkg/database/` should fail

### 5. Delete pkg/anchors/
- **File**: `pkg/anchors/` directory (handler.go, parser.go, parser_test.go)
- **Action**: `rm -rf pkg/anchors/`
- **Test**: `ls pkg/anchors/` should fail

### 6. Delete leftover configuration and test artifacts
- **Files**: `configs/llm.yaml`, `testdata/`, `migrations/`
- **Action**:
  - `rm configs/llm.yaml`
  - `rm -rf testdata/`
  - `rm -rf migrations/`
- **Test**: Confirm files/dirs are gone
- **Notes**: `configs/config.yaml` and `configs/mosquitto.conf` stay.

### 7. Simplify docker-compose.enhanced.yaml
- **File**: `docker-compose.enhanced.yaml`
- **Action**: Remove the `postgres` and `ollama` services entirely, and remove any
  volume/network entries that only those services use. Read the file first to understand
  what to keep.
- **Test**: `docker compose -f docker-compose.enhanced.yaml config --quiet` validates YAML
- **Notes**: If after removing postgres+ollama the enhanced compose becomes identical to
  the basic docker-compose.yaml, consider deleting it entirely (ask user) — or keep it
  with a comment that it's reserved for future extended services.

### 8. Run go mod tidy
- **File**: `go.mod`, `go.sum`
- **Action**: Run `go mod tidy` from project root. This will remove `github.com/lib/pq`
  and any other deps that are no longer imported by any package.
- **Test**: `go build ./...` must pass after tidy. `go test -v -race ./...` must pass.
- **Notes**: After tidy, `go.mod` should no longer list `github.com/lib/pq`.
  `github.com/eclipse/paho.mqtt.golang`, `github.com/hashicorp/nomad/api`, and
  `github.com/google/uuid` should remain as direct deps.

## Completion Criteria
- [x] `go build .` succeeds (builds the orchestrator binary)
- [x] `go test -v -race ./...` passes — only pkg/compute/, pkg/mqtt/, pkg/nomad/ and
  integration_test/ remain in scope (excluding integration tag)
- [x] `go build ./...` has no errors (no orphaned imports)
- [x] `pkg/patterns/`, `pkg/database/`, `pkg/anchors/`, `cmd/` directories do not exist
- [x] `pkg/compute/manager.go` does not import `pkg/patterns` or `os`
- [x] `go.mod` does not list `github.com/lib/pq` as a dependency
- [x] `docker compose -f docker-compose.enhanced.yaml config --quiet` validates

## New Constraints or Gotchas
- After this cleanup, `go test ./...` will cover pkg/compute, pkg/mqtt, pkg/nomad only.
  The integration_test/ suite requires `-tags integration`. Total unit test count drops
  significantly (patterns tests are gone) — this is expected and correct.
- The `docker-compose.enhanced.yaml` may become redundant after removing postgres/ollama.
  Consider whether to keep it as a placeholder or delete it.
