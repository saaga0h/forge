# Plan: Comprehensive Test Suite
## Created: 2026-02-27
## Complexity: opus
## Recommended implementation model: sonnet

## Context

The project currently has one test file (`pkg/patterns/aggregator_test.go`) with 4 tests
covering only pure aggregation functions. No HTTP handler tests, no integration tests,
and no coverage of cache, signature, anchors, MQTT types, or compute types.

**Approach chosen:** Unit tests for pure packages only (no production code refactoring to
add interfaces), plus integration tests with a dedicated docker-compose test stack. The
compute.Manager and HTTP handlers are not unit-testable without interface extraction, so
they are covered exclusively by integration tests.

**Test style:** stdlib `testing` package only, consistent with existing tests. No testify.
Table-driven tests where multiple inputs are needed.

**Integration test pattern:** A "fake worker" Go helper subscribes to MQTT job params
topics and publishes fake results back. This lets integration tests exercise the full
orchestrator flow (HTTP submission → Nomad dispatch → MQTT result) without a real GPU
compute worker.

**No production code changes** in Phase 1 (unit tests). Phase 2 adds
`docker-compose.test.yaml` and `integration_test/` directory.

## Prerequisites
- [ ] `go build ./...` passes from project root (it does after the refactor)
- [ ] `go test ./...` passes (4 existing tests pass)
- [ ] Docker available for integration tests

## Tasks

### [x] 1. Extend pkg/patterns/aggregator_test.go with edge cases
- **File**: `pkg/patterns/aggregator_test.go`
- **Action**: Add test cases for edge conditions not currently covered:
  - `TestAggregateChainStatistics_EmptyAnchors` — chain with anchor IDs that don't exist
    in the anchor slice (all filtered out); verify no panic, empty stats returned
  - `TestAggregateChainStatistics_SingleAnchor` — chain with one anchor; verify no
    transitions in Sequence, SpanHours = 0
  - `TestAggregateChainStatistics_IdenticalLocations` — all anchors at same location;
    verify LocationStats has single entry, no self-transitions in Sequence
  - `TestClassifyTimeOfDay_Boundaries` — test boundary values: exactly 6.0 (start of
    morning), exactly 12.0, exactly 18.0, exactly 22.0
  - `TestFilterAnchors_EmptyIDs` — empty ID list returns empty slice
  - `TestFilterAnchors_AllMissing` — IDs that don't exist in anchor slice
  - `TestComputeDimensionMagnitudes_AllZero` — all zero vectors → all magnitudes 0
  - `TestComputeDimensionMagnitudes_EmptySlices` — empty slices → 0 magnitude, no panic
- **Pattern**: Follow existing table-driven style in `pkg/patterns/aggregator_test.go:112`
  (the `TestClassifyTimeOfDay` pattern with `tests := []struct{...}{}`)
- **Test**: `go test -v ./pkg/patterns/`

### [x] 2. Add pkg/patterns/cache_test.go
- **File**: `pkg/patterns/cache_test.go`
- **Action**: Create new test file covering:
  - `TestCacheGetSet` — Set a pattern, Get it back, verify fields match
  - `TestCacheHas` — Has returns false before Set, true after, false after Delete
  - `TestCacheTTLExpiry` — Set entry with very short TTL cache (1ms), sleep, verify Get
    returns nil. Create cache with `NewCache(1 * time.Millisecond)` for this test.
  - `TestCacheClear` — Set multiple entries, Clear, verify Size() == 0
  - `TestCacheSize` — Size reflects actual count
  - `TestCacheStats` — after hits and misses, Stats().Hits and Stats().Misses are correct
  - `TestCacheHitRate` — HitRate returns 0 when no accesses, correct ratio after gets
  - `TestCacheGetOrSet_CacheHit` — GetOrSet returns cached value, compute func NOT called
  - `TestCacheGetOrSet_CacheMiss` — GetOrSet calls compute func, caches result
  - `TestCacheGetOrSet_ComputeError` — compute func returns error, GetOrSet propagates it
  - `TestCacheConcurrent` — 50 goroutines simultaneously Get/Set/Delete; use
    `t.Parallel()` and verify no races (run with `-race`)
