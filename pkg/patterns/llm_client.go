package patterns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"text/template"
	"time"
)

// LLMClient handles communication with local LLM (Ollama)
type LLMClient struct {
	BaseURL        string
	Model          string
	Temperature    float64
	MaxTokens      int
	Timeout        time.Duration
	RetryAttempts  int
	HTTPClient     *http.Client
	Cache          *Cache
	Mock           bool // For testing
}

// NewLLMClient creates a new LLM client with defaults
func NewLLMClient(baseURL, model string, cache *Cache) *LLMClient {
	return &LLMClient{
		BaseURL:       baseURL,
		Model:         model,
		Temperature:   0.3, // Low for consistent output
		MaxTokens:     1000,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Cache: cache,
		Mock:  false,
	}
}

// TranscodeChain converts chain statistics to human-readable pattern description
func (c *LLMClient) TranscodeChain(ctx context.Context, stats ChainStatistics) (*PatternDescription, error) {
	// Check cache first
	signature := ComputeSignature(stats)
	if cached := c.Cache.Get(signature); cached != nil {
		return cached, nil
	}

	// Mock response for testing
	if c.Mock {
		return c.mockResponse(stats), nil
	}

	// Format prompt
	prompt, err := c.formatPrompt(stats)
	if err != nil {
		return nil, fmt.Errorf("format prompt: %w", err)
	}

	// Call LLM with retries
	var result *PatternDescription
	for attempt := 0; attempt < c.RetryAttempts; attempt++ {
		result, err = c.callOllama(ctx, prompt)
		if err == nil {
			break
		}
		if attempt < c.RetryAttempts-1 {
			time.Sleep(time.Second * time.Duration(attempt+1))
		}
	}

	if err != nil {
		return nil, fmt.Errorf("call ollama after %d attempts: %w", c.RetryAttempts, err)
	}

	// Cache the result
	c.Cache.Set(signature, result)

	return result, nil
}

// formatPrompt creates the LLM prompt from chain statistics
func (c *LLMClient) formatPrompt(stats ChainStatistics) (string, error) {
	// Calculate location percentages
	totalLocs := 0
	for _, count := range stats.LocationStats {
		totalLocs += count
	}

	locationData := make([]map[string]interface{}, 0, len(stats.LocationStats))
	for loc, count := range stats.LocationStats {
		pct := 0.0
		if totalLocs > 0 {
			pct = float64(count) / float64(totalLocs) * 100.0
		}
		locationData = append(locationData, map[string]interface{}{
			"location":   loc,
			"count":      count,
			"percentage": fmt.Sprintf("%.1f", pct),
		})
	}

	// Compute dimension magnitudes for easier interpretation
	mags := ComputeDimensionMagnitudes(stats.DimensionAvgs)

	// Template data
	data := map[string]interface{}{
		"chain_id":      stats.ChainID,
		"anchor_count":  stats.AnchorCount,
		"span_hours":    fmt.Sprintf("%.1f", stats.TimeRange.SpanHours),
		"occurrences":   stats.TimeRange.Occurrences,
		"time_of_day":   stats.TemporalPattern.TimeOfDay,
		"avg_hour":      fmt.Sprintf("%.1f", stats.TemporalPattern.AvgHourOfDay),
		"day_dist":      formatDayDistribution(stats.TemporalPattern.DayDistribution),
		"locations":     locationData,
		"transitions":   stats.Sequence,
		"temporal_mag":  fmt.Sprintf("%.2f", mags["temporal"]),
		"spatial_mag":   fmt.Sprintf("%.2f", mags["spatial"]),
		"activity_mag":  fmt.Sprintf("%.2f", mags["activity"]),
		"sample_count":  len(stats.SampleAnchors),
		"samples":       stats.SampleAnchors,
	}

	tmpl := template.Must(template.New("prompt").Parse(promptTemplate))
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// formatDayDistribution formats day distribution as readable string
func formatDayDistribution(dist map[string]int) string {
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	result := ""
	for _, day := range days {
		if count, ok := dist[day]; ok {
			if result != "" {
				result += ", "
			}
			result += fmt.Sprintf("%s:%d", day, count)
		}
	}
	return result
}

// callOllama makes the HTTP request to Ollama API
func (c *LLMClient) callOllama(ctx context.Context, prompt string) (*PatternDescription, error) {
	reqBody := map[string]interface{}{
		"model":       c.Model,
		"prompt":      prompt,
		"format":      "json",
		"stream":      false,
		"temperature": c.Temperature,
		"options": map[string]interface{}{
			"num_predict": c.MaxTokens,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse Ollama response
	var ollamaResp struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Parse the JSON pattern description from response
	var pattern PatternDescription
	if err := json.Unmarshal([]byte(ollamaResp.Response), &pattern); err != nil {
		return nil, fmt.Errorf("parse pattern description: %w", err)
	}

	return &pattern, nil
}

// mockResponse returns a mock pattern for testing
func (c *LLMClient) mockResponse(stats ChainStatistics) *PatternDescription {
	return &PatternDescription{
		Name:        fmt.Sprintf("Pattern %d (Mock)", stats.ChainID),
		Description: fmt.Sprintf("Mock behavioral pattern with %d anchors", stats.AnchorCount),
		Intent:      "Testing",
		Characteristics: PatternCharacteristics{
			PrimaryLocations: []string{"test_location"},
			TypicalDuration:  "unknown",
			Frequency:        "unknown",
			TimeOfDay:        stats.TemporalPattern.TimeOfDay,
		},
		KeyObservations: []string{
			fmt.Sprintf("Spans %.1f hours", stats.TimeRange.SpanHours),
			fmt.Sprintf("Occurs %d times", stats.TimeRange.Occurrences),
		},
		Confidence: "low",
	}
}

// Prompt template for LLM
const promptTemplate = `You are a behavioral pattern analyst for smart home automation.

Analyze this behavioral chain:

Chain ID: {{.chain_id}}
Anchor count: {{.anchor_count}}

Time pattern:
- Span: {{.span_hours}} hours
- Occurs: {{.occurrences}} times
- Typical time: {{.time_of_day}} (avg hour: {{.avg_hour}})
- Days: {{.day_dist}}

Location distribution:
{{range .locations}}- {{.location}}: {{.count}} anchors ({{.percentage}}%)
{{end}}

Typical sequence:
{{range .transitions}}{{.From}} → {{.To}} ({{.Count}} times, avg {{printf "%.1f" .AvgDuration}} minutes)
{{end}}

Dimensional characteristics:
- Temporal emphasis: {{.temporal_mag}}
- Spatial coherence: {{.spatial_mag}}
- Activity type: {{.activity_mag}}

{{if gt .sample_count 0}}Sample anchors (first {{.sample_count}}):
{{range .samples}}- {{.Timestamp.Format "2006-01-02 15:04:05"}} | {{.Location}}
{{end}}{{end}}

Provide a structured analysis in JSON format:
{
  "name": "Concise pattern name (e.g., 'Morning Routine - Breakfast')",
  "description": "2-3 sentence description of what this pattern represents",
  "intent": "Predicted user intent or purpose",
  "characteristics": {
    "primary_locations": ["location1", "location2"],
    "typical_duration": "X minutes/hours",
    "frequency": "daily/weekly/occasional",
    "time_of_day": "morning/afternoon/evening/night"
  },
  "key_observations": [
    "Observation 1",
    "Observation 2"
  ],
  "confidence": "high/medium/low"
}`
