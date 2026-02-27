//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// FakeWorker simulates a compute worker over MQTT.
// It subscribes to job params topics and publishes fake results.
type FakeWorker struct {
	client  pahomqtt.Client
	mu      sync.Mutex
	results map[string]string // job_id → status to publish
	t       *testing.T
}

// newFakeWorker creates a FakeWorker connected to the test MQTT broker.
// The worker subscribes to compute/jobs/+/params and calls t.Cleanup to disconnect.
func newFakeWorker(t *testing.T) *FakeWorker {
	t.Helper()

	fw := &FakeWorker{
		results: make(map[string]string),
		t:       t,
	}

	opts := pahomqtt.NewClientOptions().
		AddBroker(mqttBrokerURL()).
		SetClientID(fmt.Sprintf("fake-worker-%d", time.Now().UnixNano())).
		SetCleanSession(true)

	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatalf("fake worker MQTT connect timeout")
	}
	if err := token.Error(); err != nil {
		t.Fatalf("fake worker MQTT connect: %v", err)
	}
	fw.client = client

	// Subscribe to all job params topics
	subToken := client.Subscribe("compute/jobs/+/params", 1, fw.onParams)
	if !subToken.WaitTimeout(5 * time.Second) {
		t.Fatalf("fake worker subscribe timeout")
	}
	if err := subToken.Error(); err != nil {
		t.Fatalf("fake worker subscribe: %v", err)
	}

	t.Cleanup(func() {
		client.Disconnect(250)
	})

	return fw
}

// SetResult configures what status the fake worker should publish when it receives params
// for any job. Call before submitting the job.
func (fw *FakeWorker) SetResult(status string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	fw.results["*"] = status
}

// onParams is called when the fake worker receives a params message.
// It extracts the job_id and publishes a fake result.
func (fw *FakeWorker) onParams(client pahomqtt.Client, msg pahomqtt.Message) {
	topic := msg.Topic()

	// Extract job_id from topic: compute/jobs/{job_id}/params
	parts := strings.Split(topic, "/")
	if len(parts) != 4 {
		fw.t.Logf("fake worker: unexpected topic format: %s", topic)
		return
	}
	jobID := parts[2]

	fw.mu.Lock()
	status, ok := fw.results["*"]
	fw.mu.Unlock()

	if !ok {
		status = "completed"
	}

	fw.publishResult(jobID, status)
}

func (fw *FakeWorker) publishResult(jobID, status string) {
	result := map[string]interface{}{
		"job_id":      jobID,
		"status":      status,
		"duration_ms": 100,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
	if status == "failed" {
		result["error"] = "compute failed (simulated)"
	} else {
		result["result"] = map[string]bool{"test": true}
	}

	data, _ := json.Marshal(result)
	resultTopic := fmt.Sprintf("compute/jobs/%s/result", jobID)

	token := fw.client.Publish(resultTopic, 1, false, data)
	if !token.WaitTimeout(3 * time.Second) {
		fw.t.Logf("fake worker: publish result timeout for job %s", jobID)
		return
	}
	if err := token.Error(); err != nil {
		fw.t.Logf("fake worker: publish result error for job %s: %v", jobID, err)
	}
	fw.t.Logf("fake worker: published %s result for job %s", status, jobID)
}

func TestFullFlow_JobCompletion(t *testing.T) {
	skipIfUnavailable(t)

	// Start fake worker BEFORE submitting the job
	fw := newFakeWorker(t)
	fw.SetResult("completed")

	// Give the subscription a moment to be established
	time.Sleep(200 * time.Millisecond)

	// Submit job via /compute/sync (synchronous, waits for result)
	body := `{"operation":"test","parameters":{"test":true},"timeout":10000000000}`
	resp, err := httpClient.Post(orchestratorURL()+"/compute/sync", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /compute/sync: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// The fake worker should publish completed result → orchestrator responds with 200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /compute/sync: got status %d, want 200\nbody: %s", resp.StatusCode, string(data))
	}

	var job map[string]interface{}
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if status, _ := job["status"].(string); status != "completed" {
		t.Errorf("job.status: got %q, want 'completed'", status)
	}
}

func TestFullFlow_JobFailure(t *testing.T) {
	skipIfUnavailable(t)

	// Start fake worker that reports failure
	fw := newFakeWorker(t)
	fw.SetResult("failed")

	// Give the subscription a moment to be established
	time.Sleep(200 * time.Millisecond)

	body := `{"operation":"test","parameters":{"test":true},"timeout":10000000000}`
	resp, err := httpClient.Post(orchestratorURL()+"/compute/sync", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /compute/sync: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	// When job status is "failed", handleComputeSync returns 500 with job JSON
	// (see main.go: only 200 when status == "completed")
	if resp.StatusCode == http.StatusOK {
		t.Error("POST /compute/sync: got 200 for failed job, expected non-200")
	}

	var job map[string]interface{}
	if err := json.Unmarshal(data, &job); err != nil {
		// Error text body (from http.Error) is also acceptable
		t.Logf("response body (non-JSON): %s", string(data))
		return
	}

	if status, _ := job["status"].(string); status != "failed" {
		t.Errorf("job.status: got %q, want 'failed'", status)
	}
}
