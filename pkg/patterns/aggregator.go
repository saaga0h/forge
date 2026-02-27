package patterns

import (
	"math"
	"sort"
	"time"
)

// AggregateChainStatistics computes statistics for a single chain
func AggregateChainStatistics(chain Chain, allAnchors []Anchor) ChainStatistics {
	stats := ChainStatistics{
		ChainID:       chain.ChainID,
		AnchorCount:   len(chain.AnchorIDs),
		LocationStats: make(map[string]int),
	}

	// Filter anchors for this chain
	chainAnchors := filterAnchors(allAnchors, chain.AnchorIDs)

	if len(chainAnchors) == 0 {
		return stats
	}

	// Calculate time range
	stats.TimeRange = calculateTimeRange(chainAnchors)

	// Count location distribution
	for _, anchor := range chainAnchors {
		stats.LocationStats[anchor.Location]++
	}

	// Analyze temporal patterns
	stats.TemporalPattern = analyzeTemporalPattern(chainAnchors)

	// Extract location sequence
	stats.Sequence = extractLocationSequence(chainAnchors)

	// Average dimensions
	stats.DimensionAvgs = averageDimensions(chainAnchors)

	// Sample anchors (first 10)
	sampleCount := min(10, len(chainAnchors))
	stats.SampleAnchors = make([]AnchorSummary, sampleCount)
	for i := 0; i < sampleCount; i++ {
		stats.SampleAnchors[i] = AnchorSummary{
			ID:        chainAnchors[i].ID,
			Timestamp: time.Unix(chainAnchors[i].Timestamp, 0),
			Location:  chainAnchors[i].Location,
		}
	}

	return stats
}

// filterAnchors extracts anchors matching the given IDs
func filterAnchors(allAnchors []Anchor, ids []string) []Anchor {
	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[id] = true
	}

	filtered := make([]Anchor, 0, len(ids))
	for _, anchor := range allAnchors {
		if idSet[anchor.ID] {
			filtered = append(filtered, anchor)
		}
	}

	// Sort by timestamp
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp < filtered[j].Timestamp
	})

	return filtered
}

// calculateTimeRange computes time span statistics
func calculateTimeRange(anchors []Anchor) TimeRangeStats {
	if len(anchors) == 0 {
		return TimeRangeStats{}
	}

	earliest := anchors[0].Timestamp
	latest := anchors[0].Timestamp

	for _, anchor := range anchors {
		if anchor.Timestamp < earliest {
			earliest = anchor.Timestamp
		}
		if anchor.Timestamp > latest {
			latest = anchor.Timestamp
		}
	}

	spanSeconds := latest - earliest
	spanHours := float64(spanSeconds) / 3600.0

	// Count distinct time periods (group by day)
	days := make(map[string]bool)
	for _, anchor := range anchors {
		t := time.Unix(anchor.Timestamp, 0)
		dayKey := t.Format("2006-01-02")
		days[dayKey] = true
	}

	return TimeRangeStats{
		EarliestTimestamp: earliest,
		LatestTimestamp:   latest,
		SpanHours:         spanHours,
		Occurrences:       len(days),
	}
}

// analyzeTemporalPattern extracts temporal characteristics
func analyzeTemporalPattern(anchors []Anchor) TemporalStats {
	if len(anchors) == 0 {
		return TemporalStats{
			DayDistribution: make(map[string]int),
		}
	}

	dayDist := make(map[string]int)
	var hourSum float64

	for _, anchor := range anchors {
		t := time.Unix(anchor.Timestamp, 0)

		// Day of week
		dayName := t.Weekday().String()[:3] // Mon, Tue, Wed...
		dayDist[dayName]++

		// Hour of day
		hour := float64(t.Hour()) + float64(t.Minute())/60.0
		hourSum += hour
	}

	avgHour := hourSum / float64(len(anchors))
	timeOfDay := classifyTimeOfDay(avgHour)

	return TemporalStats{
		AvgHourOfDay:    avgHour,
		DayDistribution: dayDist,
		TimeOfDay:       timeOfDay,
	}
}

