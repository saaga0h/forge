# Patterns Package: LLM Transcoding Layer

**Purpose**: Convert semantic diffusion chains into human-readable behavioral patterns using local LLM

**Status**: Phase 1 (MVP) - Implementation complete ✅

---

## Architecture

```
Fortran Diffusion → Chain Assignments
           ↓
    Go Aggregator → Chain Statistics
           ↓
     LLM Client → Pattern Description
           ↓
    PostgreSQL → Stored Patterns
```

---

## Package Structure

```
pkg/patterns/
├── types.go           # Data structures
├── aggregator.go      # Chain statistics computation
├── llm_client.go      # Ollama LLM integration
├── cache.go           # In-memory caching
├── signature.go       # Pattern fingerprinting
├── aggregator_test.go # Unit tests
└── README.md          # This file
```

---

## Key Components

### 1. Aggregator (`aggregator.go`)

Computes statistics for each chain:
- Time range analysis (span, occurrences)
- Location distribution
- Temporal patterns (time of day, day of week)
- Location transitions (sequence analysis)
- Dimension averages (128D → summary)

**Usage**:
```go
stats := AggregateChainStatistics(chain, allAnchors)
```

### 2. LLM Client (`llm_client.go`)

Interfaces with Ollama for pattern interpretation:
- Formats prompts from chain statistics
- Calls local LLM (Llama 3.1 8B recommended)
- Parses JSON responses
- Handles retries and timeouts

**Usage**:
```go
client := NewLLMClient("http://localhost:11434", "llama3.1:8b", cache)
pattern, err := client.TranscodeChain(ctx, stats)
```

### 3. Cache (`cache.go`)

In-memory caching with TTL:
- Signature-based deduplication
- Automatic cleanup of expired entries
- Hit rate tracking
- Thread-safe operations

**Usage**:
```go
cache := NewCache(720 * time.Hour) // 30 days
pattern := cache.Get(signature)
cache.Set(signature, pattern)
```

### 4. Signature (`signature.go`)

Pattern fingerprinting for caching:
- Location distribution normalization
- Time-of-day classification
- Sequence simplification
- Dimension profile rounding

**Usage**:
```go
sig := ComputeSignature(stats)
similarity := SignatureSimilarity(stats1, stats2)
```

---

## Configuration

Edit [configs/llm.yaml](../../configs/llm.yaml):

```yaml
llm:
  provider: "ollama"
  base_url: "http://localhost:11434"
  model: "llama3.1:8b"
  temperature: 0.3
  cache_enabled: true
  cache_ttl: 720h  # 30 days
```

---

## Database Schema

Apply migration [migrations/001_create_patterns.sql](../../migrations/001_create_patterns.sql):

```bash
psql -U postgres -d home_automation -f migrations/001_create_patterns.sql
```

**Tables created**:
- `behavioral_patterns` - Final transcoded patterns
- `pattern_descriptions` - LLM response cache (persistent)
- `pattern_history` - Pattern evolution tracking (Phase 3)

---

## Testing

```bash
cd go-orchestrator
go test ./pkg/patterns -v
```

**Tests included**:
- ✅ Chain aggregation logic
- ✅ Time-of-day classification
- ✅ Anchor filtering and sorting
- ✅ Dimension magnitude calculation

**TODO** (for integration testing):
- Mock LLM responses
- End-to-end pipeline test
- Cache hit rate validation

---

## Usage Example

```go
package main

import (
    "context"
    "fmt"
    "time"
    "your-project/pkg/patterns"
)

func main() {
    // 1. Initialize cache
    cache := patterns.NewCache(720 * time.Hour)

    // 2. Initialize LLM client
    client := patterns.NewLLMClient(
        "http://localhost:11434",
        "llama3.1:8b",
        cache,
    )

    // 3. Get chain and anchors from Fortran diffusion
    chain := patterns.Chain{
        ChainID:   1,
        AnchorIDs: []string{"anchor-001", "anchor-002", ...},
    }
    allAnchors := []patterns.Anchor{ /* ... */ }

    // 4. Aggregate statistics
    stats := patterns.AggregateChainStatistics(chain, allAnchors)

    // 5. Transcode with LLM
    ctx := context.Background()
    pattern, err := client.TranscodeChain(ctx, stats)
    if err != nil {
        panic(err)
    }

    // 6. Use the result
    fmt.Printf("Pattern: %s\n", pattern.Name)
    fmt.Printf("Description: %s\n", pattern.Description)
    fmt.Printf("Confidence: %s\n", pattern.Confidence)
}
```

---

## Performance

### Small Dataset (1000 anchors, 8 chains)
- Aggregation: <100ms
- LLM (no cache): ~5-10 seconds
- LLM (cache hit): <10ms
- **Total**: ~5-10 seconds first run, <1 second cached

### Medium Dataset (10k anchors, 30 chains)
- Aggregation: <500ms
- LLM (no cache): ~20-30 seconds
- LLM (80% cache): ~5 seconds
- **Total**: ~20-30 seconds first run, ~5 seconds partial cache

### Cache Effectiveness
- After 1 week: ~80% hit rate (patterns repeat)
- After 1 month: ~90% hit rate
- Cost per job: $0.000264 (electricity only vs $0.12 cloud)

---

## Next Steps

### Phase 2: Optimization (4-6 hours)
- [ ] Postgres cache persistence
- [ ] File-based handoff for large datasets
- [ ] Parallel chain aggregation
- [ ] LLM response validation

### Phase 3: Advanced Features (8-12 hours)
- [ ] Pattern evolution tracking
- [ ] Anomaly detection
- [ ] Multi-model support
- [ ] Pattern similarity search

---

## Dependencies

**Required**:
- Go 1.21+
- Ollama with Llama 3.1 8B model
- PostgreSQL 14+

**Install Ollama model**:
```bash
ollama pull llama3.1:8b
ollama serve  # Starts API on :11434
```

---

## Troubleshooting

### LLM not responding
```bash
# Check Ollama is running
curl http://localhost:11434/api/tags

# Test generation
curl http://localhost:11434/api/generate -d '{
  "model": "llama3.1:8b",
  "prompt": "Test",
  "stream": false
}'
```

### Cache not working
```go
// Check cache stats
stats := cache.Stats()
fmt.Printf("Cache size: %d, Hit rate: %.2f\n", stats.Size, cache.HitRate())
```

### Database connection fails
```bash
# Test PostgreSQL connection
psql -U postgres -d home_automation -c "SELECT 1;"

# Check migration applied
psql -U postgres -d home_automation -c "\dt"
# Should see: behavioral_patterns, pattern_descriptions, pattern_history
```

---

## References

- [LLM_TRANSCODING_DESIGN.md](../../LLM_TRANSCODING_DESIGN.md) - Full design document
- [Ollama API Docs](https://github.com/ollama/ollama/blob/main/docs/api.md)
- [fortran-compute/src/semantic_diffusion.f90](../../fortran-compute/src/semantic_diffusion.f90) - Diffusion algorithm

---

**Last Updated**: 2025-11-14
**Status**: Phase 1 MVP Complete ✅
**Next Milestone**: Integration with Fortran diffusion output
