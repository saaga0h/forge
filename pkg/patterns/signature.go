package patterns

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

// ComputeSignature generates a hash signature for a chain based on its characteristics
// This is used for caching - same behavioral pattern = same signature
func ComputeSignature(stats ChainStatistics) string {
	sig := ChainSignature{
		LocationDistribution: normalizeLocationDistribution(stats.LocationStats),
		TimeOfDay:            stats.TemporalPattern.TimeOfDay,
		TypicalSequence:      simplifySequence(stats.Sequence),
		DimensionProfile:     roundDimensionProfile(stats.DimensionAvgs),
	}

	// Serialize to JSON for consistent hashing
	data, err := json.Marshal(sig)
	if err != nil {
		// Fallback to simple hash if marshaling fails
		return fmt.Sprintf("error_%d", stats.ChainID)
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// normalizeLocationDistribution converts counts to percentages (0-1)
// This makes the signature independent of absolute anchor counts
func normalizeLocationDistribution(locations map[string]int) map[string]float64 {
	total := 0
	for _, count := range locations {
		total += count
	}

	if total == 0 {
		return make(map[string]float64)
	}

	normalized := make(map[string]float64)
	for loc, count := range locations {
		// Round to 2 decimal places for stability
		pct := math.Round(float64(count)/float64(total)*100) / 100
		normalized[loc] = pct
	}

	return normalized
}

// simplifySequence extracts the dominant location sequence pattern
// Returns ordered list of locations (without durations, which vary)
func simplifySequence(transitions []LocationTransition) []string {
	if len(transitions) == 0 {
		return []string{}
	}

	// Sort by count (most common first)
	sorted := make([]LocationTransition, len(transitions))
	copy(sorted, transitions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	// Take top 5 transitions
	maxTransitions := 5
	if len(sorted) > maxTransitions {
		sorted = sorted[:maxTransitions]
	}

	// Build sequence string
	sequence := make([]string, 0, len(sorted)*2)
	for _, t := range sorted {
		sequence = append(sequence, t.From, t.To)
	}

	// Deduplicate while preserving order
	seen := make(map[string]bool)
	unique := make([]string, 0, len(sequence))
	for _, loc := range sequence {
		if !seen[loc] {
			seen[loc] = true
			unique = append(unique, loc)
		}
	}

	return unique
}

// roundDimensionProfile rounds dimension averages for stable hashing
// Variations less than 0.1 are considered equivalent patterns
func roundDimensionProfile(dims DimensionAverages) []float64 {
	// Compute L2 norm for each dimension group
	groups := [][]float64{
		dims.Temporal,
		dims.Spatial,
		dims.Weather,
		dims.Lighting,
		dims.Activity,
		dims.Rhythm,
		dims.Learned,
	}

	profile := make([]float64, len(groups))
	for i, group := range groups {
		var sum float64
		for _, val := range group {
			sum += val * val
		}
		// Round magnitude to 1 decimal place
		profile[i] = math.Round(math.Sqrt(sum)*10) / 10
	}

	return profile
}

// CompareSignatures checks if two signatures are similar (for debugging)
func CompareSignatures(sig1, sig2 string) bool {
	return sig1 == sig2
}

// SignatureSimilarity computes similarity score between two chain statistics (0-1)
// Used for finding similar patterns even if signatures don't match exactly
func SignatureSimilarity(stats1, stats2 ChainStatistics) float64 {
	// Location similarity (Jaccard index)
	locSim := locationSimilarity(stats1.LocationStats, stats2.LocationStats)

	// Time of day similarity (1 if same, 0.5 if adjacent, 0 otherwise)
	timeSim := timeOfDaySimilarity(stats1.TemporalPattern.TimeOfDay, stats2.TemporalPattern.TimeOfDay)

	// Dimension profile similarity (cosine similarity)
	dimSim := dimensionSimilarity(stats1.DimensionAvgs, stats2.DimensionAvgs)

	// Weighted average
	return 0.4*locSim + 0.3*timeSim + 0.3*dimSim
}

// locationSimilarity computes Jaccard similarity between location sets
func locationSimilarity(locs1, locs2 map[string]int) float64 {
	if len(locs1) == 0 && len(locs2) == 0 {
		return 1.0
	}

	// Union and intersection
	union := make(map[string]bool)
	for loc := range locs1 {
		union[loc] = true
	}
	for loc := range locs2 {
		union[loc] = true
	}

	intersection := 0
	for loc := range locs1 {
		if _, ok := locs2[loc]; ok {
			intersection++
		}
	}

	return float64(intersection) / float64(len(union))
}

// timeOfDaySimilarity compares time periods
func timeOfDaySimilarity(t1, t2 string) float64 {
	if t1 == t2 {
		return 1.0
	}

	// Adjacent periods are somewhat similar
	adjacent := map[string][]string{
		"night":     {"morning", "evening"},
		"morning":   {"night", "afternoon"},
		"afternoon": {"morning", "evening"},
		"evening":   {"afternoon", "night"},
	}

	for _, adj := range adjacent[t1] {
		if adj == t2 {
			return 0.5
		}
	}

	return 0.0
}

// dimensionSimilarity computes cosine similarity between dimension profiles
func dimensionSimilarity(dims1, dims2 DimensionAverages) float64 {
	prof1 := roundDimensionProfile(dims1)
	prof2 := roundDimensionProfile(dims2)

	if len(prof1) != len(prof2) {
		return 0.0
	}

	var dot, mag1, mag2 float64
	for i := range prof1 {
		dot += prof1[i] * prof2[i]
		mag1 += prof1[i] * prof1[i]
		mag2 += prof2[i] * prof2[i]
	}

	mag1 = math.Sqrt(mag1)
	mag2 = math.Sqrt(mag2)

	if mag1 == 0 || mag2 == 0 {
		return 0.0
	}

	return dot / (mag1 * mag2)
}
