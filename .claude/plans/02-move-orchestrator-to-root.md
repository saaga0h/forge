# Plan: Move Orchestrator to Root & Clean Nomad Artifacts
## Created: 2026-02-27
## Complexity: opus
## Recommended implementation model: haiku

## Context

After the previous cleanup (01-codebase-cleanup.md), the repo is orchestrator-only. The
Go code still lives under `go-orchestrator/` — a subdirectory that made sense when the
repo was a monolith with multiple language implementations. That structure is now legacy.

This plan moves the Go orchestrator to project root so the repo IS the orchestrator,
cleans the `nomad-server/` Dockerfiles (both still build a Fortran binary that no longer
exists), and replaces the stale Nomad job files (all still reference `fortran-worker`)
with a single generic dev job for local orchestrator testing.

**What changes:**
- `go-orchestrator/` contents → project root (flat Go project at root)
- `nomad-server/Dockerfile.ubuntu` → rewritten as clean Nomad dev image (no Fortran)
- `nomad-server/Dockerfile.dev` → deleted (broken duplicate of ubuntu variant)
- `nomad-jobs/*.nomad.hcl` (5 files, all Fortran-specific) → replaced by one generic dev job
- `docker-compose.yaml` → orchestrator build context updated
- `docker-compose.enhanced.yaml` → orchestrator build context + postgres migrations path + nomad image updated
- `CLAUDE.md` → updated build/run commands

**What stays put:**
- `nomad-server/client.hcl`, `nomad-dev.hcl`, `entrypoint.sh` — valid config, keep as-is
- `nomad-jobs/nomad-client-gpu.hcl` — GPU node client config, still relevant
- `nomad-jobs/test-dispatch.sh` — useful dev tool
- `nomad-jobs/setup-and-deploy.sh` — useful dev tool
- `configs/mosquitto.conf` — stays at root `configs/`
- Both docker-compose files — kept, just paths updated

**Go module note:** `go.mod` declares `module gpu-compute-orchestrator` which matches the
repo name. Moving to root does NOT change the module path — Go imports stay identical.

## Prerequisites
- [ ] `go-orchestrator/` currently builds: `cd go-orchestrator && go build ./...`
- [ ] No other branches / PRs depend on `go-orchestrator/` path

## Tasks

### 1. Move Go source files to project root
- **Files**: Everything under `go-orchestrator/` except configs (handled separately)
- **Action**: Move the following to project root:
  - `go-orchestrator/go.mod` → `go.mod`
  - `go-orchestrator/go.sum` → `go.sum`
  - `go-orchestrator/main.go` → `main.go`
  - `go-orchestrator/cmd/` → `cmd/`
  - `go-orchestrator/pkg/` → `pkg/`
  - `go-orchestrator/migrations/` → `migrations/`
  - `go-orchestrator/static/` → `static/`
  - `go-orchestrator/testdata/` → `testdata/`
  - `go-orchestrator/Makefile` → `Makefile`
  - `go-orchestrator/Dockerfile` → `Dockerfile`
  - `go-orchestrator/TESTING_GUIDE.md` → `TESTING_GUIDE.md`
- **Command**: Use `git mv` for each item so git tracks the renames properly
- **Test**: `ls go.mod main.go pkg/ cmd/ migrations/` all exist at root

### 2. Merge go-orchestrator/configs into root configs/
- **Files**: `go-orchestrator/configs/config.yaml`, `go-orchestrator/configs/llm.yaml`
- **Action**: Move both files into root `configs/` — they coexist with `configs/mosquitto.conf`
  without conflict (different services use them)
- **Command**: `git mv go-orchestrator/configs/config.yaml configs/config.yaml && git mv go-orchestrator/configs/llm.yaml configs/llm.yaml`
- **Test**: `ls configs/` shows `config.yaml`, `llm.yaml`, `mosquitto.conf`

### 3. Delete go-orchestrator/ directory
- **Files**: `go-orchestrator/` (should now be empty)
- **Action**: `rmdir go-orchestrator/` (not rm -rf, to catch any missed files)
- **Test**: `ls go-orchestrator/` returns "No such file or directory"

### 4. Update docker-compose.yaml orchestrator build context
- **File**: `docker-compose.yaml`
- **Action**: Change orchestrator service `build.context` from `./go-orchestrator` to `.`
- **Pattern**: Line ~53: `context: ./go-orchestrator` → `context: .`
- **Test**: `grep "context" docker-compose.yaml` shows `context: .`

