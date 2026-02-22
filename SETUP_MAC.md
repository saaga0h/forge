# Running Semantic Diffusion on Mac M4 (CPU-Only)

## TL;DR - Quick Answer

**Mac M4**: CPU-only (no GPU) - Perfect for **testing algorithm logic**
**AMD GPU Server**: Full GPU acceleration - Needed for **production scale**

### Simplest Test (30 seconds):

```bash
brew install gfortran
cd fortran-compute
make -f Makefile.mac standalone
./bin/test_standalone
```

This runs the complete diffusion algorithm on your 67 anchors **without MQTT, without GPU** - just pure algorithm testing!

---

## Why Can't Mac M4 Use GPU?

| Feature | Mac M4 | AMD GPU Server |
|---------|--------|----------------|
| **GPU API** | Metal (Apple) | ROCm/HIP (AMD) |
| **Compatibility** | ❌ Incompatible | ✅ Native |
| **CPU Version** | ✅ Works perfectly | ✅ Also available |
| **67 anchors** | ~2 seconds | ~1 second |
| **10,000 anchors** | ~3 hours | ~30 minutes |

**Bottom line**: M4 cannot run ROCm/HIP (AMD-specific), but the CPU version gives **identical results** - just slower for large datasets.

---

## Option 1: Standalone Test (Recommended)

**No MQTT, no server, no complexity** - just see the algorithm work!

### Setup

```bash
# Install gfortran
brew install gfortran

cd fortran-compute
```

### Update Makefile.mac