- **Pattern**: Follow `pkg/patterns/aggregator_test.go` style — package `patterns`, stdlib
  testing, construct types directly
- **Notes**: For TTL expiry test, `cleanup()` runs in a goroutine started by `cleanupLoop`
  every hour — don't rely on that. Just call `Get` after TTL expires; the TTL check
  happens in `Get`, not only in cleanup. Verify by reading cache.go:Get implementation.
- **Test**: `go test -v -race ./pkg/patterns/`

### [x] 3. Add pkg/patterns/signature_test.go
- **File**: `pkg/patterns/signature_test.go`
- **Action**: Create new test file covering:
  - `TestComputeSignature_Deterministic` — same ChainStatistics produces same signature
    across multiple calls
  - `TestComputeSignature_DifferentChains` — different ChainIDs produce different
    signatures
  - `TestComputeSignature_SimilarChains` — very similar stats produce same signature
    (signature is designed to be stable under small differences — verify rounding works)
  - `TestCompareSignatures_Equal` — same signature string returns true
  - `TestCompareSignatures_Different` — different strings return false
  - `TestSignatureSimilarity_Identical` — same stats → similarity = 1.0
  - `TestSignatureSimilarity_DifferentTimeOfDay` — morning vs night → similarity < 0.5
  - `TestSignatureSimilarity_DifferentLocations` — completely different location maps →
    locationSimilarity contributes to overall score
  - `TestSignatureSimilarity_Range` — verify result is always in [0.0, 1.0]
- **Pattern**: Build minimal `ChainStatistics` structs with `patterns.ChainStatistics{...}`
  — same as in `aggregator_test.go`
- **Test**: `go test -v ./pkg/patterns/`

### [x] 4. Add pkg/patterns/llm_client_test.go
- **File**: `pkg/patterns/llm_client_test.go`
- **Action**: Create new test file covering:
  - `TestLLMClient_MockMode` — create `LLMClient{Mock: true, Cache: NewCache(1*time.Hour)}`,
    call `TranscodeChain`, verify non-nil result with expected mock fields
  - `TestLLMClient_CacheHit` — TranscodeChain twice with same stats; second call should
    use cache (set `Mock: false` but intercept by pre-populating cache with matching
    signature)
  - `TestLLMClient_HTTPMockServer` — use `net/http/httptest.NewServer` to serve a fake
    Ollama response; create LLMClient pointing at test server URL; call TranscodeChain;
    verify correct PatternDescription returned. Fake response:
    ```json
    {"response": "{\"name\":\"Test Pattern\",\"description\":\"A test\",\"intent\":\"Testing\",\"characteristics\":{\"primary_locations\":[\"home\"],\"typical_duration\":\"1h\",\"frequency\":\"daily\",\"time_of_day\":\"morning\"},\"key_observations\":[\"obs1\"],\"confidence\":\"high\"}"}
    ```
  - `TestLLMClient_HTTPError` — test server returns 500; verify error propagated after
    retries (set `RetryAttempts: 1` to keep test fast)
  - `TestLLMClient_InvalidJSON` — test server returns 200 with invalid JSON in response
    field; verify parse error returned
  - `TestFormatPrompt` — call `formatPrompt` with populated stats; verify output contains
    expected strings (chain ID, location names, time of day). formatPrompt is unexported
    so test must be in package `patterns` (not `patterns_test`)
- **Notes**: Import `net/http/httptest` (stdlib). LLMClient.HTTPClient is exported so you
  can set it to `httptest.Server.Client()` for TLS servers, but simpler to use
  `httptest.NewServer` (non-TLS) and just set `BaseURL: ts.URL`.
- **Test**: `go test -v ./pkg/patterns/`

