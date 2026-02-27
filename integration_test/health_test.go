//go:build integration

package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestHealth(t *testing.T) {
	skipIfUnavailable(t)

	resp, err := httpClient.Get(orchestratorURL() + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health: got status %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if status, ok := result["status"].(string); !ok || status != "ok" {
		t.Errorf("health.status: got %v, want \"ok\"", result["status"])
	}

	if _, ok := result["version"]; !ok {
		t.Error("health response missing 'version' field")
	}

	timeStr, ok := result["time"].(string)
	if !ok {
		t.Error("health response missing 'time' field")
	} else {
		if _, err := time.Parse(time.RFC3339, timeStr); err != nil {
			t.Errorf("health.time %q does not parse as RFC3339: %v", timeStr, err)
		}
	}
}
