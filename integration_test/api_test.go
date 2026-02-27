//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestSubmitJob_Async(t *testing.T) {
	skipIfUnavailable(t)

	body := `{"operation":"ping","parameters":{"test":true}}`
	resp, err := httpClient.Post(orchestratorURL()+"/compute", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /compute: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /compute: got status %d, want 202\nbody: %s", resp.StatusCode, string(data))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := result["job_id"]; !ok {
		t.Error("response missing 'job_id' field")
	}

	status, _ := result["status"].(string)
	if status != "pending" && status != "dispatched" {
		t.Errorf("status: got %q, want 'pending' or 'dispatched'", status)
	}
}

func TestSubmitJob_InvalidBody(t *testing.T) {
	skipIfUnavailable(t)

	resp, err := httpClient.Post(orchestratorURL()+"/compute", "application/json", strings.NewReader("{invalid json"))
	if err != nil {
		t.Fatalf("POST /compute: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /compute (bad JSON): got status %d, want 400", resp.StatusCode)
	}
}

func TestSubmitJob_WrongMethod(t *testing.T) {
	skipIfUnavailable(t)

	resp, err := httpClient.Get(orchestratorURL() + "/compute")
	if err != nil {
		t.Fatalf("GET /compute: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /compute: got status %d, want 405", resp.StatusCode)
	}
}

func TestListJobs(t *testing.T) {
	skipIfUnavailable(t)

	resp, err := httpClient.Get(orchestratorURL() + "/jobs")
	if err != nil {
		t.Fatalf("GET /jobs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /jobs: got status %d, want 200", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := result["jobs"]; !ok {
		t.Error("response missing 'jobs' field")
	}
	if _, ok := result["count"]; !ok {
		t.Error("response missing 'count' field")
	}

	count, _ := result["count"].(float64)
	if count < 0 {
		t.Errorf("count: got %f, want >= 0", count)
	}
}

func TestListJobs_AfterSubmit(t *testing.T) {
	skipIfUnavailable(t)

	// Submit a job
	body := `{"operation":"test","parameters":{}}`
	subResp, err := httpClient.Post(orchestratorURL()+"/compute", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /compute: %v", err)
	}
	subResp.Body.Close()

	if subResp.StatusCode != http.StatusAccepted {
		t.Skipf("job submission returned %d (Nomad may not be available)", subResp.StatusCode)
	}

	// List jobs
	resp, err := httpClient.Get(orchestratorURL() + "/jobs")
	if err != nil {
		t.Fatalf("GET /jobs: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	count, _ := result["count"].(float64)
	if count < 1 {
		t.Errorf("count: got %f, want >= 1 after job submission", count)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	skipIfUnavailable(t)

	resp, err := httpClient.Get(orchestratorURL() + "/jobs/nonexistent-job-id-xyz")
	if err != nil {
		t.Fatalf("GET /jobs/nonexistent: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET /jobs/nonexistent: got status %d, want 404", resp.StatusCode)
	}
}

func TestGetJob_Existing(t *testing.T) {
	skipIfUnavailable(t)

	// Submit a job
	body := `{"operation":"test","parameters":{}}`
	subResp, err := httpClient.Post(orchestratorURL()+"/compute", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /compute: %v", err)
	}
	defer subResp.Body.Close()

	if subResp.StatusCode != http.StatusAccepted {
		t.Skipf("job submission returned %d (Nomad may not be available)", subResp.StatusCode)
	}

	var subResult map[string]interface{}
	if err := json.NewDecoder(subResp.Body).Decode(&subResult); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}

	jobID, _ := subResult["job_id"].(string)
	if jobID == "" {
		t.Fatal("submit response missing job_id")
	}

	// Get the job
	resp, err := httpClient.Get(orchestratorURL() + "/jobs/" + jobID)
	if err != nil {
		t.Fatalf("GET /jobs/%s: %v", jobID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /jobs/%s: got status %d, want 200", jobID, resp.StatusCode)
	}

	var jobResult map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jobResult); err != nil {
		t.Fatalf("decode job response: %v", err)
	}

	if gotID, _ := jobResult["job_id"].(string); gotID != jobID {
		t.Errorf("job.job_id: got %q, want %q", gotID, jobID)
	}
	if _, ok := jobResult["status"]; !ok {
		t.Error("job response missing 'status' field")
	}
}

func TestCancelJob_NotFound(t *testing.T) {
	skipIfUnavailable(t)

	req, err := http.NewRequest(http.MethodPost, orchestratorURL()+"/jobs/cancel?job_id=nonexistent-xyz", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("POST /jobs/cancel: %v", err)
	}
	defer resp.Body.Close()

	// CancelJob returns error if job not found → handleCancelJob returns 500
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("POST /jobs/cancel (not found): got status %d, want 500", resp.StatusCode)
	}
}

func TestCancelJob_Success(t *testing.T) {
	skipIfUnavailable(t)

	// Submit a job
	body := `{"operation":"test","parameters":{}}`
	subResp, err := httpClient.Post(orchestratorURL()+"/compute", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /compute: %v", err)
	}
	defer subResp.Body.Close()

	if subResp.StatusCode != http.StatusAccepted {
		t.Skipf("job submission returned %d (Nomad may not be available)", subResp.StatusCode)
	}

	var subResult map[string]interface{}
	if err := json.NewDecoder(subResp.Body).Decode(&subResult); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}

	jobID, _ := subResult["job_id"].(string)
	if jobID == "" {
		t.Fatal("submit response missing job_id")
	}

	// Cancel the job
	cancelURL := fmt.Sprintf("%s/jobs/cancel?job_id=%s", orchestratorURL(), jobID)
	req, err := http.NewRequest(http.MethodPost, cancelURL, bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}

	cancelResp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("POST /jobs/cancel: %v", err)
	}
	defer cancelResp.Body.Close()

	if cancelResp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(cancelResp.Body)
		t.Fatalf("POST /jobs/cancel: got status %d, want 200\nbody: %s", cancelResp.StatusCode, string(data))
	}

	var cancelResult map[string]interface{}
	if err := json.NewDecoder(cancelResp.Body).Decode(&cancelResult); err != nil {
		t.Fatalf("decode cancel response: %v", err)
	}

	if status, _ := cancelResult["status"].(string); status != "cancelled" {
		t.Errorf("cancel.status: got %q, want 'cancelled'", status)
	}
}