### [x] 5. Add pkg/anchors/parser_test.go
- **File**: `pkg/anchors/parser_test.go`
- **Action**: Create new test file covering:
  - `TestFormatAsCSV_RoundTrip` — create AnchorDataset with 3 anchors (128D vectors),
    call FormatAsCSV, write to temp file, call ParseAnchorCSV on it, verify all anchors
    match original. Use `os.CreateTemp` + `defer os.Remove`.
  - `TestFormatAsCSV_HeaderLines` — verify output starts with `# Jeeves Anchor Export v1.0`
    and includes `# Dimensions: 128`
  - `TestParseAnchorCSV_InvalidFile` — non-existent path returns error
  - `TestParseAnchorRecord_TooFewFields` — record with < 4 fields returns error
  - `TestParseAnchorRecord_InvalidTimestamp` — non-integer timestamp returns error
  - `TestParseAnchorRecord_InvalidVector` — non-float vector value returns error
  - `TestAnchorDataset_Statistics_Empty` — empty dataset returns `num_anchors: 0` map
  - `TestAnchorDataset_Statistics_MultipleLocations` — verify location counts in stats
  - `TestParseDiffusionResult_Basic` — parse multi-chain result text with metadata
    comments; verify chain assignments and NumChains
  - `TestParseDiffusionResult_EmptyInput` — empty string returns empty result, no panic
- **Notes**: `parseAnchorRecord` is unexported; test it via `ParseAnchorCSV` with a
  temp file, or note that it's implicitly tested via the round-trip test. For the
  unexported function tests, use package `anchors` (not `anchors_test`).
- **Test**: `go test -v ./pkg/anchors/`

### [x] 6. Add pkg/mqtt/types_test.go
- **File**: `pkg/mqtt/types_test.go`
- **Action**: Create new test file covering:
  - `TestTopicBuilder_Params` — verify `NewTopicBuilder("job-123").Params()` returns
    `"gpu/jobs/job-123/params"` (check against MQTT topic convention in CLAUDE.md)
  - `TestTopicBuilder_Status` — same for Status()
  - `TestTopicBuilder_Result` — same for Result()
  - `TestTopicBuilder_Logs` — same for Logs()
  - `TestTopicPatternConstants` — verify `TopicPatternResults` uses `+` wildcard in right
    position to match a specific job's result topic
  - `TestJobParams_JSONRoundTrip` — marshal JobParams to JSON and back, verify fields
  - `TestJobResult_JSONRoundTrip` — marshal JobResult with Metrics, verify Metrics fields
  - `TestJobLog_JSONRoundTrip` — marshal JobLog, verify Level and Message preserved
- **Notes**: Check the actual topic format in `pkg/mqtt/types.go`. TopicBuilder returns
  e.g. `gpu/jobs/{jobID}/params`. Constants use `+` MQTT wildcard for subscriptions.
- **Test**: `go test -v ./pkg/mqtt/`

### [x] 7. Add pkg/compute/types_test.go
- **File**: `pkg/compute/types_test.go`
- **Action**: Create new test file covering:
  - `TestJob_IsComplete_States` — table-driven: verify IsComplete returns true for
    "completed", "failed", "timeout", "cancelled"; false for "pending", "dispatched",
    "starting", "computing", and empty string
  - `TestJob_Duration_WithCompletedAt` — set CreatedAt and CompletedAt, verify Duration()
    returns correct difference
  - `TestJob_Duration_NoCompletedAt` — CompletedAt is nil, verify Duration() returns
    time since CreatedAt (approximately, check > 0)
  - `TestRequest_Defaults` — create Request with empty fields; verify no panics when
    accessing zero values
- **Pattern**: Package `compute`, stdlib testing
- **Test**: `go test -v ./pkg/compute/`