### 5. Update docker-compose.enhanced.yaml paths and nomad image
- **File**: `docker-compose.enhanced.yaml`
- **Action** (three changes):
  1. Orchestrator build context: `context: ./go-orchestrator` → `context: .`
  2. Postgres migrations volume: `./go-orchestrator/migrations:/docker-entrypoint-initdb.d:ro`
     → `./migrations:/docker-entrypoint-initdb.d:ro`
  3. Orchestrator configs volume: `./go-orchestrator/configs:/app/configs:ro`
     → `./configs:/app/configs:ro`
  4. nomad-server service: replace `build: { context: ., dockerfile: nomad-server/Dockerfile.ubuntu }`
     with `build: { context: nomad-server, dockerfile: Dockerfile }`
     (pointing to the new clean Dockerfile written in task 6)
- **Test**: No `go-orchestrator` path references remain in the file

### 6. Rewrite nomad-server/Dockerfile.ubuntu → nomad-server/Dockerfile
- **File**: `nomad-server/Dockerfile.ubuntu` → rename to `nomad-server/Dockerfile`
- **Action**: Remove both the `fortran-builder` multi-stage build and all Fortran
  references. The result should be a clean Ubuntu-based Nomad dev image that:
  1. Copies the Nomad binary from the official image
  2. Copies `entrypoint.sh`, `client.hcl`, and `nomad-dev.hcl`
  3. Copies the dev job definition (from `nomad-jobs/gpu-compute-dev.nomad.hcl`,
     written in task 8) to `/etc/nomad.d/gpu-compute.nomad.hcl`
  4. Runs `entrypoint.sh` (which starts Nomad + auto-registers the dev job)

  New file content:
  ```dockerfile
  FROM ubuntu:22.04

  RUN apt-get update && apt-get install -y \
      ca-certificates \
      wget \
      && rm -rf /var/lib/apt/lists/*

  COPY --from=hashicorp/nomad:1.8 /bin/nomad /usr/local/bin/nomad

  COPY entrypoint.sh /usr/local/bin/entrypoint.sh
  COPY client.hcl /etc/nomad.d/client.hcl
  COPY nomad-dev.hcl /etc/nomad.d/nomad-dev.hcl

  # Job definition is injected at build time so the container is self-contained
  # for local dev; in production workers register their own jobs
  COPY gpu-compute-dev.nomad.hcl /etc/nomad.d/gpu-compute.nomad.hcl

  RUN chmod +x /usr/local/bin/nomad /usr/local/bin/entrypoint.sh && \
      groupadd nomad && \
      useradd -r -g nomad nomad && \
      mkdir -p /opt/nomad/data && \
      chown -R nomad:nomad /opt/nomad

  EXPOSE 4646 4647 4648

  ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
  ```

  Note: The Dockerfile copies `gpu-compute-dev.nomad.hcl` from the build context
  (nomad-server/), so that file must be present there. Copy it in task 8 or adjust
  the path. Alternatively the entrypoint.sh already handles a missing job file
  gracefully with a WARNING — so the COPY can be conditional / optional.

- **Test**: `docker build -f nomad-server/Dockerfile nomad-server/` completes without error
  (requires running Docker)

### 7. Delete nomad-server/Dockerfile.dev
- **File**: `nomad-server/Dockerfile.dev`
- **Action**: `git rm nomad-server/Dockerfile.dev`
- **Notes**: This was an Alpine variant of the same broken Fortran dev image. Now that
  we have a single clean `nomad-server/Dockerfile`, the `.dev` file is redundant.
- **Test**: File does not exist

### 8. Replace stale nomad-jobs with one generic dev job
- **Files to delete** (all Fortran-specific):
  - `nomad-jobs/gpu-compute.nomad.hcl` (production Singularity job)
  - `nomad-jobs/gpu-compute-dev.nomad.hcl` (Docker fortran image job)
  - `nomad-jobs/gpu-compute-local.nomad.hcl` (raw_exec fortran binary job)
  - `nomad-jobs/gpu-compute-rawexec.nomad.hcl` (rawexec + wrapper script job)
  - `nomad-jobs/gpu-compute-singularity.nomad.hcl` (Singularity variant)
- **File to create**: `nomad-jobs/gpu-compute-dev.nomad.hcl` (new, generic placeholder)

  New file content — parameterized batch job, raw_exec, shell sleep to simulate work,
  no Fortran references:
  ```hcl
  # gpu-compute-dev.nomad.hcl
  # Development placeholder job for testing orchestrator dispatch locally.
  # Real worker jobs live in compute worker repositories and are deployed separately.
  # This job accepts the same dispatch contract (job_id metadata, MQTT env vars)
  # but does nothing except sleep, simulating a worker that takes time to complete.

  job "gpu-compute" {
    type = "batch"

    parameterized {
      meta_required = ["job_id"]
      meta_optional = ["priority"]
    }

    datacenters = ["dc1"]

    group "compute" {
      count = 1

      restart {
        attempts = 0
        mode     = "fail"
      }

      reschedule {
        attempts  = 0
        unlimited = false
      }

      task "compute-worker" {
        driver = "raw_exec"

        env {
          JOB_ID      = "${NOMAD_META_job_id}"
          MQTT_BROKER = "tcp://mqtt:1883"
        }

        config {
          command = "/bin/sh"
          args    = ["-c", "echo \"[dev-worker] job=$JOB_ID started\" && sleep 10 && echo \"[dev-worker] job=$JOB_ID done\""]
        }

        resources {
          cpu    = 100
          memory = 64
        }

        logs {
          max_files     = 3
          max_file_size = 5
        }

        kill_timeout = "5s"
      }
    }
  }
  ```

