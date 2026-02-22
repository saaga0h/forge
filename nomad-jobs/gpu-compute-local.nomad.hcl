# nomad-jobs/gpu-compute-local.nomad.hcl
# Local development version - runs Fortran binary directly in nomad container
# For Mac development without GPU/Singularity

job "gpu-compute" {
  type = "batch"
  
  parameterized {
    meta_required = ["job_id"]
    meta_optional = ["priority"]
  }

  datacenters = ["dc1"]

  group "compute" {
    count = 1

    restart {
      attempts = 2
      delay    = "15s"
      interval = "5m"
      mode     = "fail"
    }

    network {
      mode = "bridge"
    }

    task "fortran-worker" {
      driver = "raw_exec"
      
      env {
        MQTT_BROKER = "tcp://mqtt:1883"  # Docker network hostname
        JOB_ID      = "${NOMAD_META_JOB_ID}"
        OMP_NUM_THREADS = "4"
      }

      config {
        command = "/app/main_diffusion_cpu"  # Direct binary execution
      }

      resources {
        cpu    = 2000  # 2 CPU cores
        memory = 4096  # 4 GB RAM
      }

      kill_timeout = "30s"
    }
  }
}