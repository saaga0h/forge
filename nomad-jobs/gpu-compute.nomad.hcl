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
  
  # Job will be placed on nodes with GPU
  constraint {
    attribute = "${attr.unique.hostname}"
    operator  = "regexp"
    value     = "gpu-node.*"
  }

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
      driver = "raw_exec"  # We'll use raw_exec to call singularity
      
      # Environment variables
      env {
        MQTT_BROKER = "tcp://mqtt.service.consul:1883"
        JOB_ID      = "${NOMAD_META_JOB_ID}"
        OMP_NUM_THREADS = "8"
        HIP_VISIBLE_DEVICES = "0"
      }

      # Task configuration
      config {
        command = "/usr/local/bin/singularity"
        args = [
          "run",
          "--nv",  # NVIDIA GPU support (use --rocm for ROCm if available in your singularity version)
          "--bind", "/dev/kfd:/dev/kfd",
          "--bind", "/dev/dri:/dev/dri",
          "--containall",
          "/opt/containers/gpu_worker.sif"
        ]
      }

      # Resource allocation
      resources {
        cpu    = 4000  # 4 CPU cores
        memory = 8192  # 8 GB RAM
        
        # GPU device requirement
        device "amd/gpu" {
          count = 1
          
          # Constraints for specific GPU model
          constraint {
            attribute = "${device.model}"
            operator  = "regexp"
            value     = "Radeon.*R9700"
          }
        }
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