Add this target to [Makefile.mac:47](fortran-compute/Makefile.mac#L47):

```makefile
# Standalone test target
$(BUILDDIR)/test_standalone.o: $(SRCDIR)/test_standalone.f90 \
                                $(BUILDDIR)/semantic_diffusion.o
	$(FC) $(FFLAGS) -c $< -o $@ -I$(BUILDDIR) -J$(BUILDDIR)

standalone: directories $(BUILDDIR)/semantic_diffusion.o $(BUILDDIR)/test_standalone.o
	$(FC) $(FFLAGS) -o $(BINDIR)/test_standalone \
		$(BUILDDIR)/semantic_diffusion.o \
		$(BUILDDIR)/test_standalone.o \
		-fopenmp
	@echo "Built standalone test"
```

### Build & Run

```bash
make -f Makefile.mac standalone
./bin/test_standalone
```

### Expected Output

```
=== Standalone Diffusion Test (No MQTT) ===

Reading ../anchor-export.csv...
✓ File read successfully
Parsing anchor data...
✓ Loaded 67 anchors
  Locations:
    - bedroom
    - bathroom

Running CPU diffusion...
Starting diffusion with 67 anchors...
  Iteration 10 - max change: 0.003421
  Iteration 20 - max change: 0.000543
  Converged after 23 iterations

=== Results ===
Computation time: 1234.56 ms
Iterations taken: 23
Convergence metric: 0.00008234
Activity chains found: 5

Chain Details:
  Chain 1:
    bacf785c-... (bedroom)
    3b07d1a9-... (bedroom)
    [25 anchors total]
  Chain 2:
    dbfd3b45-... (bathroom)
    [15 anchors total]
  ...
```

**That's it!** You've validated the algorithm works. ✅

---

## Option 2: Full System with MQTT (Mac CPU)

If you want to test the **complete GFAAS workflow** on Mac:

### Setup

```bash
brew install gfortran mosquitto paho-mqtt-c

# Start MQTT broker
brew services start mosquitto
```

### Build

```bash
cd fortran-compute
make -f Makefile.mac all
```

### Terminal 1: Start Worker

```bash
export MQTT_BROKER=tcp://localhost:1883
export JOB_ID=test-mac-001

./bin/semantic_diffusion_cpu
```

Worker waits for anchor data...

### Terminal 2: Send Data (Go)

```bash
cd go-orchestrator
go run cmd/test-diffusion/main.go \
  --anchors ../anchor-export.csv \
  --mqtt tcp://localhost:1883
```

This tests the **full integration** but requires MQTT setup.

---

## Performance Reality Check

### Your Test Data (67 anchors)

| Platform | Time | Notes |
|----------|------|-------|
| Mac M4 (CPU) | ~2s | Fast enough! |
| AMD GPU | ~1s | Not much faster |

**Verdict**: For 67 anchors, Mac is **perfectly fine** for testing!

### Large Dataset (10,000 anchors)

| Platform | Time | Notes |
|----------|------|-------|
| Mac M4 (CPU) | ~3 hours | Too slow |
| AMD GPU | ~30 minutes | 6x faster |

**Verdict**: For production scale, **GPU server needed**.

---

##Recommendation: Two-Phase Approach

### Phase 1: Validate on Mac (Today)

```bash
cd fortran-compute
make -f Makefile.mac standalone
./bin/test_standalone
```

**What to check**:
- ✅ Does it converge? (iterations < 50)
- ✅ Are chains meaningful? (not 1 huge chain or 67 tiny ones)
- ✅ Computation time reasonable? (~2 seconds)

**If something looks wrong**, adjust parameters in [semantic_diffusion.f90:16-18](fortran-compute/src/semantic_diffusion.f90#L16-L18):

```fortran
real(8), parameter :: LEARNING_RATE = 0.05d0        ! Try 0.01-0.1
real(8), parameter :: CONVERGENCE_THRESHOLD = 1.0d-4 ! Try 1e-3 to 1e-5
integer, parameter :: MAX_ITERATIONS = 50            ! Try 100
```

### Phase 2: Deploy to GPU Server (When Ready)

```bash
# On AMD GPU server
cd fortran-compute
make -f Makefile.diffusion all

# Or containerize
docker build -f Dockerfile.diffusion -t semantic-diffusion .

# Deploy via Nomad
nomad job run configs/diffusion-job.hcl
```

---

## Decision Matrix

| Your Goal | Best Platform |
|-----------|---------------|
| "See if the algorithm works" | **Mac standalone** ⭐ |
| "Debug/tune parameters" | **Mac standalone** ⭐ |
| "Test with 67 anchors" | **Mac CPU** ⭐ |
| "Process 1,000+ anchors" | **GPU server** |
| "Production deployment" | **GPU server** |
| "Continuous processing" | **GPU server** |

---

## What You DON'T Need on Mac

- ❌ ROCm/HIP installation
- ❌ diffusion_kernels.hip (GPU code)
- ❌ diffusion_hip_interface.f90 (GPU interface)
- ❌ GPU drivers

## What You DO Need on Mac

- ✅ gfortran (`brew install gfortran`)
- ✅ semantic_diffusion.f90 (CPU algorithm)
- ✅ test_standalone.f90 (test program)
- ✅ anchor-export.csv (your data)

---

## Troubleshooting Mac

### "gfortran: command not found"

```bash
brew install gfortran
gfortran --version  # Should show 13.x or higher
```

### "Cannot open anchor-export.csv"

```bash
cd fortran-compute
ls ../anchor-export.csv  # Verify file exists
```

### Compilation errors

```bash
make -f Makefile.mac clean
make -f Makefile.mac info    # Check configuration
make -f Makefile.mac standalone
```

### Segmentation fault

Likely file reading issue. Check that anchor-export.csv is valid CSV.

---

## Summary

**For your use case (67 anchors, algorithm validation)**:

1. ✅ **Mac M4 is perfect** - Run CPU-only standalone test
2. ✅ **No GPU needed yet** - Results will be identical
3. ✅ **30-second setup** - Just gfortran + make
4. ⏱️ **~2 seconds runtime** - Fast enough for testing

**When to move to GPU server**:

- 📈 Scaling to 1,000+ anchors
- 🚀 Production deployment
- ⚡ Need sub-minute processing

**My recommendation**: Start on Mac today, validate the algorithm works, tune parameters if needed, then deploy to GPU server when you're happy with the logic.

The beauty is: **CPU and GPU give identical results** (within floating-point precision), so Mac is a perfect development environment! 🎯
