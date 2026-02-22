#!/bin/bash
# nomad-jobs/setup-and-deploy.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Nomad GPU Compute Setup ==="
echo ""

# Configuration
NOMAD_ADDR="${NOMAD_ADDR:-http://localhost:4646}"
CONTAINER_PATH="${CONTAINER_PATH:-/opt/containers/gpu_worker.sif}"
JOB_FILE="${JOB_FILE:-gpu-compute-singularity.nomad.hcl}"

echo "Nomad Address: $NOMAD_ADDR"
echo "Container Path: $CONTAINER_PATH"
echo "Job File: $JOB_FILE"
echo ""

# Check if Nomad is available
if ! command -v nomad &> /dev/null; then
    echo "ERROR: nomad command not found"
    exit 1
fi

# Check Nomad connectivity
echo "Checking Nomad connectivity..."
if ! nomad status &> /dev/null; then
    echo "ERROR: Cannot connect to Nomad at $NOMAD_ADDR"
    exit 1
fi

echo "✓ Connected to Nomad"
echo ""

# List available nodes
echo "Available Nomad nodes:"
nomad node status
echo ""

# Check for GPU nodes
echo "Checking for GPU nodes..."
GPU_NODES=$(nomad node status -json | jq -r '.[] | select(.NodeResources.Devices != null) | select(.NodeResources.Devices[] | select(.Type == "gpu")) | .ID')

if [ -z "$GPU_NODES" ]; then
    echo "WARNING: No GPU nodes detected in Nomad cluster"
    echo "Make sure your GPU node is:"
    echo "  1. Running Nomad agent"
    echo "  2. Has AMD GPU device plugin configured"
    echo "  3. Has /dev/kfd and /dev/dri accessible"
else
    echo "✓ Found GPU nodes:"
    echo "$GPU_NODES"
fi
echo ""

# Validate job file
echo "Validating job file: $JOB_FILE"
if ! nomad job validate "$SCRIPT_DIR/$JOB_FILE"; then
    echo "ERROR: Job validation failed"
    exit 1
fi

echo "✓ Job file is valid"
echo ""

# Register the parameterized job
echo "Registering parameterized job..."
nomad job run "$SCRIPT_DIR/$JOB_FILE"

if [ $? -eq 0 ]; then
    echo "✓ Job registered successfully"
else
    echo "ERROR: Job registration failed"
    exit 1
fi

echo ""
echo "=== Setup Complete ==="
echo ""
echo "The parameterized job 'gpu-compute' is now registered."
echo "The Go orchestrator can dispatch jobs using:"
echo "  nomad job dispatch -meta job_id=<id> gpu-compute"
echo ""
echo "To test manually:"
echo "  nomad job dispatch -meta job_id=test-$(date +%s) gpu-compute"
echo ""
echo "To monitor:"
echo "  nomad job status gpu-compute"
echo "  nomad alloc logs -f <alloc-id>"
echo ""