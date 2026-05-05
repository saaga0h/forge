# Building a FORGE Worker — Lessons and Landmines

> _Everything that went wrong (and the fixes) across two GPU compute workers:
> semantic-n-body (Julia) and grassmann-distance (Julia)._

**As of**: May 2026

---

## Table of Contents

1. [The Contract](#1-the-contract)
2. [Singularity Containers](#2-singularity-containers)
3. [MQTT and libmosquitto](#3-mqtt-and-libmosquitto)
4. [Nomad Job Definitions](#4-nomad-job-definitions)
5. [GPU Access and ROCm](#5-gpu-access-and-rocm)
6. [Julia-Specific](#6-julia-specific)
7. [Gitea CI Pipeline](#7-gitea-ci-pipeline)
8. [Integration Testing](#8-integration-testing)

---

## 1. The Contract

### Request Envelope

Clients publish to `compute/request/{client_id}/{correlation_id}`:

```json
{
  "operation": "your-nomad-job-name",
  "payload": {
    "your": "job-specific params"
  }
}
```

- `operation` maps **directly** to the Nomad job name. No validation — Nomad is the
  source of truth. If the job doesn't exist, Nomad returns an error and Forge surfaces it.
- `payload` is an arbitrary JSON object. Forge does not inspect it.

### What the Worker Receives

Forge flattens the payload and injects `job_id`, then publishes as a **retained** message
to `compute/jobs/{job_id}/params`:

```json
{
  "job_id": "forge-generated-uuid",
  "your": "job-specific params"
}
```

The worker never sees the `operation` field or the client identity.

### What the Worker Publishes

Result to `compute/jobs/{job_id}/result` (QoS 1):

```json
{
  "job_id": "...",
  "success": true,
  "result": { ... },
  "worker_id": "nomad-<alloc-id>",
  "timestamp": "2026-05-05T12:00:00.000Z"
}
```

Optional status updates to `compute/jobs/{job_id}/status` (QoS 0):
`"processing"`, `"completed"`, `"error"`.

Optional logs to `compute/jobs/{job_id}/logs` (QoS 0).

### Timing Invariant

Forge publishes retained params **before** dispatching the Nomad job. The worker must
subscribe to the params topic immediately on startup — the retained message will be
waiting. Do not add delays or health checks before subscribing.

---

## 2. Singularity Containers

### Build on Local Disk, Not NFS

NFS extended attributes break Singularity's fakeroot unpacking. Always build on local disk:

```bash
BUILD_TMPDIR="/var/tmp/singularity-build-$$"
mkdir -p "$BUILD_TMPDIR"
SINGULARITY_TMPDIR="$BUILD_TMPDIR" singularity build --fakeroot worker.sif singularity.def
rm -rf "$BUILD_TMPDIR"
```

Then copy the `.sif` to NFS for Nomad to read.

### Singularity Path

The `singularity` binary lives at different paths depending on the host. Check the actual
location on your build and runtime nodes — `/usr/local/bin/singularity` vs `/usr/bin/singularity`.
The Nomad job `config.command` must match the runtime node's path.

### Container Validation

Run tests inside the container during build (`%post`). A container that builds but fails
at runtime wastes a full rebuild cycle (which can be 10+ minutes). Catch it early:

```
%post
    julia --project=. test/runtests.jl
    julia --project=. -e 'using YourPackage; println("Package load OK")'
```

---

## 3. MQTT and libmosquitto

### Build-Time and Runtime

libmosquitto must be present at both stages:

- **Build time**: `libmosquitto-dev` in `%post` for the Julia/Python package to compile
  against.
- **Runtime**: The shared library must be findable. In Singularity, libraries installed
  during `%post` are baked into the image — this works. But if you bind-mount a different
  root at runtime, verify `libmosquitto.so` is still on the library path.

### Conditional Loading

If your language supports optional dependencies (Julia does), load MQTT conditionally so
the package works in dev/test without a broker:

```julia
const HAS_MOSQUITTO = try
    @eval using Mosquitto
    true
catch e
    @warn "Mosquitto not available" reason=sprint(showerror, e)
    false
end
```

**Critical**: if any functions are defined inside the conditionally-loaded file (e.g.
`julia_main()`), provide a fallback definition in the else branch. Otherwise the function
simply doesn't exist and the call site gets `UndefVarError` with no useful context.

---

## 4. Nomad Job Definitions

### Use Meta Constraints, Not Device Blocks

Nomad's `device "amd/gpu"` resource block requires a working device plugin that
fingerprints the GPU hardware. If the plugin isn't running or doesn't detect the device,
placement silently fails with "missing devices filtered N nodes."

Meta constraints are more reliable for pinning to GPU nodes:

```hcl
constraint {
    attribute = "${meta.gpu}"
    value     = "true"
}

constraint {
    attribute = "${meta.rocm}"
    value     = "true"
}
```

These require the Nomad client config on the GPU node to have:

```hcl
client {
    meta {
        gpu  = "true"
        rocm = "true"
    }
}
```

The `device` block is useful when you have multiple GPUs and need Nomad to allocate
specific ones. For a single-GPU node where the worker gets the whole GPU, meta constraints
are sufficient and don't depend on device plugin state.

### Parameterized Batch Jobs

Every FORGE worker is a parameterized batch job. One dispatch = one job = exits.

```hcl
type = "batch"

parameterized {
    meta_required = ["job_id"]
}
```

`job_id` flows from Forge dispatch metadata to `NOMAD_META_job_id` environment variable.
The worker reads this to know which MQTT topic to subscribe to.

### No Restarts, No Rescheduling

Workers are one-shot. If they fail, the failure is reported via MQTT. Restarts would
re-process the same job or race with a replacement dispatch.

```hcl
restart {
    attempts = 0
    mode     = "fail"
}

reschedule {
    attempts  = 0
    unlimited = false
}
```

---

## 5. GPU Access and ROCm

### Bind Mount, Not Device Plugin

For `raw_exec` + Singularity, ROCm access is through bind mounts:

```hcl
config {
    command = "/usr/bin/singularity"
    args = [
        "run",
        "--bind", "/opt/rocm:/opt/rocm",
        "/nfs/images/your-worker/worker.sif",
    ]
}
```

The container needs ROCm runtime libraries but not the full ROCm install. In the
Singularity def:

```
%post
    apt-get install -y --no-install-recommends \
        libxml2 libelf1 libdrm2 libdrm-amdgpu1 libnuma1
```

The actual ROCm drivers and runtime come from the bind mount.

### Sysimage and AMDGPU (Julia-Specific)

Julia's PackageCompiler uses its bundled LLVM to compile sysimages. AMDGPU.jl injects
AMD GCN bitcode into the compilation pipeline. Julia's generic LLVM cannot handle GCN
intrinsics and aborts:

```
LLVM ERROR: Broken module found
```

This happens even when AMDGPU isn't in the sysimage package list — `using YourPackage`
transitively loads it.

**Workaround**: skip sysimage during container build. Build it separately on the GPU node
where ROCm's LLVM is present:

```bash
singularity exec --bind /opt/rocm:/opt/rocm worker.sif \
    julia --project=/app -e '
        using PackageCompiler
        create_sysimage(:YourPackage;
            sysimage_path="/nfs/images/your-worker/sysimage.so",
            precompile_execution_file="/app/test/runtests.jl")'
```

The container runscript checks for an external sysimage and falls back gracefully:

```bash
SYSIMAGE="${SYSIMAGE_PATH:-/app/sysimage.so}"
if [ -f "$SYSIMAGE" ]; then
    SYSIMAGE_FLAG="--sysimage=$SYSIMAGE"
fi
```

Without a sysimage, expect ~30s JIT compilation on cold start. Acceptable for batch jobs.

---

## 6. Julia-Specific

### Conditional Dependencies

Julia 1.12+ requires explicit dependency entries in Project.toml for all packages,
including stdlibs like `Serialization`, `Base64`, `Dates`. If you get
"Package X does not have Y in its dependencies", add it with `Pkg.add("Y")`.

### Conditional GPU Loading

AMDGPU must be in Project.toml even if only conditionally loaded. Without it, `@eval using
AMDGPU` fails at precompile time with a dependency error, not a missing-package error:

```
ArgumentError: Package YourPackage does not have AMDGPU in its dependencies
```

Declare it as a dependency, load conditionally, provide CPU fallback:

```julia
# In module definition
function select_backend(use_gpu::Bool)
    if use_gpu
        @warn "GPU requested but AMDGPU not available, falling back to CPU"
    end
    return CPU()
end

const HAS_AMDGPU = try
    @eval using AMDGPU
    true
catch e
    @warn "AMDGPU not available" reason=sprint(showerror, e)
    false
end

if HAS_AMDGPU
    include("gpu.jl")  # Redefines select_backend with GPU support
end
```

### Manifest.toml

Commit `Manifest.toml`. Singularity builds run `Pkg.instantiate()` which needs the
lockfile for reproducible builds. If it's in `.gitignore`, the container build fails with
missing dependencies.

---

## 7. Gitea CI Pipeline

### Build → Publish → Register

Standard pipeline for FORGE workers:

1. Build Singularity image on the packer runner (local disk)
2. Copy `.sif` to `/nfs/images/{worker-name}/` with a manifest
3. Register Nomad job with `nomad job run`

The packer runner needs: `singularity` on PATH, sudo/fakeroot, NFS mounted.

### Nomad Address

`NOMAD_ADDR` must be set as a Gitea repository variable. The CI step that runs
`nomad job run` needs it. Easy to forget, silent failure.

---

## 8. Integration Testing

### Test as a Client, Not as Forge

The integration test is a FORGE client. It publishes to
`compute/request/{client_id}/{correlation_id}` and waits for the response on
`compute/response/{client_id}/{correlation_id}`. It does **not**:

- Dispatch Nomad jobs (Forge does that)
- Publish to `compute/jobs/*/params` (Forge does that)
- Know the job_id (Forge generates it)

### Timeouts

Workers without a sysimage JIT-compile on cold start. Set integration test timeouts to
180s minimum. The first job will be slow; subsequent dispatches within the same Nomad
allocation are faster (if the worker stays warm, which batch jobs don't — each dispatch
is a fresh container).

### Credentials

Store MQTT credentials in `.env` (gitignored). Only `MQTT_BROKER`, `MQTT_USER`, and
`MQTT_PASSWORD` are needed — the test client doesn't talk to Nomad or Vault.
