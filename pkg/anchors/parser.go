package anchors

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

// SemanticAnchor represents a single anchor point in 128D semantic space
type SemanticAnchor struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Location  string    `json:"location"`
	Vector    []float64 `json:"vector"` // 128-dimensional vector
}

// AnchorDataset represents a collection of semantic anchors
type AnchorDataset struct {
	Anchors   []SemanticAnchor `json:"anchors"`
	NumDims   int              `json:"num_dims"`
	Generated time.Time        `json:"generated"`
}

// ParseAnchorCSV reads and parses anchor data from CSV file
func ParseAnchorCSV(filepath string) (*AnchorDataset, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comment = '#'
	reader.FieldsPerRecord = -1 // Variable fields

	dataset := &AnchorDataset{
		Anchors:   make([]SemanticAnchor, 0),
		NumDims:   128,
		Generated: time.Now(),
	}

	// Skip header lines manually since CSV reader doesn't expose them
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV: %w", err)
		}

		// Parse anchor record
		anchor, err := parseAnchorRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to parse anchor record: %w", err)
		}

		dataset.Anchors = append(dataset.Anchors, anchor)
	}

	return dataset, nil
}

// parseAnchorRecord parses a single CSV record into a SemanticAnchor
func parseAnchorRecord(record []string) (SemanticAnchor, error) {
	if len(record) < 4 {
		return SemanticAnchor{}, fmt.Errorf("invalid record: expected at least 4 fields, got %d", len(record))
	}

	anchor := SemanticAnchor{
		ID:       record[0],
		Location: record[2],
		Vector:   make([]float64, 0, 128),
	}

	// Parse timestamp
	timestampUnix, err := strconv.ParseInt(record[1], 10, 64)
	if err != nil {
		return SemanticAnchor{}, fmt.Errorf("failed to parse timestamp: %w", err)
	}
	anchor.Timestamp = time.Unix(timestampUnix, 0)

	// Parse vector values (starting from field 3)
	for i := 3; i < len(record); i++ {
		val, err := strconv.ParseFloat(record[i], 64)
		if err != nil {
			return SemanticAnchor{}, fmt.Errorf("failed to parse vector value at index %d: %w", i, err)
		}
		anchor.Vector = append(anchor.Vector, val)
	}

	return anchor, nil
}

// FormatAsCSV converts anchor dataset to CSV text format for compute workers.
// Format: CSV with header comments, followed by data lines:
// # Jeeves Anchor Export v1.0
// # Dimensions: 128
// id,timestamp,location,v1,v2,...,v128
func (d *AnchorDataset) FormatAsCSV() string {
	var builder strings.Builder

	// Write header
	builder.WriteString("# Jeeves Anchor Export v1.0\n")
	builder.WriteString(fmt.Sprintf("# Format: id,timestamp_unix,location,v1,v2,...,v%d\n", d.NumDims))
	builder.WriteString(fmt.Sprintf("# Dimensions: %d\n", d.NumDims))
	builder.WriteString(fmt.Sprintf("# Generated: %s\n", d.Generated.Format(time.RFC3339)))
	builder.WriteString(fmt.Sprintf("# Anchors: %d\n", len(d.Anchors)))

	// Write data lines
	for _, anchor := range d.Anchors {
		builder.WriteString(anchor.ID)
		builder.WriteString(",")
		builder.WriteString(strconv.FormatInt(anchor.Timestamp.Unix(), 10))
		builder.WriteString(",")
		builder.WriteString(anchor.Location)

		for _, val := range anchor.Vector {
			builder.WriteString(",")
			builder.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
		}

		builder.WriteString("\n")
	}

	return builder.String()
}

// Statistics about the dataset
func (d *AnchorDataset) Statistics() map[string]interface{} {
	if len(d.Anchors) == 0 {
		return map[string]interface{}{
			"num_anchors": 0,
			"num_dims":    d.NumDims,
		}
	}

	// Time range
	minTime := d.Anchors[0].Timestamp
	maxTime := d.Anchors[0].Timestamp
	locations := make(map[string]int)

	for _, anchor := range d.Anchors {
		if anchor.Timestamp.Before(minTime) {
			minTime = anchor.Timestamp
		}
		if anchor.Timestamp.After(maxTime) {
			maxTime = anchor.Timestamp
		}
		locations[anchor.Location]++
	}

	return map[string]interface{}{
		"num_anchors": len(d.Anchors),
		"num_dims":    d.NumDims,
		"time_range": map[string]string{
			"start": minTime.Format(time.RFC3339),
			"end":   maxTime.Format(time.RFC3339),
		},
		"locations": locations,
	}
}

// DiffusionResult represents the result from semantic diffusion computation
type DiffusionResult struct {
	DenoisedAnchors   []SemanticAnchor         `json:"denoised_anchors"`
	ChainAssignments  map[string]int           `json:"chain_assignments"` // anchor ID -> chain ID
	Chains            map[int][]string         `json:"chains"`            // chain ID -> []anchor IDs
	NumChains         int                      `json:"num_chains"`
	Iterations        int                      `json:"iterations"`
	ConvergenceMetric float64                  `json:"convergence_metric"`
	DurationMs        float64                  `json:"duration_ms"`
}

// ParseDiffusionResult parses the text output from semantic diffusion
func ParseDiffusionResult(resultText string) (*DiffusionResult, error) {
	result := &DiffusionResult{
		ChainAssignments: make(map[string]int),
		Chains:           make(map[int][]string),
	}

	lines := strings.Split(resultText, "\n")
	section := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if line == "" || strings.HasPrefix(line, "#") {
			// Parse header metadata
			if strings.HasPrefix(line, "# Chains:") {
				fmt.Sscanf(line, "# Chains: %d", &result.NumChains)
			} else if strings.HasPrefix(line, "# Iterations:") {
				fmt.Sscanf(line, "# Iterations: %d", &result.Iterations)
			} else if strings.HasPrefix(line, "# Convergence:") {
				fmt.Sscanf(line, "# Convergence: %f", &result.ConvergenceMetric)
			} else if line == "# DENOISED_ANCHORS" {
				section = "anchors"
			} else if line == "# CHAIN_ASSIGNMENTS" {
				section = "chains"
			}
			continue
		}

		switch section {
		case "chains":
			// Parse chain assignments: chain_1: id1 id2 id3
			if strings.Contains(line, "chain_") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					var chainID int
					fmt.Sscanf(parts[0], "chain_%d", &chainID)

					anchorIDs := strings.Fields(strings.TrimSpace(parts[1]))
					result.Chains[chainID] = anchorIDs

					for _, anchorID := range anchorIDs {
						result.ChainAssignments[anchorID] = chainID
					}
				}
			}
		}
	}

	return result, nil
}
