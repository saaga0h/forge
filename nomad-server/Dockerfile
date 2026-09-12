# Clean Nomad dev server image for local orchestrator development.
# No compute worker embedded — real workers live in compute repos and deploy separately.
# Includes entrypoint that auto-registers the dev placeholder job on startup.

FROM ubuntu:24.04@sha256:224a1869083a311ef3f13648a154ba79832fbef6364d31493642ca03082da254

RUN apt-get update && apt-get install -y \
    ca-certificates \
    wget \
    && rm -rf /var/lib/apt/lists/*

COPY --from=hashicorp/nomad:1.11@sha256:996069720c5dbb48c644bf5ac61f14593fb362408921b7d4c6086fb2cfc9f2c4 /bin/nomad /usr/local/bin/nomad

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
