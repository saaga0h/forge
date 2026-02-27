//go:build integration

package integration_test

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// orchestratorURL returns the base URL for the orchestrator under test.
func orchestratorURL() string {
	if u := os.Getenv("TEST_ORCHESTRATOR_URL"); u != "" {
		return u
	}
	return "http://localhost:18080"
}

// mqttBrokerURL returns the MQTT broker address for the test stack.
func mqttBrokerURL() string {
	if u := os.Getenv("TEST_MQTT_BROKER"); u != "" {
		return u
	}
	return "tcp://localhost:11883"
}

// httpClient is a shared test HTTP client with a reasonable timeout.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// skipIfUnavailable skips the test if the orchestrator is not reachable.
func skipIfUnavailable(t *testing.T) {
	t.Helper()
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(orchestratorURL() + "/health")
	if err != nil {
		t.Skipf("orchestrator not available at %s: %v", orchestratorURL(), err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Skipf("orchestrator health check returned %d (not running?)", resp.StatusCode)
	}
}
