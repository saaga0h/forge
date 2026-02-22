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

  # DEV MODE: No GPU constraints for Mac testing
  # In production, uncomment the constraint below:
  # constraint {
  #   attribute = "${attr.unique.hostname}"
  #   operator  = "regexp"
  #   value     = "gpu-node.*"
  # }

  group "compute" {
    count = 1

    # Restart policy for batch jobs
    restart {
      attempts = 2
      delay    = "15s"
      interval = "5m"
      mode     = "fail"
    }

    # Resource requirements
    task "fortran-worker" {
      # DEV MODE: Use Docker driver instead of raw_exec/singularity
      driver = "docker"

      # Environment variables
      env {
        MQTT_BROKER = "tcp://mqtt:1883"
        JOB_ID      = "${NOMAD_META_JOB_ID}"
        WORKER_ID   = "nomad-worker-${NOMAD_ALLOC_ID}"
        NUM_THREADS = "4"
      }

      # Task configuration
      config {
        image = "fortran-diffusion-cpu:test"

        # Connect to the docker-compose network
        network_mode = "gpu-compute-orchestrator_gpu-compute-net"

        # Force pull is disabled since we're using local image
        force_pull = false
      }

      # Resource allocation (CPU-only for dev)
      resources {
        cpu    = 2000  # 2 CPU cores
        memory = 4096  # 4 GB RAM
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
