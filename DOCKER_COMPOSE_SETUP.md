# Docker Compose Local Testing Stack - Setup Guide

**Status**: Ready for testing (with notes)
**Created**: 2025-11-14
**Last Updated**: 2025-11-14

---

## What Was Built

### 1. Fortran CPU Worker Docker Image ✅

Created [fortran-compute/Dockerfile.cpu](fortran-compute/Dockerfile.cpu) for CPU-only testing on Mac:

**Features**:
- Ubuntu 22.04 base image
- gfortran compiler with OpenMP support
- MQTT client (libpaho-mqtt1.3)
- Multi-stage build (smaller runtime image)
- Health checks
- Built successfully: `fortran-diffusion-cpu:test`

**Build command**:
```bash
cd fortran-compute
docker build -f Dockerfile.cpu -t fortran-diffusion-cpu:test .
```

**Key fixes applied**:
- Fixed OpenMP pragma line length (split across multiple lines)
- Excluded .mod files from Docker COPY to avoid version conflicts
- Uses Makefile.mac for CPU-only build

---

### 2. Enhanced Docker Compose Stack ✅

Created [docker-compose.enhanced.yaml](docker-compose.enhanced.yaml) with complete local testing stack:

**Services**:
1. **mqtt** - Eclipse Mosquitto 2.0
2. **postgres** - PostgreSQL 16 (for pattern storage)
3. **ollama** - Ollama LLM server
4. **nomad-server** - HashiCorp Nomad (job orchestration)
5. **consul** - HashiCorp Consul (service discovery)
6. **orchestrator** - Go orchestrator with LLM integration
7. **fortran-worker** - CPU-only Fortran diffusion worker
8. **mqtt-explorer** - MQTT web UI (debugging)
9. **pgadmin** - PostgreSQL web UI (optional, `--profile debug`)

**Key features**:
- All services with health checks
- Proper dependency ordering
- Shared network: `gpu-compute-net`
- Volumes for data persistence
- PostgreSQL migrations auto-apply on startup
- CPU-only Fortran worker (Mac compatible)

---

## Current State

### Services Status

| Service | Status | Port | Notes |
|---------|--------|------|-------|
| MQTT | ⚠️ **Port conflict** | 1883 | Already running (jeeves-test-mosquitto) |
| PostgreSQL | ⚠️ **Port conflict** | 5432 | Already running (jeeves-test-postgres) |
| Ollama | ✅ Running | 11434 | Host Ollama (qwen2.5:7b available) |
| Nomad | ✅ Ready | 4646 | Started successfully |
| Consul | ✅ Ready | 8500 | Started successfully |
| Orchestrator | 🔶 Not tested | 8080 | Depends on MQTT/Postgres |
| Fortran Worker | 🔶 Not tested | - | Depends on MQTT |

### Port Conflicts

Two services from another project (`jeeves-test`) are currently running:
- `jeeves-test-mosquitto` on port 1883
- `jeeves-test-postgres` on port 5432

**Options to resolve**:

1. **Stop existing containers** (cleanest):
   ```bash
   docker stop jeeves-test-mosquitto jeeves-test-postgres
   docker-compose -f docker-compose.enhanced.yaml up -d
   ```

2. **Change ports** in docker-compose.enhanced.yaml:
   ```yaml
   mqtt:
     ports:
       - "1884:1883"  # Change external port

   postgres:
     ports:
       - "5433:5432"  # Change external port
   ```

3. **Use existing services** (requires configuration):
   - Configure orchestrator to use `localhost:1883` and `localhost:5432`
   - Remove mqtt and postgres from docker-compose.enhanced.yaml
   - Use `host.docker.internal` for container-to-host communication

---

## Testing Steps

### Option 1: Full Stack (Recommended)

```bash
# 1. Stop conflicting containers
docker stop jeeves-test-mosquitto jeeves-test-postgres

# 2. Start the enhanced stack
docker-compose -f docker-compose.enhanced.yaml up -d

# 3. Check status
docker-compose -f docker-compose.enhanced.yaml ps

# 4. View logs
docker-compose -f docker-compose.enhanced.yaml logs -f orchestrator

# 5. Test health checks
docker ps --filter "name=gpu-compute-orchestrator" --format "table {{.Names}}\t{{.Status}}"
```

### Option 2: Use Existing Services

```bash
# 1. Verify existing services are running
docker ps | grep jeeves-test

# 2. Start only new services (excluding mqtt and postgres)
docker-compose -f docker-compose.enhanced.yaml up -d \
  nomad-server consul orchestrator fortran-worker

# 3. Update orchestrator config to use host services
# (Modify docker-compose.enhanced.yaml orchestrator environment)
```

---

## Verification Tests

### 1. PostgreSQL Migrations

```bash
# Connect to database
docker exec -it postgres-patterns psql -U postgres -d home_automation

# Verify schema
\dt
# Should show: behavioral_patterns, pattern_descriptions

# Check views
\dv
# Should show: recent_patterns, cache_stats

\q
```

### 2. Ollama Service

```bash
# Test from host
curl http://localhost:11434/api/tags

# Test from orchestrator container (if using docker ollama)
docker exec gpu-orchestrator curl http://ollama:11434/api/tags
```