// classifyTimeOfDay categorizes hour into time period
func classifyTimeOfDay(hour float64) string {
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 22:
		return "evening"
	default:
		return "night"
	}
}

// extractLocationSequence finds common location transitions
func extractLocationSequence(anchors []Anchor) []LocationTransition {
	if len(anchors) < 2 {
		return []LocationTransition{}
	}

	// Track transitions
	type transitionKey struct {
		from string
		to   string
	}

	transitions := make(map[transitionKey][]float64) // Store durations

	for i := 0; i < len(anchors)-1; i++ {
		from := anchors[i].Location
		to := anchors[i+1].Location

		if from == to {
			continue // Skip same-location transitions
		}

		duration := float64(anchors[i+1].Timestamp-anchors[i].Timestamp) / 60.0 // Minutes
		key := transitionKey{from: from, to: to}
		transitions[key] = append(transitions[key], duration)
	}

	// Convert to slice and calculate averages
	result := make([]LocationTransition, 0, len(transitions))
	for key, durations := range transitions {
		var sum float64
		for _, d := range durations {
			sum += d
		}
		avg := sum / float64(len(durations))

		result = append(result, LocationTransition{
			From:        key.from,
			To:          key.to,
			Count:       len(durations),
			AvgDuration: avg,
		})
	}

	// Sort by count (most frequent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	// Limit to top 10 transitions
	if len(result) > 10 {
		result = result[:10]
	}

	return result
}

// averageDimensions computes average values for each dimension group
func averageDimensions(anchors []Anchor) DimensionAverages {
	if len(anchors) == 0 {
		return DimensionAverages{}
	}

	// Dimension ranges for semantic diffusion
	const (
		dimTemporalStart = 0
		dimTemporalEnd   = 12
		dimSpatialStart  = 12
		dimSpatialEnd    = 28
		dimWeatherStart  = 28
		dimWeatherEnd    = 40
		dimLightingStart = 40
		dimLightingEnd   = 60
		dimActivityStart = 60
		dimActivityEnd   = 80
		dimRhythmStart   = 80
		dimRhythmEnd     = 100
		dimLearnedStart  = 100
		dimLearnedEnd    = 128
	)  // Fixed: Added closing paren

	sums := make([]float64, 128)
	for _, anchor := range anchors {
		for i, val := range anchor.Vector {
			sums[i] += val
		}
	}

	n := float64(len(anchors))
	for i := range sums {
		sums[i] /= n
	}

	return DimensionAverages{
		Temporal: sums[dimTemporalStart:dimTemporalEnd],
		Spatial:  sums[dimSpatialStart:dimSpatialEnd],
		Weather:  sums[dimWeatherStart:dimWeatherEnd],
		Lighting: sums[dimLightingStart:dimLightingEnd],
		Activity: sums[dimActivityStart:dimActivityEnd],
		Rhythm:   sums[dimRhythmStart:dimRhythmEnd],
		Learned:  sums[dimLearnedStart:dimLearnedEnd],
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ComputeDimensionMagnitudes calculates L2 norms for each dimension group
func ComputeDimensionMagnitudes(dims DimensionAverages) map[string]float64 {
	magnitude := func(vec []float64) float64 {
		var sum float64
		for _, v := range vec {
			sum += v * v
		}
		return math.Sqrt(sum)
	}

	return map[string]float64{
		"temporal": magnitude(dims.Temporal),
		"spatial":  magnitude(dims.Spatial),
		"weather":  magnitude(dims.Weather),
		"lighting": magnitude(dims.Lighting),
		"activity": magnitude(dims.Activity),
		"rhythm":   magnitude(dims.Rhythm),
		"learned":  magnitude(dims.Learned),
	}
}
