# Plan: Documentation Cleanup and README Refresh
## Created: 2026-02-27
## Complexity: sonnet
## Recommended implementation model: haiku

## Context

All computational code (Fortran, LLM/patterns, PostgreSQL, Ollama) was removed in earlier
refactoring. Nine root-level markdown files from November 2025 still describe the old
system in detail — they reference `cmd/test-patterns/`, `pkg/patterns/`, PostgreSQL, Ollama,
and Fortran workers, none of which exist anymore. They are dead documentation and should be
deleted entirely; none contain salvageable content for the current repo.

README.md was partially updated but still carries stale references: wrong MQTT topic prefix
(`gpu/jobs/` instead of `compute/jobs/`), LLM/patterns mentions in the architecture, a
"Full Stack with PostgreSQL + LLM" Quick Start step, DB/Ollama env vars in the configuration
section, a `cmd/` directory reference, and Fortran worker references in Compute Workers.

Goal: delete all stale docs, rewrite README.md to accurately reflect the current pure
orchestrator. Lean and factual — describe what exists, nothing more.

## Prerequisites
- [x] Plans 03 and 04 are complete (test suite added, computation packages removed)
- [x] go build ./... passes
- [x] docker-compose.enhanced.yaml no longer has postgres/ollama

## Tasks

### [x] 1. Delete 9 stale root-level documentation files
- **Files**:
  - `TESTING_GUIDE.md`
  - `DOCKER_COMPOSE_SETUP.md`
  - `WEB_UI_GUIDE.md`
  - `SETUP_MAC.md`
  - `MAC_VS_GPU_GUIDE.md`
  - `PATTERNS_IMPLEMENTATION_SUMMARY.md`
  - `MQTT_TOPIC_FIX.md`
  - `MQTT_CONNECTION_ISSUE.md`
  - `NOMAD_RAWEXEC_SETUP.md`
- **Action**: `rm` all nine files. None contain content relevant to the current repo.
- **Test**: `ls *.md` at project root should show only `README.md` and `CLAUDE.md`
- **Notes**: Do not touch `CLAUDE.md` (current, accurate). Do not touch `.claude/plans/`
  (internal planning docs, not user-facing).

### [x] 2. Rewrite README.md
- **File**: `README.md`
- **Action**: Replace the entire file with a clean, accurate README covering only what
  exists today. Target ~150-180 lines. Structure:

  **Header**: One sentence — what it is.

  **Architecture**: Keep the ASCII diagram but remove "Aggregates patterns" from the
  orchestrator box. Worker box should say "Compute Worker (any language/container)" not
  "Fortran Worker".

  **Components**: 4 components — Go Orchestrator, MQTT Broker, Nomad, Compute Workers.
  Remove LLM pattern aggregation from orchestrator description. Remove Julia worker link.
  Keep the "workers live in separate repos" note.

  **Prerequisites**: Keep as-is (Docker, Go 1.23, Make, Git for dev machine; Ubuntu/ROCm
  for GPU node).

  **Quick Start**: 3 steps only:
  1. `make build` (or `make docker-build`)
  2. `docker-compose up -d` (dev stack)
  3. Submit a test job via curl

  Remove step "Start Full Stack (with PostgreSQL + LLM)" entirely.
  Remove step "Register Nomad Job" via setup-and-deploy.sh (that's GPU node setup, not
  quick start).

  **API Reference**: Keep exactly as-is — it's accurate.

  **MQTT Topics**: Fix prefix from `gpu/jobs/` to `compute/jobs/`.

  **Configuration**: Keep only the 4 current env vars:
  `MQTT_BROKER`, `NOMAD_ADDR`, `NOMAD_JOB_NAME`, `PORT`.
  Remove the "Enhanced Stack" configuration section entirely (no DB/Ollama vars).

  **Testing**: Keep the existing Testing section — it's accurate (unit tests, integration
  tests, port table, coverage).

  **GPU Node Setup**: Keep — this is still relevant production info. Rename from part of
  Quick Start to its own "GPU Node Setup" section.

  **Monitoring**: Slim to just Nomad UI (`http://localhost:4646`) and the web UI
  (`http://localhost:8080`). Remove MQTT Explorer reference (not in the compose stacks).

  **Troubleshooting**: Keep GPU not detected, MQTT issues, Nomad issues. Fix MQTT topic
  in the `mosquitto_sub` example to use `compute/jobs/#`.

  **Remove entirely**: Production Deployment section (security/HA/monitoring checklist —
  too generic, not orchestrator-specific).

- **Test**: Read the file after writing. Grep for `gpu/jobs`, `postgres`, `ollama`,
  `fortran`, `patterns`, `llm`, `cmd/` — none should appear.
- **Notes**:
  - The web UI lives at `static/index.html` and serves at `http://localhost:8080/`.
    Mention it briefly under Monitoring as a basic job status dashboard.
  - `diffusion` / `semantic_diffusion` operations send raw CSV as the params payload;
    all others send JSON — this is a worker protocol detail, not needed in README.
  - Keep the License section.

## Completion Criteria
- [x] `ls *.md` at project root shows only `README.md` and `CLAUDE.md`
- [x] `grep -i "gpu/jobs\|postgres\|ollama\|fortran\|pkg/patterns\|cmd/test-patterns\|llm" README.md` returns nothing
- [x] README.md is under 200 lines (207 — within margin)
- [x] README.md accurately describes the current build/test/run workflow

## New Constraints or Gotchas
<!-- Nothing new for agents — this is purely doc changes -->
