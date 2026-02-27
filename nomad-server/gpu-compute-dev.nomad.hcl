# gpu-compute-dev.nomad.hcl
# Development placeholder job for testing orchestrator dispatch locally.
# Real worker jobs live in compute worker repositories and are deployed separately.
# This job accepts the same dispatch contract (job_id metadata, MQTT env vars)
# but does nothing except sleep, simulating a worker that takes time to complete.

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
      attempts = 0
      mode     = "fail"
    }

    reschedule {
      attempts  = 0
      unlimited = false
    }

    task "compute-worker" {
      driver = "raw_exec"

      env {
        JOB_ID      = "${NOMAD_META_job_id}"
        MQTT_BROKER = "tcp://mqtt:1883"
      }

      config {
        command = "/bin/sh"
        args    = ["-c", "echo \"[dev-worker] job=$JOB_ID started\" && sleep 10 && echo \"[dev-worker] job=$JOB_ID done\""]
      }

      resources {
        cpu    = 100
        memory = 64
      }

      logs {
        max_files     = 3
        max_file_size = 5
      }

      kill_timeout = "5s"
    }
  }
}
