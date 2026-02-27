package patterns

import (
	"testing"
	"time"
)

// TestAggregateChainStatistics tests the main aggregation function
func TestAggregateChainStatistics(t *testing.T) {
	// Create test data: morning routine pattern
	now := time.Now().Unix()
	anchors := []Anchor{
		{
			ID:        "anchor-001",
			Timestamp: now,
			Location:  "bedroom",
			Vector:    make([]float64, 128),
		},
		{
			ID:        "anchor-002",
			Timestamp: now + 900, // 15 min later
			Location:  "bathroom",
			Vector:    make([]float64, 128),
		},
		{
			ID:        "anchor-003",
			Timestamp: now + 1200, // 20 min later
			Location:  "kitchen",
			Vector:    make([]float64, 128),
		},
	}

	// Initialize vectors with some test values
	for i := range anchors {
		// Temporal dimension
		for j := 0; j < 12; j++ {
			anchors[i].Vector[j] = 0.5
		}
		// Activity dimension
		for j := 60; j < 80; j++ {
			anchors[i].Vector[j] = 0.8
		}
	}

	chain := Chain{
		ChainID:     1,
		AnchorIDs:   []string{"anchor-001", "anchor-002", "anchor-003"},
		AnchorCount: 3,
	}

	stats := AggregateChainStatistics(chain, anchors)

	// Verify basic stats
	if stats.ChainID != 1 {
		t.Errorf("Expected ChainID 1, got %d", stats.ChainID)
	}

	if stats.AnchorCount != 3 {
		t.Errorf("Expected AnchorCount 3, got %d", stats.AnchorCount)
	}

	// Verify location stats
	if stats.LocationStats["bedroom"] != 1 {
		t.Errorf("Expected 1 bedroom anchor, got %d", stats.LocationStats["bedroom"])
	}
	if stats.LocationStats["bathroom"] != 1 {
		t.Errorf("Expected 1 bathroom anchor, got %d", stats.LocationStats["bathroom"])
	}
	if stats.LocationStats["kitchen"] != 1 {
		t.Errorf("Expected 1 kitchen anchor, got %d", stats.LocationStats["kitchen"])
	}

	// Verify time range
	if stats.TimeRange.SpanHours < 0.3 || stats.TimeRange.SpanHours > 0.4 {
		t.Errorf("Expected span ~0.33 hours, got %.2f", stats.TimeRange.SpanHours)
	}

	// Verify transitions
	if len(stats.Sequence) < 2 {
		t.Errorf("Expected at least 2 transitions, got %d", len(stats.Sequence))
	}

	// Check that we have bedroom->bathroom and bathroom->kitchen
	foundBedroomBathroom := false
	foundBathroomKitchen := false
	for _, trans := range stats.Sequence {
		if trans.From == "bedroom" && trans.To == "bathroom" {
			foundBedroomBathroom = true
		}
		if trans.From == "bathroom" && trans.To == "kitchen" {
			foundBathroomKitchen = true
		}
	}

	if !foundBedroomBathroom {
		t.Error("Missing bedroom->bathroom transition")
	}
	if !foundBathroomKitchen {
		t.Error("Missing bathroom->kitchen transition")
	}

	// Verify dimension averages
	if len(stats.DimensionAvgs.Temporal) != 12 {
		t.Errorf("Expected 12 temporal dimensions, got %d", len(stats.DimensionAvgs.Temporal))
	}
	if len(stats.DimensionAvgs.Activity) != 20 {
		t.Errorf("Expected 20 activity dimensions, got %d", len(stats.DimensionAvgs.Activity))
	}
}

// TestClassifyTimeOfDay tests time period classification
func TestClassifyTimeOfDay(t *testing.T) {
	tests := []struct {
		hour     float64
		expected string
	}{
		{6.5, "morning"},
		{12.0, "afternoon"},
		{18.5, "evening"},
		{23.0, "night"},
		{2.0, "night"},
	}

	for _, tt := range tests {
		result := classifyTimeOfDay(tt.hour)
		if result != tt.expected {
			t.Errorf("Hour %.1f: expected %s, got %s", tt.hour, tt.expected, result)
		}
	}
}

// TestFilterAnchors tests anchor filtering
func TestFilterAnchors(t *testing.T) {
	anchors := []Anchor{
		{ID: "a1", Timestamp: 100},
		{ID: "a2", Timestamp: 200},
		{ID: "a3", Timestamp: 150},
	}

	ids := []string{"a1", "a3"}
	filtered := filterAnchors(anchors, ids)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 filtered anchors, got %d", len(filtered))
	}

	// Should be sorted by timestamp
	if filtered[0].ID != "a1" {
		t.Errorf("Expected first anchor to be a1, got %s", filtered[0].ID)
	}
	if filtered[1].ID != "a3" {
		t.Errorf("Expected second anchor to be a3, got %s", filtered[1].ID)
	}
}

// TestComputeDimensionMagnitudes tests dimension magnitude calculation
func TestComputeDimensionMagnitudes(t *testing.T) {
	dims := DimensionAverages{
		Temporal: []float64{0.5, 0.5, 0.5}, // sqrt(0.75) ≈ 0.866
		Spatial:  []float64{1.0, 1.0},      // sqrt(2) ≈ 1.414
		Weather:  []float64{0.0, 0.0},      // 0
		Lighting: []float64{0.3, 0.4},      // sqrt(0.25) = 0.5
		Activity: []float64{1.0},           // 1.0
		Rhythm:   []float64{},              // 0
		Learned:  []float64{0.1, 0.1, 0.1}, // sqrt(0.03) ≈ 0.173
	}

	mags := ComputeDimensionMagnitudes(dims)

	tests := []struct {
		dim      string
		expected float64
		tolerance float64
	}{
		{"temporal", 0.866, 0.01},
		{"spatial", 1.414, 0.01},
		{"weather", 0.0, 0.01},
		{"lighting", 0.5, 0.01},
		{"activity", 1.0, 0.01},
		{"rhythm", 0.0, 0.01},
		{"learned", 0.173, 0.01},
	}

	for _, tt := range tests {
		actual := mags[tt.dim]
		if actual < tt.expected-tt.tolerance || actual > tt.expected+tt.tolerance {
			t.Errorf("%s magnitude: expected %.3f, got %.3f", tt.dim, tt.expected, actual)
		}
	}
}