### 3. MQTT Connectivity

```bash
# Publish test message
docker exec mqtt-broker mosquitto_pub -h localhost -t test/topic -m "Hello"

# Subscribe to test messages (in another terminal)
docker exec mqtt-broker mosquitto_sub -h localhost -t test/#
```

### 4. Full Pipeline Test

```bash
# Run pattern test with database
cd go-orchestrator
go run ./cmd/test-patterns \
  -input testdata/sample-diffusion-output.json \
  -ollama-url http://localhost:11434 \
  -ollama-model qwen2.5:7b \
  -db-host localhost \
  -db-port 5432 \
  -db-pass postgres_dev_password
```

---

## Files Modified/Created

### Created Files

1. **fortran-compute/Dockerfile.cpu** (48 lines)
   - CPU-only Docker image for Mac development
   - Multi-stage build with Ubuntu 22.04
   - OpenMP support for parallelization

2. **docker-compose.enhanced.yaml** (226 lines)
   - Complete local testing stack
   - All services with health checks
   - PostgreSQL, Ollama, Fortran worker, orchestrator

3. **DOCKER_COMPOSE_SETUP.md** (this file)
   - Setup guide and troubleshooting

### Modified Files

1. **fortran-compute/src/semantic_diffusion.f90:249-251**
   - Fixed OpenMP pragma line truncation
   - Split long directive across multiple lines:
   ```fortran
   !$OMP PARALLEL DO PRIVATE(i, j, d, sim, force_magnitude) &
   !$OMP SHARED(forces, similarities, positions, weights, learned_weight, n, n_dims) &
   !$OMP SCHEDULE(dynamic)
   ```

---

## Known Issues

### 1. Port Conflicts ⚠️

**Issue**: Ports 1883 (MQTT) and 5432 (PostgreSQL) already in use by jeeves-test containers.

**Solution**: See "Options to resolve" section above.

### 2. Ollama Performance on Mac

**Issue**: Containerized Ollama may be slower than host Ollama on Mac M4.

**Solution**: Use host Ollama instead:
```yaml
orchestrator:
  environment:
    - OLLAMA_URL=http://host.docker.internal:11434
```

### 3. Singularity on Mac

**Issue**: Mac doesn't support Singularity containers.

**Solution**: Already addressed - using Docker with CPU-only builds.

---

## Performance Expectations

### Fortran CPU Worker (Mac M4, 4 OpenMP threads)
- **Small dataset** (100 anchors): ~30-60 seconds
- **Medium dataset** (1000 anchors): ~2-5 minutes
- **Large dataset** (10k+ anchors): Use GPU worker instead

### LLM Pattern Transcoding (qwen2.5:7b)
- **Per chain** (first run): 3-6 seconds
- **Cached patterns**: ~10µs
- **Cost**: $0.000264 per job (electricity only)

### Database Operations
- **Pattern insert**: <5ms
- **Cache lookup**: <1ms
- **Complex queries**: <50ms

---

## Next Steps

1. **Resolve port conflicts**: Choose option 1, 2, or 3 above
2. **Start full stack**: `docker-compose -f docker-compose.enhanced.yaml up -d`
3. **Verify migrations**: Check PostgreSQL schema
4. **Test end-to-end**: Run pattern transcoding test
5. **Deploy to production**: Update configuration for GPU workers

---

## Production Deployment Notes

When deploying to production (Linux with GPUs):

1. **Use GPU Dockerfile**: Switch to `Dockerfile.diffusion` for Fortran worker
2. **Update Nomad jobs**: Configure for GPU allocation
3. **Change passwords**: Update `POSTGRES_PASSWORD` and `PGADMIN_PASSWORD`
4. **Add monitoring**: Prometheus, Grafana, AlertManager
5. **SSL/TLS**: Enable for MQTT, PostgreSQL, and HTTP endpoints
6. **Backup strategy**: Configure PostgreSQL backups
7. **Resource limits**: Set CPU/memory limits in docker-compose

---

## Troubleshooting

### Services won't start

```bash
# Check logs
docker-compose -f docker-compose.enhanced.yaml logs <service-name>

# Check health status
docker inspect <container-name> | grep -A 20 Health
```

### Database connection failed

```bash
# Check PostgreSQL is running
docker exec postgres-patterns pg_isready -U postgres

# Check migrations applied
docker exec postgres-patterns psql -U postgres -d home_automation -c "\dt"
```

### Fortran worker not connecting

```bash
# Check MQTT broker
docker exec mqtt-broker mosquitto_sub -h localhost -t '#' -v

# Check worker logs
docker logs fortran-diffusion-worker
```

---

## References

- [TESTING_GUIDE.md](go-orchestrator/TESTING_GUIDE.md) - Pattern transcoding tests
- [LLM_TRANSCODING_DESIGN.md](LLM_TRANSCODING_DESIGN.md) - Original design doc
- [PATTERNS_IMPLEMENTATION_SUMMARY.md](PATTERNS_IMPLEMENTATION_SUMMARY.md) - Implementation details

---

**Status**: Ready for user to resolve port conflicts and test full stack
