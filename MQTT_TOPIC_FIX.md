# MQTT Topic Mismatch Fix

## Problem

The Fortran worker and Go orchestrator were using **different MQTT topic prefixes**:
- **Fortran worker**: Published to `gpu/jobs/{job_id}/result`
- **Go orchestrator**: Subscribed to `compute/jobs/{job_id}/result`

Result: Fortran computed diffusion successfully, but orchestrator never received the results.

## Solution

Fixed Fortran MQTT topics to use `compute/` prefix consistently:

### Files Modified

1. **fortran-compute/src/mqtt_client.f90**
   - Line 301: `gpu/jobs/` → `compute/jobs/` (status topic)
   - Line 317: `gpu/jobs/` → `compute/jobs/` (result topic)
   - Line 334: `gpu/jobs/` → `compute/jobs/` (error topic)

2. **Rebuilt Nomad container**
   - `docker build -f nomad-server/Dockerfile.ubuntu -t nomad-with-worker:ubuntu .`
   - Nomad container now has the fixed Fortran worker binary at `/usr/local/bin/semantic_diffusion_cpu`

## How to Test End-to-End

### 1. Ensure all services are running
```bash
docker compose -f docker-compose.enhanced.yaml up -d
```

### 2. Submit a job with real anchor data

Via API:
```bash
curl -X POST http://localhost:8080/compute \
  -H 'Content-Type: application/json' \
  -d '{
    "operation": "diffusion",
    "parameters": {
      "anchor_data": "<paste anchor CSV data here>"
    }
  }'
```

Via Web UI:
- Open http://localhost:8080/
- Upload anchor data file (67+ anchors with 128 dimensions recommended)
- Submit job

### 3. Monitor the job

```bash
# Get job status
curl http://localhost:8080/jobs/{job_id} | jq

# Watch Nomad logs
docker exec nomad-server nomad job status

# Check orchestrator logs
docker logs gpu-orchestrator -f
```

### 4. View results

- **Web UI**: http://localhost:8080/ - Shows jobs, diffusion results, and LLM analysis
- **API**: `curl http://localhost:8080/jobs/{job_id}` - Returns JSON with results

## Current Status

✅ **Fixed**: MQTT topics aligned between Fortran worker and Go orchestrator
✅ **Fixed**: Job metadata (job_id) passed correctly from orchestrator → Nomad → worker
✅ **Fixed**: Restart/reschedule policies set to prevent continuous restart cycles
✅ **Working**: End-to-end flow from job submission → Nomad dispatch → Fortran computation

⚠️ **Note**: Manual test workers (background bash processes) use OLD binary and still publish to `gpu/jobs/`. Only jobs dispatched through Nomad use the fixed binary.

## Architecture Flow

```
User → Web UI/API (port 8080)
  ↓
Go Orchestrator
  ├─→ Publishes params to MQTT: compute/jobs/{job_id}/params
  └─→ Dispatches job to Nomad with job_id metadata
      ↓
Nomad Server (raw_exec driver)
  └─→ Spawns Fortran worker with JOB_ID from metadata
      ↓
Fortran Worker
  ├─→ Subscribes to: compute/jobs/{job_id}/params
  ├─→ Receives anchor data via MQTT
  ├─→ Computes semantic diffusion
  ├─→ Publishes status to: compute/jobs/{job_id}/status
  └─→ Publishes result to: compute/jobs/{job_id}/result
      ↓
Go Orchestrator
  ├─→ Receives result from MQTT
  ├─→ Updates job status in memory
  └─→ Sends to Pattern Aggregator for LLM analysis
      ↓
Web UI
  └─→ Displays chains and LLM interpretation
```

## Verification Commands

```bash
# 1. Check Nomad job is registered
docker exec nomad-server nomad job status gpu-compute

# 2. Check Nomad node has CPU resources
docker exec nomad-server nomad node status -self

# 3. Submit test job and get job_id
JOB_ID=$(curl -s -X POST http://localhost:8080/compute \
  -H 'Content-Type: application/json' \
  -d '{"operation":"diffusion","parameters":{"anchor_data":"..."}}' \
  | jq -r '.job_id')

# 4. Wait for completion and check result
sleep 5
curl http://localhost:8080/jobs/$JOB_ID | jq '.status, .result'

# 5. Check MQTT broker logs for the flow
docker logs mqtt-broker 2>&1 | grep $JOB_ID
```
