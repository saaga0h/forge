# Clean Nomad dev server image for local orchestrator development.
# No compute worker embedded — real workers live in compute repos and deploy separately.
# Includes entrypoint that auto-registers the dev placeholder job on startup.

FROM ubuntu:22.04@sha256:962f6cadeae0ea6284001009daa4cc9a8c37e75d1f5191cf0eb83fe565b63dd7

RUN apt-get update && apt-get install -y \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY --from=hashicorp/nomad:1.8@sha256:e6ef961e11941b464784c3eed70b6e9822244eb3ca8a5167f5a46cb0d7fbc022 /bin/nomad /usr/local/bin/nomad

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
COPY client.hcl /etc/nomad.d/client.hcl
COPY nomad-dev.hcl /etc/nomad.d/nomad-dev.hcl
COPY gpu-compute-dev.nomad.hcl /etc/nomad.d/gpu-compute.nomad.hcl

RUN chmod +x /usr/local/bin/nomad /usr/local/bin/entrypoint.sh && \
    groupadd nomad && \
    useradd -r -g nomad nomad && \
    mkdir -p /opt/nomad/data && \
    chown -R nomad:nomad /opt/nomad

EXPOSE 4646 4647 4648

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
