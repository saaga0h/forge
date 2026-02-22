# Testing Guide: Pattern Transcoding Layer

**Status**: Ready for end-to-end testing ✅
**Last Updated**: 2025-11-14

---

## Quick Start: Run End-to-End Test

### 1. With Mock LLM (No Dependencies)

```bash
cd go-orchestrator

# Run with synthetic data
go run ./cmd/test-patterns -input testdata/sample-diffusion-output.json -mock
```

**Expected output:**
- ✓ Loads diffusion results
- ✓ Processes 4 chains
- ✓ Aggregates statistics
- ✓ Mock LLM generates patterns
- ✓ Displays results

**Time**: < 1 second

---

### 2. With Real LLM (Requires Ollama)

**Prerequisites:**
```bash
# Install Ollama
curl https://ollama.ai/install.sh | sh

# Pull model
ollama pull llama3.1:8b

# Start service
ollama serve  # Runs on :11434
```

**Run test:**
```bash
go run ./cmd/test-patterns \
  -input testdata/sample-diffusion-output.json \
  -ollama-url http://localhost:11434 \
  -ollama-model llama3.1:8b
```

**Expected output:**
- ✓ Loads diffusion results
- ✓ Processes 4 chains
- ✓ Aggregates statistics
- ✓ **Real LLM** generates pattern descriptions
- ✓ Displays human-readable patterns

**Time**: ~20-30 seconds (first run, no cache)

---

### 3. With Database (Full Pipeline)

**Prerequisites:**
```bash
# Install PostgreSQL (if not installed)
brew install postgresql  # Mac
sudo apt install postgresql  # Linux

# Create database
psql -U postgres -c "CREATE DATABASE home_automation;"

# Apply schema
psql -U postgres -d home_automation -f migrations/001_create_patterns.sql
```

**Run test:**
```bash
go run ./cmd/test-patterns \
  -input testdata/sample-diffusion-output.json \
  -db-host localhost \
  -db-port 5432 \
  -db-user postgres \
  -db-pass postgres \
  -db-name home_automation
```

**Expected output:**
- ✓ All previous steps
- ✓ **Saves patterns to database**
- ✓ Caches descriptions in Postgres
- ✓ Shows database cache stats

---

## Command-Line Options

```bash
go run ./cmd/test-patterns [options]

Required:
  -input <path>          Path to diffusion output JSON

Optional:
  -mock                  Use mock LLM (default: false)
  -ollama-url <url>      Ollama API URL (default: http://localhost:11434)
  -ollama-model <model>  Model name (default: llama3.1:8b)
  -db-host <host>        Database host (default: localhost)
  -db-port <port>        Database port (default: 5432)
  -db-user <user>        Database user (default: postgres)
  -db-pass <pass>        Database password (default: postgres)
  -db-name <name>        Database name (default: home_automation)
```

---

## Creating Real Test Data

### From Fortran Diffusion

After running Fortran diffusion, create JSON output:

```fortran
! In Fortran (future enhancement):
! Export diffusion results as JSON for Go orchestrator

! Or manually create from Fortran output:
```

**JSON format:**
```json
{
  "job_id": "diffusion-2025-11-14-001",
  "status": "completed",
  "anchors_processed": 1009,
  "chains_found": 8,
  "iterations": 312,
  "convergence": 0.000098,
  "chains": [
    {
      "chain_id": 1,
      "anchor_ids": ["anchor-001", "anchor-002", ...],
      "anchor_count": 52
    }
  ]
}
```

---

## Testing Different Scenarios

### Scenario 1: Morning Routine Pattern

```json
{
  "job_id": "test-morning-routine",
  "chains": [{
    "chain_id": 1,
    "anchor_ids": ["morning-001", "morning-002", ...],
    "anchor_count": 15
  }]
}
```

Expected LLM output:
- Name: "Morning Routine - Breakfast"
- Time: morning (6-9am)
- Locations: bedroom → bathroom → kitchen → dining_room
- Confidence: high

---

### Scenario 2: Evening Relaxation

```json
{
  "job_id": "test-evening",
  "chains": [{
    "chain_id": 1,
    "anchor_ids": ["evening-001", ...],
    "anchor_count": 20
  }]
}
```

