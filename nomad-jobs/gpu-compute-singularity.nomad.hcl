# nomad-jobs/gpu-compute-singularity.nomad.hcl
# Alternative version using Nomad's Singularity driver plugin (if installed)

job "gpu-compute" {
  type = "batch"
  
  parameterized {
    meta_required = ["job_id", "operation", "size"]
    meta_optional = ["priority"]
  }

  datacenters = ["dc1"]
  
  # Only run on GPU nodes
  constraint {
    attribute = "${node.class}"
    value     = "gpu"
  }

  group "compute" {
    count = 1

    restart {
      attempts = 2
      delay    = "15s"
      interval = "5m"
      mode     = "fail"
    }

    # Network for MQTT access
    network {
      mode = "host"
    }

    task "fortran-worker" {
      driver = "singularity"
      
      env {
        MQTT_BROKER         = "tcp://${attr.unique.network.ip-address}:1883"
        JOB_ID              = "${NOMAD_META_JOB_ID}"
        OPERATION           = "${NOMAD_META_OPERATION}"
        SIZE                = "${NOMAD_META_SIZE}"
        OMP_NUM_THREADS     = "8"
        HIP_VISIBLE_DEVICES = "0"
        HSA_OVERRIDE_GFX_VERSION = "11.0.0"  # For RDNA 4 if needed
      }

      config {
        image   = "/opt/containers/gpu_worker.sif"
        
        # GPU device access
        binds = [
          "/dev/kfd:/dev/kfd",
          "/dev/dri:/dev/dri"
        ]
        
        # Security options
        security = [
          "seccomp:unconfined"
        ]
        
        # Add to video group for GPU access
        group_add = ["video"]
      }

      resources {
        cpu    = 4000
        memory = 8192
        
        device "amd/gpu" {
          count = 1
          
          constraint {
            attribute = "${device.model}"
            operator  = "regexp"
            value     = ".*R9700.*"
          }
        }
      }

      logs {
        max_files     = 5
        max_file_size = 10
      }

      kill_timeout = "30s"
      kill_signal  = "SIGTERM"
    }
  }
}