# Web UI Guide - GPU Compute Orchestrator

**URL**: http://localhost:8080/

---

## Features

### 🎨 Modern Dark Theme Interface
- Clean, professional dark theme design
- Responsive layout that works on all devices
- Real-time status updates
- Drag-and-drop file upload

### 📤 File Upload
- **Drag & Drop**: Drop CSV files directly onto the upload area
- **Click to Browse**: Traditional file picker
- **File Validation**: Only accepts .csv files
- **Size Limit**: Up to 10MB (configured in mosquitto.conf)

### 📊 Job Management
- **Auto-refresh**: Jobs update every 2 seconds while active
- **Status Indicators**:
  - 🟠 `pending` - Job queued
  - 🔵 `dispatched` - Sent to Nomad
  - 🟢 `computing` - Worker processing
  - ✅ `completed` - Finished successfully
  - 🔴 `failed` - Error occurred

### ✨ Results Display
- **Pattern Cards**: Each detected pattern shown in a card
- **Pattern Details**:
  - Human-readable name (from LLM transcoding)
  - Natural language description
  - Confidence level
  - Anchor count
  - Time of day
  - Chain ID
- **Statistics**: Key metrics for each pattern

---

## How to Use

### 1. Start the Stack

```bash
docker-compose -f docker-compose.enhanced.yaml up -d
```

**Services**:
- ✅ Orchestrator (with Web UI): http://localhost:8080
- ✅ PostgreSQL: localhost:5432
- ✅ MQTT: localhost:1883
- ✅ Nomad: http://localhost:4646
- ✅ Consul: http://localhost:8500

### 2. Access the Web UI

Open http://localhost:8080/ in your browser

### 3. Upload Anchor CSV

**CSV Format**:
```csv
# anchor_id,timestamp,location,dim1,dim2,...,dim128
anchor-001,1699564800,bedroom,0.1,0.2,0.3,...
anchor-002,1699565100,kitchen,0.15,0.25,0.35,...
```

**Steps**:
1. Click the upload area or drag & drop your CSV file
2. File name and size will be displayed
3. Click **"Submit Job"**
4. Job will appear in the Active Jobs list

### 4. Monitor Progress

- Jobs auto-refresh every 2 seconds
- Status changes from `pending` → `dispatched` → `computing` → `completed`
- Click on any job to view detailed information

### 5. View Results

Once a job completes:
- Results section automatically appears
- Each pattern displayed as a card with:
  - **Name**: "Morning Routine - Breakfast"
  - **Description**: Natural language explanation
  - **Statistics**: Confidence, anchor count, time of day, chain ID
  - **Intent**: Inferred purpose from LLM

---

## API Endpoints

The same endpoints work via API:

### Submit Job
```bash
curl -X POST http://localhost:8080/compute \
  -H "Content-Type: application/json" \
  -d '{
    "operation": "diffusion",
    "parameters": {
      "anchor_data": "anchor-001,1699564800,bedroom,0.1,0.2,...",
      "iterations": 500,
      "convergence_threshold": 0.0001
    },
    "priority": "normal"
  }'
```

### List All Jobs
```bash
curl http://localhost:8080/jobs
```

### Get Job Details
```bash
curl http://localhost:8080/jobs/{job_id}
```

### Health Check
```bash
curl http://localhost:8080/health
```

---

## Architecture Flow

```
┌─────────────┐
│  Web UI     │ Upload CSV file (1MB of data)
│  Browser    │
└──────┬──────┘
       │ HTTP POST /compute
       │ (CSV content in JSON)
       ▼
┌─────────────────┐
│ Go Orchestrator │
│  :8080          │
└──────┬──────────┘
       │ 1. Publish params to MQTT (retained)
       │ 2. Dispatch job to Nomad
       ▼
┌──────────────┐
│ Nomad Server │ Schedules job on worker
│  :4646       │
└──────┬───────┘
       │
       ▼
┌────────────────────┐
│ Fortran Worker     │
│ (Docker Container) │
│ - Reads from MQTT  │
│ - Runs diffusion   │
│ - Posts results    │
└────────┬───────────┘
         │ Results via MQTT
         ▼
┌──────────────────┐
│ Go Orchestrator  │
│ - LLM transcoding│ Uses Ollama (qwen2.5:7b)
│ - Pattern naming │
│ - Save to DB     │
└────────┬─────────┘
         │
         ▼
┌─────────────────┐
│ PostgreSQL      │ Stores patterns
│  :5432          │
└─────────────────┘
         │
         ▼
┌─────────────────┐
│ Web UI          │ Displays results
│ (auto-refresh)  │
└─────────────────┘
```

---

## Features Implemented

### ✅ File Upload
- Drag & drop support
- File validation
- Size display
- Progress indication

### ✅ Job Submission
- Reads CSV content into memory
- Sends as JSON parameter
- Handles large files (up to 10MB)
- Error handling with user feedback

### ✅ Real-time Monitoring
- Auto-refresh active jobs
- Status color coding
- Duration tracking
- Click to view details

### ✅ Results Display
- Pattern cards with LLM-generated descriptions
- Statistics grid
- Confidence levels
- Temporal patterns

---

## Troubleshooting

### Web UI doesn't load
```bash
# Check orchestrator is running
docker logs gpu-orchestrator

# Verify health endpoint
curl http://localhost:8080/health
```

### Job submission fails
```bash
# Check MQTT broker
docker logs mqtt-broker

# Verify Nomad is running
curl http://localhost:4646/v1/status/leader
```

### No workers available
```bash
# Start Fortran worker
docker-compose -f docker-compose.enhanced.yaml up -d fortran-worker

# Check worker logs
docker logs fortran-diffusion-worker
```

### Results don't show patterns
```bash
# Check Ollama is accessible
curl http://localhost:11434/api/tags

# Check database connection
docker exec postgres-patterns psql -U postgres -d home_automation -c "\dt"
```

---

## Next Steps

1. **Test with Real Data**:
   - Upload your anchor CSV file
   - Watch the diffusion process in real-time
   - View LLM-transcoded patterns

2. **Scale Up**:
   - Deploy to Linux servers with GPUs
   - Use `Dockerfile.diffusion` for GPU workers
   - Configure multiple workers via Nomad

3. **Production Deployment**:
   - Change database password
   - Enable SSL/TLS
   - Add authentication
   - Configure monitoring (Prometheus/Grafana)

---

**Status**: ✅ Fully functional local development environment with web UI

**Last Updated**: 2025-11-14
