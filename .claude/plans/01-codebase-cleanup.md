# Plan: Codebase Cleanup — Orchestration Focus
## Created: 2026-02-22
## Complexity: opus
## Recommended implementation model: haiku

## Context

This project started as a proof-of-concept platform called FORGE that explored
GPU-accelerated computation using Fortran (with HIP/ROCm) and later Julia for
semantic N-body simulations. Both compute implementations have now been superseded:
the Julia code has been moved to a separate repository, and the Fortran code was
always experimental scaffolding.

The remaining value is in the **orchestration layer**: the Go service that accepts
jobs via HTTP, dispatches them to Nomad, communicates via MQTT, and aggregates
semantic patterns with LLM assistance. That is what this repository should be.

This plan removes all proof-of-concept compute implementations, their test
infrastructure, and associated stale documentation, leaving a clean
orchestration-focused codebase.

**What we are keeping:**
- `go-orchestrator/` — Go HTTP API, MQTT client, Nomad dispatcher, pattern aggregator
- `nomad-server/` — Nomad server configuration for local dev
- `nomad-jobs/` (root level) — Nomad job definitions
- `configs/` — Mosquitto MQTT config
- `docker-compose.yaml` — local dev stack (clean, no Fortran references)
- Docs relevant to orchestration: MQTT, Nomad, Docker Compose, Web UI, Mac dev setup

**What we are removing:**
- All Fortran code (two locations)
- All Julia code (moved to another repo)
- The Rust worker that wrapped the Fortran kernel
- Python benchmarks for old compute
- Stale Go CLI commands tied to removed compute
- Root-level test data, results, and analysis scripts for old compute
- Stale documentation about old compute implementations
- The `/compute/` directory entirely (after removing its contents)

## Prerequisites
- [ ] No running processes depend on files being deleted
- [ ] Git status is clean or changes are intentional (current untracked files are
      mostly in directories being removed anyway)

## Tasks

### 1. Remove /fortran-compute/ directory
- **File**: `/fortran-compute/` (entire directory)
- **Action**: `rm -rf fortran-compute/`
- **Notes**: Contains 30+ Fortran source files, 6 Makefiles, 5 Dockerfiles, 2
  Singularity defs, build artifacts, test scripts. Entire directory is legacy POC.
- **Test**: `ls fortran-compute/` should return "No such file or directory"

### 2. Remove /compute/semantic-n-body/ directory
- **File**: `/compute/semantic-n-body/` (entire directory)
- **Action**: `rm -rf compute/semantic-n-body/`
- **Notes**: Julia implementation already moved to another repository. Includes
  src/, test/, deploy/ (Packer + Nomad), Dockerfile, Singularity def, Project.toml.
  The untracked files in git status (.gitignore, README.md, LANGUAGE_REWRITE_ANALYSIS.md,
  deploy/, test_data/, test_local.jl) are all in this directory.
- **Test**: `ls compute/semantic-n-body/` should fail

