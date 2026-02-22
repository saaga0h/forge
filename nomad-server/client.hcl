client {
  enabled = true

  # Explicit resource reservation for Docker container environment
  # This fixes the "0 MHz CPU" detection issue
  reserved {
    cpu            = 100  # Reserve 100 MHz for system
    memory         = 512  # Reserve 512 MB for system
    disk           = 1024 # Reserve 1 GB for system
  }

  # Set total resources explicitly (Docker Desktop usually has ~4000 MHz available)
  cpu_total_compute = 4000
}
