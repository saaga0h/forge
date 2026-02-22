# nomad-jobs/nomad-client-gpu.hcl
# Configuration for Nomad client running on GPU node

# Basic settings
name       = "gpu-node-01"
datacenter = "dc1"
region     = "global"

# Client configuration
client {
  enabled = true
  
  # Node class for constraints
  node_class = "gpu"
  
  # Metadata
  meta {
    "gpu_type"  = "amd"
    "gpu_model" = "R9700"
    "rocm_version" = "7.0"
  }
  
  # Host volumes for container storage
  host_volume "containers" {
    path      = "/opt/containers"
    read_only = true
  }
  
  # Reserved resources (adjust based on your system)
  reserved {
    cpu            = 1000   # Reserve 1 core for system
    memory         = 2048   # Reserve 2GB for system
    disk           = 10240  # Reserve 10GB disk
  }
}

# Plugin configuration
plugin "raw_exec" {
  config {
    enabled = true
  }
}

# GPU device plugin configuration (if using Nomad device plugins)
plugin "amd_gpu" {
  config {
    enabled = true
    
    # Fingerprinting configuration
    fingerprint_period = "1m"
  }
}

# ACL (if using Nomad ACL)
acl {
  enabled = false
}

# Telemetry
telemetry {
  collection_interval = "10s"
  publish_allocation_metrics = true
  publish_node_metrics       = true
}

# Logging
log_level = "INFO"
log_json  = false

# Consul integration (if using service discovery)
consul {
  address = "127.0.0.1:8500"
  auto_advertise = true
  server_auto_join = true
  client_auto_join = true
}
