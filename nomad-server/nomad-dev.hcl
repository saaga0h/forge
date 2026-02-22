# Nomad dev configuration with explicit CPU resources for Mac/ARM64
# This solves the issue where Nomad can't detect CPU frequency on ARM64

# Client configuration
client {
  enabled = true

  # Explicitly set CPU resources (Mac M4 has ~3.5 GHz cores)
  # Setting total compute to 16 cores * 3500 MHz = 56000 MHz
  reserved {
    cpu = 8000  # Reserve 8 GHz for host system
  }

  # Override CPU detection
  cpu_total_compute = 56000  # 16 cores * 3500 MHz
}

# Server configuration
server {
  enabled = true
  bootstrap_expect = 1
}

# Enable raw_exec driver (required for embedded worker binary)
# Note: This is the default in dev mode, so we just document it here