Expected LLM output:
- Name: "Evening Routine - Relaxation"
- Time: evening (18-22pm)
- Locations: kitchen → dining_room → living_room → bedroom
- Confidence: high

---

## Verifying Results

### Check Cache Performance

```bash
# Run twice - second should be faster
go run ./cmd/test-patterns -input testdata/sample.json

# Check cache stats in output:
# Cache: 4 entries, 4 total hits, 100.0% hit rate
```

### Query Database

```bash
psql -U postgres -d home_automation

-- View recent patterns
SELECT * FROM recent_patterns LIMIT 10;

-- Check cache stats
SELECT * FROM cache_stats;

-- Find specific pattern
SELECT name, description, confidence
FROM behavioral_patterns
WHERE job_id = 'diffusion-test-2025-11-14-001';

-- Check location distribution
SELECT name, location_stats
FROM behavioral_patterns
WHERE location_stats ? 'kitchen';  -- Has 'kitchen'
```

---

## Performance Benchmarking

### Small Dataset (100 anchors, 4 chains)

```bash
time go run ./cmd/test-patterns -input testdata/sample-diffusion-output.json -mock

# Expected:
# - Aggregation: ~1ms per chain
# - Mock LLM: <1ms
# - Total: <100ms
```

### Medium Dataset (1000 anchors, 10 chains)

```bash
time go run ./cmd/test-patterns -input testdata/medium.json

# Expected (with real LLM):
# - Aggregation: <10ms per chain
# - LLM (no cache): ~1s per chain
# - Total: ~10-15 seconds
```

### With Cache Hit

```bash
# Run twice
go run ./cmd/test-patterns -input testdata/medium.json  # First run
go run ./cmd/test-patterns -input testdata/medium.json  # Second run

# Second run should be ~100x faster (cache hits)
```

---

## Troubleshooting

### Error: "Cannot connect to Ollama"

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# If not running:
ollama serve

# Test model
ollama run llama3.1:8b "Test"
```

### Error: "Database connection failed"

```bash
# Check PostgreSQL is running
psql -U postgres -c "SELECT 1;"

# Check database exists
psql -U postgres -c "\l" | grep home_automation

# If not exists:
psql -U postgres -c "CREATE DATABASE home_automation;"
psql -U postgres -d home_automation -f migrations/001_create_patterns.sql
```

### Error: "No such file: testdata/sample.json"

```bash
# Create testdata directory
mkdir -p testdata

# Copy sample
cp testdata/sample-diffusion-output.json testdata/test.json

# Or specify full path
go run ./cmd/test-patterns -input /full/path/to/output.json -mock
```

---

## Integration with Fortran

### Future: Direct Integration

```go
// pkg/anchors/handler.go (not yet implemented)

func HandleDiffusionResults(result *patterns.DiffusionResult, anchors []patterns.Anchor) error {
    cache := patterns.NewCache(720 * time.Hour)
    client := patterns.NewLLMClient("http://localhost:11434", "llama3.1:8b", cache)
    store := database.NewPatternStore(db)

    for _, chain := range result.Chains {
        // 1. Aggregate
        stats := patterns.AggregateChainStatistics(chain, anchors)

        // 2. Transcode
        pattern, err := client.TranscodeChain(ctx, stats)
        if err != nil {
            return err
        }

        // 3. Store
        signature := patterns.ComputeSignature(stats)
        err = store.SavePattern(ctx, result.JobID, chain.ChainID, stats, pattern, signature)
        if err != nil {
            return err
        }
    }

    return nil
}
```

---

## Next Steps

1. **✓ Run basic test** (mock LLM)
2. **→ Setup Ollama** (real LLM)
3. **→ Test with real Fortran output**
4. **→ Setup PostgreSQL** (persistence)
5. **→ Integrate with anchor handler**
6. **→ Deploy to production**

---

## References

- [PATTERNS_IMPLEMENTATION_SUMMARY.md](../PATTERNS_IMPLEMENTATION_SUMMARY.md) - What was built
- [pkg/patterns/README.md](pkg/patterns/README.md) - API documentation
- [LLM_TRANSCODING_DESIGN.md](../LLM_TRANSCODING_DESIGN.md) - Original design

---

**Last Updated**: 2025-11-14
**Status**: Ready for testing ✅
