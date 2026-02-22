job "gpu-compute" {
  # Parameterized job - dispatched by Go orchestrator
  type = "batch"

  # This job is a parameterized job template
  parameterized {
    # Metadata passed from dispatcher
    meta_required = ["job_id"]
    meta_optional = ["priority"]
  }

  datacenters = ["dc1"]

  # DEV MODE: raw_exec for Mac development (binary embedded in Nomad container)
  # In production, use Docker driver with GPU constraints

  group "compute" {
    count = 1

    # Restart policy for batch jobs
    # For development: no restarts since worker exits after 30s timeout
    # In production with real jobs, you may want attempts = 1-2
    restart {
      attempts = 0
      delay    = "15s"
      interval = "5m"
      mode     = "fail"
    }

    # Reschedule policy for batch jobs
    # For development: no rescheduling when allocation fails
    # In production with real jobs, you may want attempts = 1-2
    reschedule {
      attempts  = 0
      unlimited = false
    }

    # Resource requirements
    task "fortran-worker" {
      # DEV MODE: Use raw_exec (binary is in Nomad container at /usr/local/bin)
      driver = "raw_exec"

      # Environment variables
      env {
        MQTT_BROKER = "tcp://mqtt:1883"
        NUM_THREADS = "4"
      }

      # Template to create wrapper script with dispatch metadata
      # Access dispatch metadata via NOMAD_META_ environment variables
      template {
        data = <<EOF
#!/bin/sh
export JOB_ID="{{ env "NOMAD_META_job_id" }}"
export WORKER_ID="nomad-worker-{{ env "NOMAD_ALLOC_ID" }}"
export MQTT_BROKER="{{ env "MQTT_BROKER" }}"
export NUM_THREADS="{{ env "NUM_THREADS" }}"
exec /usr/local/bin/semantic_diffusion_cpu
EOF
        destination = "local/run.sh"
        perms = "755"
      }

      # Task configuration
      config {
        # Run the generated wrapper script
        command = "${NOMAD_TASK_DIR}/run.sh"
      }

      # Resource allocation (CPU-only for dev)
      resources {
        cpu    = 500   # 0.5 CPU cores (reduced for Mac dev)
        memory = 1024  # 1 GB RAM
      }

      # Logs
      logs {
        max_files     = 5
        max_file_size = 10  # MB
      }

      # Kill timeout
      kill_timeout = "30s"
    }
  }
}
