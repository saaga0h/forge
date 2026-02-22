#!/bin/bash
# nomad-jobs/test-dispatch.sh
# Manually test dispatching a GPU compute job

set -e

NOMAD_ADDR="${NOMAD_ADDR:-http://localhost:4646}"
JOB_ID="${1:-test-$(date +%s)}"

echo "=== Dispatching GPU Compute Job ==="
echo "Job ID: $JOB_ID"
echo "Nomad: $NOMAD_ADDR"
echo ""

# Dispatch the job
echo "Dispatching job..."
DISPATCH_OUTPUT=$(nomad job dispatch -meta "job_id=$JOB_ID" gpu-compute)

echo "$DISPATCH_OUTPUT"
echo ""

# Extract the dispatched job ID and evaluation ID
DISPATCHED_JOB=$(echo "$DISPATCH_OUTPUT" | grep "Dispatched Job ID" | awk '{print $NF}')
EVAL_ID=$(echo "$DISPATCH_OUTPUT" | grep "Evaluation ID" | awk '{print $NF}')

echo "Dispatched Job: $DISPATCHED_JOB"
echo "Evaluation: $EVAL_ID"
echo ""

# Wait a moment for allocation
sleep 2

# Get allocation ID
echo "Finding allocation..."
ALLOC_ID=$(nomad job allocs "$DISPATCHED_JOB" -json | jq -r '.[0].ID')

if [ -z "$ALLOC_ID" ] || [ "$ALLOC_ID" == "null" ]; then
    echo "ERROR: No allocation found"
    echo "Check job status with: nomad job status $DISPATCHED_JOB"
    exit 1
fi

echo "Allocation ID: $ALLOC_ID"
echo ""

# Show allocation status
echo "=== Allocation Status ==="
nomad alloc status "$ALLOC_ID"
echo ""

# Follow logs
echo "=== Following Logs (Ctrl+C to exit) ==="
echo ""
nomad alloc logs -f "$ALLOC_ID"