### [x] 8. Add docker-compose.test.yaml
- **File**: `docker-compose.test.yaml`
- **Action**: Create a self-contained test stack. Key differences from docker-compose.yaml:
  - Different container names to avoid conflicts with dev stack
  - Different port mappings to avoid conflicts
  - Uses official `hashicorp/nomad:1.8` in dev mode (no custom Dockerfile — integration
    tests don't need the dev job auto-registered since fake worker handles results)
  - Orchestrator built from source (context: .)
  - No Consul (integration tests don't need service discovery)
  - No MQTT explorer

  Services:
  - `mqtt-test`: eclipse-mosquitto:2.0, port 11883:1883 (offset to avoid dev conflict)
  - `nomad-test`: hashicorp/nomad:1.8, dev mode, ports 14646:4646
  - `orchestrator-test`: build from `.`, ports 18080:8080, env vars pointing to
    mqtt-test:1883 and nomad-test:4646

  Environment variables the orchestrator needs (pointing to test services):
  ```yaml
  MQTT_BROKER: tcp://mqtt-test:1883
  NOMAD_ADDR: http://nomad-test:4646
  NOMAD_JOB_NAME: gpu-compute
  PORT: 8080
  OLLAMA_URL: http://localhost:11434  # won't be used in tests
  ```

  Add healthchecks so `depends_on` can gate orchestrator startup:
  - mqtt-test healthcheck: `mosquitto_pub -h localhost -t test -m test`
  - nomad-test healthcheck: `nomad status`

- **Test**: `docker-compose -f docker-compose.test.yaml config` validates YAML

### [x] 9. Add integration_test/health_test.go
- **File**: `integration_test/health_test.go`
- **Action**: Create integration test for `/health` endpoint.
  ```go
  //go:build integration
  package integration_test
  ```
  - `TestHealth` — GET http://localhost:18080/health, verify 200, JSON body has
    `"status": "ok"`, `"version"` field present, `"time"` field parses as RFC3339
  - Helper function `orchestratorURL()` reads `TEST_ORCHESTRATOR_URL` env var,
    defaults to `http://localhost:18080`
  - Helper `skipIfUnavailable(t)` — attempts GET /health; if connection refused or
    timeout, calls `t.Skip("orchestrator not running")`
- **Notes**: The `//go:build integration` tag means these tests only run with
  `go test -tags integration ./...`. Without the tag they are invisible to `go test ./...`
- **Test**: With stack running: `go test -tags integration -v ./integration_test/`

### [x] 10. Add integration_test/api_test.go
- **File**: `integration_test/api_test.go`
- **Action**: Create integration tests for the full HTTP API:
  ```go
  //go:build integration
  package integration_test
  ```
  - `TestSubmitJob_Async` — POST /compute with valid body; verify 202, response has
    `job_id`, `status: "pending"` or `"dispatched"`
  - `TestSubmitJob_InvalidBody` — POST /compute with malformed JSON; verify 400
  - `TestSubmitJob_WrongMethod` — GET /compute; verify 405
  - `TestListJobs_Empty` — GET /jobs on fresh stack; verify 200, `count: 0`, `jobs: []`
  - `TestListJobs_AfterSubmit` — submit job, GET /jobs, verify count >= 1
  - `TestGetJob_NotFound` — GET /jobs/nonexistent-id; verify 404
  - `TestGetJob_Existing` — submit job, GET /jobs/{id}; verify 200, job fields present
  - `TestCancelJob_NotFound` — POST /jobs/cancel?job_id=nonexistent; verify 500 (or 404
    depending on implementation — check and match actual behavior)
  - `TestCancelJob_Success` — submit job, cancel it, verify cancel returns 200 with
    `status: "cancelled"`
  - Use `skipIfUnavailable(t)` at start of each test
- **Test**: `go test -tags integration -v ./integration_test/`

### [x] 11. Add integration_test/flow_test.go (fake worker pattern)
- **File**: `integration_test/flow_test.go`
- **Action**: Create end-to-end flow test using MQTT fake worker:
  ```go
  //go:build integration
  package integration_test
  ```

  Fake worker pattern:
  1. Connect to MQTT broker (use Paho client, same dependency already in go.mod)
  2. Subscribe to `gpu/jobs/+/params` to detect when orchestrator publishes job params
  3. Submit job via HTTP POST /compute/sync (synchronous endpoint with timeout)
  4. When params received: publish fake JobResult JSON to `gpu/jobs/{job_id}/result`
  5. The orchestrator receives result via MQTT and marks job complete
  6. /compute/sync returns with the completed job

  Tests:
  - `TestFullFlow_JobCompletion` — submit job via /compute/sync with 10s timeout;
    fake worker publishes result `{"job_id":"{id}","status":"completed","result":{"test":true},"duration_ms":500}`;
    verify HTTP response has `status: "completed"`
  - `TestFullFlow_JobFailure` — same but fake worker publishes `status: "failed"` with
    `"error": "compute failed"`; verify response has `status: "failed"`

  Helper: `newMQTTFakeWorker(t, brokerURL) *FakeWorker` — struct with Subscribe and
  PublishResult methods, calls t.Cleanup for disconnect.

  MQTT broker URL read from `TEST_MQTT_BROKER` env var, defaults to `tcp://localhost:11883`

- **Notes**: The Paho MQTT client is already in go.mod as a direct dependency.
  `compute/sync` blocks waiting for result (up to timeout). The fake worker must publish
  result AFTER subscribing to params. Race: fake worker goroutine may miss the params
  publish if subscription is set up after orchestrator publishes. Solution: subscribe
  to `gpu/jobs/+/params` BEFORE submitting the job.
- **Test**: `go test -tags integration -v -timeout 30s ./integration_test/`

### [x] 12. Update Makefile with test targets
- **File**: `Makefile`
- **Action**: Add new targets:
  ```makefile
  test-unit: ## Run unit tests only (no external dependencies)
  	go test -v -race -coverprofile=coverage.out ./pkg/... ./...

  test-integration: ## Run integration tests (requires docker-compose.test.yaml stack)
  	docker-compose -f docker-compose.test.yaml up -d --wait
  	go test -tags integration -v -timeout 60s ./integration_test/ ; \
  	EXIT_CODE=$$? ; \
  	docker-compose -f docker-compose.test.yaml down ; \
  	exit $$EXIT_CODE

  test-all: test-unit test-integration ## Run all tests
  ```
  Also update existing `test` target description to clarify it runs unit tests:
  ```makefile
  test: ## Run unit tests with race detector and coverage
  ```
- **Notes**: `docker-compose up --wait` requires compose v2.1+. If not available, use
  `up -d` followed by a retry loop checking /health. Keep it simple — if `--wait` fails,
  document in README.
- **Test**: `make test-unit` passes; `make help` shows all new targets

## Completion Criteria
- [ ] `go test -v -race ./...` runs all 4 phases of tests + all new unit tests, 0 failures
- [ ] `go test -v -race ./pkg/patterns/` — at minimum 20 test functions pass
- [ ] `go test -v -race ./pkg/anchors/` — at least 10 test functions pass
- [ ] `go test -v -race ./pkg/mqtt/` — at least 8 test functions pass
- [ ] `go test -v -race ./pkg/compute/` — at least 4 test functions pass
- [ ] `make test-unit` passes
- [ ] `go test -tags integration -v ./integration_test/` passes with the test stack running
- [ ] `docker-compose.test.yaml` can be brought up and down cleanly without conflicting
      with the dev stack on the same machine
- [ ] All tests pass with `-race` flag (no race conditions)

## New Constraints or Gotchas
- Integration tests use `//go:build integration` tag — they are INVISIBLE to plain
  `go test ./...`. Always run with `-tags integration` to include them.
- Integration test ports are offset by 10000 from dev stack to allow both to run
  simultaneously: MQTT on 11883, Nomad on 14646, orchestrator on 18080.
- The fake worker in flow tests MUST subscribe to MQTT before submitting the job, or
  the job params publish (which is RETAINED) will be replayed on subscribe. Actually,
  MQTT retained messages mean subscribing AFTER is fine — broker replays the retained
  params message. But note: if the orchestrator already processed another connection's
  retained message for the same topic from a previous test run, cleanup between tests
  matters. Use unique job IDs (uuid) so retained messages don't interfere.
- `pkg/patterns/llm_client_test.go` must be in package `patterns` (not `patterns_test`)
  to access the unexported `formatPrompt` method.
- `pkg/anchors/parser_test.go` for `parseAnchorRecord` must be in package `anchors`
  to access the unexported function.
- The `cleanupLoop` goroutine in Cache starts on `NewCache()`. Tests that create a Cache
  will have a background goroutine. This is fine for unit tests but be aware it prevents
  the test process from exiting if something holds a reference. The goroutine exits when
  the cache is garbage collected (it uses a ticker with no stop mechanism — acceptable
  in tests since they're short-lived).
