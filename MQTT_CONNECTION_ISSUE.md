# MQTT Connection Issue - Fortran Worker in Docker

## Problem

The Fortran diffusion worker fails to connect to the MQTT broker when running in Docker with error code **-6 (MQTTCLIENT_BAD_PROTOCOL)**.

## Status

### Working ✅
- Web UI at http://localhost:8080/
- Job submission via web interface
- Orchestrator publishing job parameters to MQTT (retained messages)
- MQTT broker (Mosquitto 2.0) - accessible from other Docker containers
- PostgreSQL patterns database
- Fortran worker compilation
- Fortran standalone test on Mac host (using macOS Paho MQTT library)

### Not Working ❌
- Fortran worker MQTT connection in Docker (Ubuntu ARM64, Paho MQTT 1.3.9)
- Nomad job dispatch (Docker driver not available in Nomad container on Mac)

## Investigation Summary

### What We Found

1. **MQTT broker is accessible**: Other Docker containers (mosquitto_pub/sub) can successfully connect and communicate
2. **Job parameters are published**: Orchestrator successfully publishes to `compute/jobs/{job_id}/params` with retained flag
3. **Worker binary runs**: The Fortran program starts, creates MQTT client handle successfully
4. **Connection fails at protocol level**: `MQTTClient_connect()` with NULL options returns -6

###Error Timeline

```
DEBUG: Creating MQTT client
  Broker URI: tcp://mqtt:1883
  Client ID: fortran-worker-0cf6e7d9-50e8-41c7-bcfe-98370f02d3b9
DEBUG: Client created successfully, attempting connection...
ERROR: MQTTClient_connect failed with code -6
```

### Root Cause

The Paho MQTT C library version 1.3.9 (Ubuntu ARM64) behaves differently than the macOS version when `NULL` is passed for connection options. Error code -6 indicates `MQTTCLIENT_BAD_PROTOCOL`, suggesting the library expects explicit connection options rather than defaulting to MQTT 3.1.1.

### What Was Tried

1. ✅ Verified MQTT broker connectivity from containers
2. ✅ Verified DNS resolution (mqtt -> 172.18.0.5)
3. ✅ Checked library linking (libpaho-mqtt3c.so.1.3.9 present)
4. ❌ Added explicit `MQTTClient_connectOptions` struct (struct version/layout mismatch)
5. ❌ Tried different struct_version values (7, 8)
6. ❌ Verified broker URL format (`tcp://mqtt:1883`)

## Possible Solutions

### Option 1: C Wrapper (Recommended)
Create a thin C wrapper around Paho MQTT that properly initializes connection options:

```c
// mqtt_wrapper.c
#include <MQTTClient.h>

int mqtt_connect_wrapper(void *handle) {
    MQTTClient_connectOptions conn_opts = MQTTClient_connectOptions_initializer;
    conn_opts.MQTTVersion = MQTTVERSION_3_1_1;
    conn_opts.keepAliveInterval = 60;
    conn_opts.cleansession = 1;
    return MQTTClient_connect(handle, &conn_opts);
}
```

Then call this from Fortran instead of calling `MQTTClient_connect` directly.

### Option 2: Use Mosquitto C Library
Switch from Paho MQTT C to the Mosquitto C library (`libmosquitto-dev`), which has a simpler API:

```fortran
! Mosquitto API is more straightforward
mosquitto_lib_init()
mosq = mosquitto_new(client_id, clean_session, NULL)
mosquitto_connect(mosq, broker, port, keepalive)
```

### Option 3: External MQTT Client
Run `mosquitto_sub` as a separate process to receive parameters, pipe to Fortran worker:

```bash
mosquitto_sub -h mqtt -t "compute/jobs/+/params" | fortran_worker
```

### Option 4: Different Paho Version
Try building against Paho MQTT C 1.3.13 (latest) instead of Ubuntu's 1.3.9.

## Current Workaround

For Mac development without Nomad dispatch:

1. Submit job via web UI at http://localhost:8080/
2. Note the `job_id` from the response or `/jobs` endpoint
3. Manually start worker:
   ```bash
   ./start-worker.sh <job_id>
   ```

## Next Steps

1. Implement Option 1 (C wrapper) - quickest fix
2. Test end-to-end flow with real CSV data
3. Verify LLM transcoding pipeline
4. Document deployment for Linux servers with actual GPU/Nomad dispatch

## Files Involved

- `fortran-compute/src/mqtt_client.f90` - MQTT client implementation
- `fortran-compute/Dockerfile.cpu` - Docker build configuration
- `start-worker.sh` - Manual worker startup script
- `docker-compose.enhanced.yaml` - Full stack configuration

## Environment

- **Host**: macOS (M4)
- **Docker**: Docker Desktop for Mac
- **MQTT Broker**: Mosquitto 2.0.20
- **Paho MQTT C**: 1.3.9 (Ubuntu 22.04 ARM64 in container)
- **Fortran**: gfortran (Ubuntu 22.04)

**Last Updated**: 2025-11-14