### 3. Remove /compute/diffusion-nbody/ directory
- **File**: `/compute/diffusion-nbody/` (entire directory)
- **Action**: `rm -rf compute/diffusion-nbody/`
- **Notes**: Contains Fortran kernel (fortran-kernel/src/*.f90, nbody_kernel.hip),
  Rust worker (rust-worker/src/*.rs), Singularity defs, test scripts, test job JSON
  files. The rust-worker exists solely to orchestrate the Fortran kernel; both go together.
  Many untracked files in this dir (mosquitto.db, singularity-old.def,
  LANGUAGE_REWRITE_ANALYSIS.md, test_cpu_worker.sh, test_gpu_worker_old.sh).
- **Test**: `ls compute/diffusion-nbody/` should fail

### 4. Remove /compute/packer/ and /compute/nomad-jobs/
- **File**: `/compute/packer/`, `/compute/nomad-jobs/`
- **Action**: `rm -rf compute/packer/ compute/nomad-jobs/`
- **Notes**: Packer image builder configs and Nomad job defs for old workers.
  The root-level `/nomad-jobs/` (separate directory) is the canonical location and stays.
- **Test**: Both directories gone; `ls compute/` shows only DEPLOYMENT.md

### 5. Remove the /compute/ directory
- **File**: `/compute/` (entire directory after tasks 2-4)
- **Action**: `rm -rf compute/`
- **Notes**: After removing semantic-n-body, diffusion-nbody, packer, and nomad-jobs,
  only `compute/DEPLOYMENT.md` remains. The DEPLOYMENT.md content should be checked
  first — if it contains relevant orchestration deployment info not covered elsewhere,
  merge it into the root README before deleting. Otherwise delete outright.
  Current git status shows `compute/DEPLOYMENT.md` is modified (M) — read it first.
- **Test**: `ls compute/` should fail

### 6. Remove /python-benchmark/ directory
- **File**: `/python-benchmark/` (entire directory)
- **Action**: `rm -rf python-benchmark/`
- **Notes**: Python benchmarks comparing Python vs Fortran compute. No longer relevant.
  Contains benchmark_python.py, Dockerfile, docker-compose.yml, requirements.txt.
- **Test**: `ls python-benchmark/` should fail

### 7. Remove stale Go CLI commands from go-orchestrator
- **File**: `go-orchestrator/cmd/convert-fortran/main.go`,
  `go-orchestrator/cmd/test-diffusion/main.go`
- **Action**: `rm -rf go-orchestrator/cmd/convert-fortran/ go-orchestrator/cmd/test-diffusion/`
- **Notes**: `convert-fortran` converts Fortran binary format data — no Fortran, no need.
  `test-diffusion` tests the diffusion pipeline — that pipeline is gone.
  `cmd/test-patterns/main.go` tests the LLM pattern aggregator — KEEP that one.
- **Test**: Only `cmd/test-patterns/` remains under `go-orchestrator/cmd/`

### 8. Check go-orchestrator for remaining Fortran references
- **File**: `go-orchestrator/` (read and grep)
- **Action**: Search for any remaining references to Fortran, diffusion, or removed paths
  in Go source files. Update or remove as needed.
- **Pattern**: `grep -r "fortran\|diffusion\|convert-fortran" go-orchestrator/`
- **Notes**: The `pkg/compute/manager.go` and `pkg/compute/types.go` may reference
  Fortran job types. Update to be generic/compute-agnostic if needed.
  The Makefile may have Fortran build targets — remove them.
- **Test**: `grep -ri "fortran" go-orchestrator/` returns nothing meaningful

### 9. Remove stale Python analysis scripts from root
- **Files**: `analyze_chains.py`, `analyze_chains_detailed.py`, `check_chains.py`,
  `generate_realistic_anchors.py`
- **Action**: `rm analyze_chains.py analyze_chains_detailed.py check_chains.py generate_realistic_anchors.py`
- **Notes**: These scripts analyzed chain data from the old compute pipeline.
  The anchor-related ones generated test data for Fortran/diffusion jobs.
- **Test**: Files gone from root

### 10. Remove stale data and result files from root
- **Files**: `anchor-export.csv`, `old-export.csv`, `orinal-anchor-export.csv`,
  `test-anchors.csv`, `test-real-format.csv`, `week-synthetic.csv`,
  `result_cpu_pure.json`, `result_gpu_fallback.json`, `temp_job.json`,
  `test_job.json`, `semantic_diffusion_mod.mod`
- **Action**: Delete all of the above
- **Notes**: These are test inputs and outputs for the old Fortran/diffusion compute.
  `semantic_diffusion_mod.mod` is a compiled Fortran module artifact accidentally
  committed or left at root level.
- **Test**: Files gone from root

### 11. Remove stale scripts from root
- **Files**: `start-worker.sh`, `test_large_job.sh`
- **Action**: `rm start-worker.sh test_large_job.sh`
- **Notes**: `start-worker.sh` starts the old Fortran worker. `test_large_job.sh`
  submits large jobs to the old compute pipeline.
- **Test**: Files gone

### 12. Remove stale misc files from root
- **Files**: `name.md`, `answer_chain_question.md`, `test_progressive_learning.yaml`
- **Action**: Delete all of the above
- **Notes**: Unclear provenance, likely scratch notes or test configs for old compute.
- **Test**: Files gone

### 13. Remove stale documentation files from root
- **Files** (Fortran/diffusion/Julia specific):
  - `CHAIN_ANALYSIS.md` — analysis of semantic chain behavior in diffusion sim
  - `E2E_PIPELINE_BUG_ANALYSIS.md` — E2E bugs in old pipeline
  - `E2E_REAL_ISSUE.md` — E2E issue with old pipeline
  - `IMPLEMENTATION_SUMMARY.md` — summary of old Fortran+Go implementation
  - `LLM_TRANSCODING_DESIGN.md` — design doc for LLM-based Fortran transcoding
  - `PHYSICS_MODEL_ANALYSIS.md` — physics analysis for n-body diffusion
  - `PROJECT_COMPLETION_REPORT.md` — completion report for old FORGE POC
  - `QUICKSTART_DIFFUSION.md` — diffusion worker quickstart
  - `SEMANTIC_DIFFUSION_README.md` — diffusion simulation readme
  - `SUMMARY.md` — summary of old implementation
  - `TEMPORAL_GATING_SUCCESS.md` — temporal gating in diffusion sim
  - `TUNING_QUICK_REFERENCE.md` — tuning guide for diffusion physics
  - `VALIDATION_REPORT.md` — validation of old compute results
  - `WEIGHT_TUNING_RESULTS.md` — weight tuning for diffusion physics
  - `DOCUMENTATION_INDEX.md` — index of now-removed docs
- **Action**: Delete all of the above
- **Notes**: Read each briefly before deleting to confirm no orchestration-relevant
  content is buried inside. If any contains useful orchestration info, extract it
  first into the README.
- **Test**: Only orchestration-relevant docs remain at root

### 14. Review and possibly remove docker-compose.enhanced.yaml
- **File**: `docker-compose.enhanced.yaml`
- **Action**: Read the file. If it adds meaningful value over docker-compose.yaml
  (e.g., additional monitoring, logging), keep it. If it's a duplicate or references
  removed services, delete it.
- **Test**: Deliberate decision made and documented

### 15. Update README.md
- **File**: `README.md`
- **Action**: Rewrite to reflect the current orchestration-only architecture:
  - Update the architecture diagram — remove Fortran/Singularity/ROCm from the diagram
  - Update Components section — remove "Fortran Compute Worker", reflect that workers
    are generic Nomad-dispatched containers
  - Update Quick Start — remove "Build Fortran Worker Container" step
  - Update Configuration — remove Fortran worker environment variables
  - Keep: API endpoints, MQTT topics, Nomad setup, monitoring, troubleshooting
- **Pattern**: Current README structure is good; surgical edits rather than full rewrite
- **Test**: README accurately describes the current codebase with no Fortran references

### 16. Create CLAUDE.md
- **File**: `CLAUDE.md` (new file at root)
- **Action**: Create project memory file capturing the current architecture and decisions
- **Content should include**:
  - What this repo is: Go orchestrator for GPU compute via Nomad + MQTT
  - Key components and their locations
  - What was removed and why (FORGE POC cleanup)
  - Where the Julia compute code now lives (mention it moved to another repo)
  - Go version, how to run tests, how to build
  - MQTT topic conventions
  - Nomad job structure
- **Test**: CLAUDE.md exists and covers key architectural decisions

## Completion Criteria
- [x] `fortran-compute/` is gone
- [x] `compute/` directory is entirely gone
- [x] `python-benchmark/` is gone
- [x] `go-orchestrator/cmd/convert-fortran/` and `cmd/test-diffusion/` are gone
- [x] No `*.f90`, `*.mod`, `*.f`, `*.for` files exist anywhere in the repo
- [x] No `*.jl` files exist anywhere in the repo
- [x] No Rust files from the diffusion worker exist
- [x] Root directory contains only: go-orchestrator/, nomad-server/, nomad-jobs/,
      configs/, docker-compose.yaml, README.md, CLAUDE.md, and relevant docs
- [x] `grep -ri "fortran" .` in go-orchestrator/ returns no source code references
- [x] `go build ./...` still passes in go-orchestrator/
- [x] README.md accurately reflects the remaining codebase

## Context Updates
When implementation is complete, add to CLAUDE.md:
- This was a POC platform (FORGE) that evolved into a pure orchestrator
- Julia compute was moved to a separate repository (not deleted — it lives elsewhere)
- Fortran/Rust/Python compute code was proof-of-concept and removed
- The Go orchestrator is the primary artifact: HTTP API + MQTT + Nomad dispatching
  + LLM-assisted pattern aggregation
- The `pkg/patterns/` package is the most sophisticated part: aggregates semantic
  patterns from compute results using LLM
