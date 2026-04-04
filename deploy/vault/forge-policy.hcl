# Vault policy for Forge Nomad jobs.
# Apply with: vault policy write forge deploy/vault/forge-policy.hcl
#
# Note: KV v2 stores data under secret/data/<path> even though
# `vault kv put` uses secret/<path> on the CLI.

path "secret/data/nomad/forge" {
  capabilities = ["read"]
}
