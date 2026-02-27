package patterns

import "time"

// ChainStatistics contains aggregated statistics for a single behavioral chain
type ChainStatistics struct {
	ChainID         int                    `json:"chain_id"`
	AnchorCount     int                    `json:"anchor_count"`
	TimeRange       TimeRangeStats         `json:"time_range"`
	LocationStats   map[string]int         `json:"locations"`
	TemporalPattern TemporalStats          `json:"temporal_pattern"`
	Sequence        []LocationTransition   `json:"sequence"`
	DimensionAvgs   DimensionAverages      `json:"dimension_averages"`
	SampleAnchors   []AnchorSummary        `json:"sample_anchors"` // First 10
}

// TimeRangeStats describes the time span of a chain
type TimeRangeStats struct {
	EarliestTimestamp int64   `json:"earliest"`
	LatestTimestamp   int64   `json:"latest"`
	SpanHours         float64 `json:"span_hours"`
	Occurrences       int     `json:"occurrences"` // How many distinct time periods
}

// TemporalStats describes when a chain typically occurs
type TemporalStats struct {
	AvgHourOfDay    float64        `json:"avg_hour"`
	DayDistribution map[string]int `json:"day_distribution"` // Mon:7, Tue:7...
	TimeOfDay       string         `json:"time_of_day"`      // "night", "morning", "afternoon", "evening"
}

// LocationTransition describes movement between locations
type LocationTransition struct {
	From        string  `json:"from"`
	To          string  `json:"to"`
	Count       int     `json:"count"`
	AvgDuration float64 `json:"avg_duration_minutes"`
}

// DimensionAverages contains averaged dimension groups
type DimensionAverages struct {
	Temporal []float64 `json:"temporal"` // Avg of dims 1-12
	Spatial  []float64 `json:"spatial"`  // Avg of dims 13-28
	Weather  []float64 `json:"weather"`  // Avg of dims 29-40
	Lighting []float64 `json:"lighting"` // Avg of dims 41-60
	Activity []float64 `json:"activity"` // Avg of dims 61-80
	Rhythm   []float64 `json:"rhythm"`   // Avg of dims 81-100
	Learned  []float64 `json:"learned"`  // Avg of dims 101-128
}

// AnchorSummary is a simplified anchor for LLM context
type AnchorSummary struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Location  string    `json:"location"`
	// Vector summary omitted to reduce size
}

// PatternDescription is the LLM's interpretation of a chain
type PatternDescription struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Intent          string                 `json:"intent"`
	Characteristics PatternCharacteristics `json:"characteristics"`
	KeyObservations []string               `json:"key_observations"`
	Confidence      string                 `json:"confidence"` // "high", "medium", "low"
}

// PatternCharacteristics describes pattern properties
type PatternCharacteristics struct {
	PrimaryLocations []string `json:"primary_locations"`
	TypicalDuration  string   `json:"typical_duration"`
	Frequency        string   `json:"frequency"`    // "daily", "weekly", "occasional"
	TimeOfDay        string   `json:"time_of_day"`  // "morning", "afternoon", "evening", "night"
}

// ChainSignature is used for caching (hash of pattern characteristics)
type ChainSignature struct {
	LocationDistribution map[string]float64 // Normalized percentages
	TimeOfDay            string
	TypicalSequence      []string // Simplified location order
	DimensionProfile     []float64 // Rounded averages
}

// DiffusionResult represents the output from semantic diffusion
type DiffusionResult struct {
	JobID            string  `json:"job_id"`
	Status           string  `json:"status"`
	AnchorsProcessed int     `json:"anchors_processed"`
	ChainsFound      int     `json:"chains_found"`
	Iterations       int     `json:"iterations"`
	Convergence      float64 `json:"convergence"`
	Chains           []Chain `json:"chains"`
}

// Chain represents a single behavioral chain from diffusion
type Chain struct {
	ChainID     int      `json:"chain_id"`
	AnchorIDs   []string `json:"anchor_ids"`
	AnchorCount int      `json:"anchor_count"`
}

// Anchor represents a full semantic anchor with 128D vector
type Anchor struct {
	ID        string    `json:"id"`
	Timestamp int64     `json:"timestamp"`
	Location  string    `json:"location"`
	Vector    []float64 `json:"vector"` // 128 dimensions
}

// BehavioralPattern is the final stored pattern in database
type BehavioralPattern struct {
	ID              int                    `json:"id"`
	JobID           string                 `json:"job_id"`
	ChainID         int                    `json:"chain_id"`
	SignatureHash   string                 `json:"signature_hash"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Intent          string                 `json:"intent"`
	Confidence      string                 `json:"confidence"`
	AnchorCount     int                    `json:"anchor_count"`
	TimeRange       TimeRangeStats         `json:"time_range"`
	LocationStats   map[string]int         `json:"location_stats"`
	TemporalPattern TemporalStats          `json:"temporal_pattern"`
	Characteristics PatternCharacteristics `json:"characteristics"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}
