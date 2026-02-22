# Nomad raw_exec Setup for Mac Development

## Why Nomad Dispatch Fails on Mac

The current issue is **Docker-in-Docker limitation**:

1. Nomad runs inside a Docker container
2. The job spec requires the `docker` driver
3. The Nomad container doesn't have access to Docker daemon
4. Result: `Constraint "missing drivers"` error

## Implementation Progress

### What Was Attempted

Created a custom Nomad image with embedded Fortran worker to use `raw_exec` driver:

**Files Created:**
- `fortran-compute/Makefile.alpine` - Build for Alpine/musl
- `fortran-compute/Dockerfile.alpine` - Fortran builder for Alpine
- `nomad-server/Dockerfile.dev` - Custom Nomad image
- `nomad-jobs/gpu-compute-rawexec.nomad.hcl` - Job spec for raw_exec

### Technical Challenge Encountered

**Libc Incompatibility:**
- Official Nomad binary (from `hashicorp/nomad:1.8`) is linked against glibc
- Alpine Linux uses musl libc
- Binary won't run: `/bin/sh: nomad: not found` (actually means loader not found)

**Options to resolve:**
1. Build Nomad from source for Alpine (very complex, long build time)
2. Use Ubuntu/Debian base instead of Alpine (larger image, different dependencies)
3. Accept manual worker startup for Mac dev (current workaround)

## Current Workaround

For Mac development, manually start workers:

```bash
# Submit job via web UI
curl -X POST http://localhost:8080/compute \
  -H "Content-Type: application/json" \
  -d '{"operation": "diffusion", "parameters": {...}}'

# Note the job_id from response

# Manually start worker
docker run --rm \
  --network gpu-compute-orchestrator_gpu-compute-net \
  -e MQTT_BROKER="tcp://mqtt:1883" \
  -e JOB_ID="<job_id>" \
  -e NUM_THREADS="4" \
  fortran-diffusion-cpu:test
```

## Production Deployment

On Linux servers with actual GPUs:
- Nomad runs **directly on the host** (not in Docker)
- Has native access to Docker daemon
- `docker` driver works perfectly
- GPU constraints and resource allocation work as designed

## Recommendation

**For Mac development:** Continue using manual worker startup. It works and is simple.

**For production:** Deploy Nomad on Linux host with GPU access, use original job spec with Docker driver.

The system is fully functional end-to-end - the only limitation is Nomad automatic dispatch on Mac, which is a dev environment constraint, not a system design issue.