- **Test**: Only `gpu-compute-dev.nomad.hcl`, `nomad-client-gpu.hcl`,
  `test-dispatch.sh`, `setup-and-deploy.sh` remain in `nomad-jobs/`

### 9. Copy dev job into nomad-server/ for Dockerfile COPY step
- **File**: `nomad-server/gpu-compute-dev.nomad.hcl`
- **Action**: Copy (or symlink) the dev job into the `nomad-server/` directory so the
  Dockerfile COPY step in task 6 can include it in the image build context.
  Use `cp nomad-jobs/gpu-compute-dev.nomad.hcl nomad-server/gpu-compute-dev.nomad.hcl`
  and commit both files (they should be identical; source of truth is `nomad-jobs/`).
- **Alternative**: Skip the COPY in the Dockerfile and let entrypoint.sh handle the
  missing job file gracefully (it already emits a WARNING). In that case, the job
  must be registered manually after the container starts. Choose based on preference.
- **Test**: File exists at `nomad-server/gpu-compute-dev.nomad.hcl` OR Dockerfile
  does not reference it (if alternative chosen)

### 10. Verify Go build still works
- **File**: Root `go.mod`, `main.go`
- **Action**: Run `go build ./...` and `go test -v -race ./...` from project root
- **Notes**: Go module name stays `gpu-compute-orchestrator` — no import changes needed.
  If the build was passing before the move, it should pass after (pure filesystem change).
- **Test**: `go build ./...` exits 0; `go test ./...` exits 0

### 11. Update CLAUDE.md
- **File**: `CLAUDE.md`
- **Action**: Remove `cd go-orchestrator` prefix from all build commands since Go code
  now lives at root. Update the Dev Stack section to note that `docker-compose.enhanced.yaml`
  now uses `nomad-server/Dockerfile` (clean Nomad dev image, no Fortran).
  Update the file structure context if present.

  Specific change to Build & Test section:
  ```markdown
  ## Build & Test
  make build          # → bin/orchestrator
  make test           # go test -v -race ./...
  make fmt            # go fmt + go vet
  make lint           # golangci-lint
  ```
  (Remove the `cd go-orchestrator` line; everything runs from repo root now)
- **Test**: CLAUDE.md build commands work from project root without cd

## Completion Criteria
- [x] `go.mod`, `main.go`, `pkg/`, `cmd/`, `Makefile`, `Dockerfile` all exist at project root
- [x] `go-orchestrator/` directory does not exist
- [x] `configs/` contains `config.yaml`, `llm.yaml`, and `mosquitto.conf`
- [x] `go build ./...` passes from project root
- [x] `go test -v -race ./...` passes from project root
- [x] No `go-orchestrator` path references in either docker-compose file
- [x] `nomad-server/Dockerfile` exists with no Fortran build stages; `Dockerfile.dev` is deleted
- [x] Only generic `gpu-compute-dev.nomad.hcl` (new) and `nomad-client-gpu.hcl` remain as HCL files in `nomad-jobs/`
- [x] `grep -r "fortran" nomad-server/ nomad-jobs/` returns nothing
- [x] CLAUDE.md build commands work without `cd go-orchestrator`

## New Constraints or Gotchas
- The `nomad-server/Dockerfile` COPY step for the dev job file requires that
  `gpu-compute-dev.nomad.hcl` is in the `nomad-server/` build context directory.
  If you change `docker-compose.enhanced.yaml` to use `context: .` instead, adjust
  the COPY path to `nomad-jobs/gpu-compute-dev.nomad.hcl`.
- The `entrypoint.sh` registers a job named `gpu-compute` from `/etc/nomad.d/gpu-compute.nomad.hcl`.
  This works only in the enhanced stack; the basic `docker-compose.yaml` uses the
  official `hashicorp/nomad:1.8` image in pure dev mode with no pre-registered jobs.
- After moving to root, Docker builds use `context: .` — make sure `.gitignore` /
  `.dockerignore` exclude large directories (testdata, bin/) if present.
