#!/bin/sh
set -e

echo "Starting Nomad server..."

# Start Nomad in the background with client config
# This fixes the "0 MHz" issue in Docker containers
nomad agent -dev -bind=0.0.0.0 \
  -config=/etc/nomad.d/client.hcl \
  &
NOMAD_PID=$!

# Wait for Nomad to be ready
echo "Waiting for Nomad to be ready..."
for i in $(seq 1 30); do
    if nomad node status > /dev/null 2>&1; then
        echo "Nomad is ready"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "ERROR: Nomad failed to start within 30 seconds"
        exit 1
    fi
    sleep 1
done

# Register the gpu-compute job if the job file exists
if [ -f /etc/nomad.d/gpu-compute.nomad.hcl ]; then
    echo "Registering gpu-compute job..."
    if nomad job run /etc/nomad.d/gpu-compute.nomad.hcl; then
        echo "✓ Job registered successfully"
    else
        echo "WARNING: Failed to register job (may already exist)"
    fi
else
    echo "WARNING: Job definition not found at /etc/nomad.d/gpu-compute.nomad.hcl"
fi

# Wait for Nomad process
wait $NOMAD_PID
