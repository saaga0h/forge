package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"gpu-compute-orchestrator/pkg/database"
	"gpu-compute-orchestrator/pkg/patterns"
)

func main() {
	// Command-line flags
	var (
		inputFile  = flag.String("input", "", "Path to compute diffusion output JSON")
		useMockLLM = flag.Bool("mock", false, "Use mock LLM (no Ollama required)")
		dbHost     = flag.String("db-host", "localhost", "Database host")
		dbPort     = flag.Int("db-port", 5432, "Database port")
		dbUser     = flag.String("db-user", "postgres", "Database user")
		dbPass     = flag.String("db-pass", "postgres", "Database password")
		dbName     = flag.String("db-name", "home_automation", "Database name")
		ollamaURL  = flag.String("ollama-url", "http://localhost:11434", "Ollama API URL")
		ollamaModel = flag.String("ollama-model", "llama3.1:8b", "Ollama model")
	)
	flag.Parse()

	if *inputFile == "" {
		log.Fatal("Usage: test-patterns -input <diffusion-output.json>")
	}

	fmt.Println("=== Pattern Transcoding Test ===")
	fmt.Println()

	// 1. Load diffusion results
	fmt.Printf("Loading diffusion results from %s...\n", *inputFile)
	diffResult, err := loadDiffusionResults(*inputFile)
	if err != nil {
		log.Fatalf("Load diffusion results: %v", err)
	}
	fmt.Printf("✓ Loaded: %d anchors, %d chains\n", diffResult.AnchorsProcessed, diffResult.ChainsFound)
	fmt.Println()

	// 2. Load anchor data (simplified - in real scenario, would come from database)
	fmt.Println("Loading anchor data...")
	anchors, err := loadAnchors()
	if err != nil {
		log.Fatalf("Load anchors: %v", err)
	}
	fmt.Printf("✓ Loaded: %d anchors\n", len(anchors))
	fmt.Println()

	// 3. Initialize cache and LLM client
	fmt.Println("Initializing LLM client...")
	cache := patterns.NewCache(720 * time.Hour) // 30 days
	client := patterns.NewLLMClient(*ollamaURL, *ollamaModel, cache)
	client.Mock = *useMockLLM
	if *useMockLLM {
		fmt.Println("✓ Using mock LLM (no Ollama required)")
	} else {
		fmt.Printf("✓ LLM client ready: %s\n", *ollamaModel)
	}
	fmt.Println()

	// 4. Connect to database (optional)
	var store *database.PatternStore
	if !*useMockLLM {
		fmt.Printf("Connecting to database %s@%s:%d/%s...\n", *dbUser, *dbHost, *dbPort, *dbName)
		db, err := database.ConnectDB(*dbHost, *dbPort, *dbUser, *dbPass, *dbName)
		if err != nil {
			log.Printf("⚠ Database connection failed: %v (continuing without DB)", err)
		} else {
			store = database.NewPatternStore(db)
			defer db.Close()
			fmt.Println("✓ Database connected")
		}
		fmt.Println()
	}

	// 5. Process each chain
	ctx := context.Background()
	fmt.Printf("Processing %d chains...\n", len(diffResult.Chains))
	fmt.Println()

	for i, chain := range diffResult.Chains {
		fmt.Printf("--- Chain %d/%d (ID: %d, Anchors: %d) ---\n",
			i+1, len(diffResult.Chains), chain.ChainID, chain.AnchorCount)

		// Aggregate statistics
		start := time.Now()
		stats := patterns.AggregateChainStatistics(chain, anchors)
		aggDuration := time.Since(start)

		fmt.Printf("  Aggregation: %v\n", aggDuration)
		fmt.Printf("  Time range: %.1f hours, %d occurrences\n",
			stats.TimeRange.SpanHours, stats.TimeRange.Occurrences)
		fmt.Printf("  Locations: %v\n", formatLocations(stats.LocationStats))
		fmt.Printf("  Time of day: %s (avg hour: %.1f)\n",
			stats.TemporalPattern.TimeOfDay, stats.TemporalPattern.AvgHourOfDay)

		// Transcode with LLM
		start = time.Now()
		pattern, err := client.TranscodeChain(ctx, stats)
		llmDuration := time.Since(start)

		if err != nil {
			log.Printf("  ✗ LLM error: %v\n", err)
			continue
		}

		fmt.Printf("  LLM: %v\n", llmDuration)
		fmt.Printf("  Pattern: %s\n", pattern.Name)
		fmt.Printf("  Confidence: %s\n", pattern.Confidence)
		fmt.Printf("  Description: %s\n", pattern.Description)

		// Store in database
		if store != nil {
			signature := patterns.ComputeSignature(stats)
			err = store.SavePattern(ctx, diffResult.JobID, chain.ChainID, stats, pattern, signature)
			if err != nil {
				log.Printf("  ⚠ Database save failed: %v\n", err)
			} else {
				fmt.Println("  ✓ Saved to database")
			}
		}

		fmt.Println()
	}

	// 6. Summary
	fmt.Println("=== Summary ===")
	stats := cache.Stats()
	fmt.Printf("Cache: %d entries, %d total hits, %.1f%% hit rate\n",
		stats.Size, stats.TotalHits, cache.HitRate()*100)

	if store != nil {
		cacheStats, err := store.GetCacheStats(ctx)
		if err == nil {
			fmt.Printf("Database cache: %v entries, %v total hits\n",
				cacheStats["total_entries"], cacheStats["total_hits"])
		}
	}

	fmt.Println()
	fmt.Println("✓ Test complete!")
}

// loadDiffusionResults loads semantic diffusion output
func loadDiffusionResults(path string) (*patterns.DiffusionResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var result patterns.DiffusionResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// loadAnchors loads anchor data (simplified version)
// In production, this would come from the original anchor CSV or database
func loadAnchors() ([]patterns.Anchor, error) {
	// For testing, we'll create synthetic anchors
	// In real usage, parse anchor-export.csv or load from DB

	now := time.Now().Unix()
	locations := []string{"bedroom", "bathroom", "kitchen", "living_room", "dining_room"}

	anchors := make([]patterns.Anchor, 100)
	for i := range anchors {
		vec := make([]float64, 128)
		// Initialize with some test values
		for j := range vec {
			vec[j] = 0.1 * float64(i%10)
		}

		anchors[i] = patterns.Anchor{
			ID:        fmt.Sprintf("anchor-%03d", i),
			Timestamp: now + int64(i*900), // 15 min intervals
			Location:  locations[i%len(locations)],
			Vector:    vec,
		}
	}

	return anchors, nil
}

// formatLocations formats location stats for display
func formatLocations(locs map[string]int) string {
	result := ""
	for loc, count := range locs {
		if result != "" {
			result += ", "
		}
		result += fmt.Sprintf("%s:%d", loc, count)
	}
	return result
}
