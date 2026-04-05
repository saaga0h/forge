# forge-daemons.hcl
# Long-running Forge GPU compute orchestrator service.
#
# Prerequisites:
#   - Vault policy "forge" applied (deploy/vault/forge-policy.hcl)
#   - raw_exec enabled on Nomad client:
#       plugin "raw_exec" { config { enabled = true } }
#   - Consul agent running on Nomad client
#   - Gitea repo variables: NOMAD_ADDR, ARTIFACT_BASE
#
# Deploy:
#   ARTIFACT_BASE=<url> NOMAD_ADDR=<url> envsubst '${ARTIFACT_BASE} ${NOMAD_ADDR}' < deploy/nomad/forge-daemons.hcl | nomad job run -
#
# Secrets stored at: secret/data/nomad/forge
#   MQTT_BROKER, MQTT_USER, MQTT_PASSWORD, LOG_LEVEL

job "forge-daemons" {
  datacenters = ["the-collective"]
  type        = "service"

  meta {
    artifact_base  = "${ARTIFACT_BASE}"
    nomad_addr     = "${NOMAD_ADDR}"
    # Name of the parameterized Nomad worker job Forge dispatches to
    worker_job     = "gpu-compute"
  }

  # Forge runs on the GPU node (where MQTT broker is reachable)
  constraint {
    attribute = "${meta.gpu}"
    value     = "true"
  }

  constraint {
    attribute = "${meta.rocm}"
    value     = "true"
  }

  group "daemons" {
    count = 1

    restart {
      attempts = 5
      interval = "5m"
      delay    = "15s"
      mode     = "delay"
    }

    # ── orchestrator ──────────────────────────────────────────────────────────

    task "orchestrator" {
      driver = "raw_exec"
      config {
        command = "/bin/sh"
        args    = ["-c", "chmod +x ${NOMAD_TASK_DIR}/orchestrator && exec ${NOMAD_TASK_DIR}/orchestrator"]
      }

      artifact {
        source      = "${NOMAD_META_artifact_base}/${attr.cpu.arch}/orchestrator"
        destination = "local/orchestrator"
        mode        = "file"
        options {
          checksum = "sha256:${ARTIFACT_SHA256}"
        }
      }

      template {
        destination = "secrets/forge.env"
        env         = true
        data        = <<EOT
{{ with secret "secret/data/nomad/forge" }}
MQTT_BROKER={{ .Data.data.MQTT_BROKER }}
MQTT_USER={{ .Data.data.MQTT_USER }}
MQTT_PASSWORD={{ .Data.data.MQTT_PASSWORD }}
LOG_LEVEL={{ .Data.data.LOG_LEVEL }}
{{ end }}
NOMAD_ADDR={{ env "NOMAD_META_nomad_addr" }}
NOMAD_JOB_NAME={{ env "NOMAD_META_worker_job" }}
EOT
      }

      vault {
        policies = ["forge"]
      }

      resources {
        cpu    = 200
        memory = 128
      }

    }

  }
}
