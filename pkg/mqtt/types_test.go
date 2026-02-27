package mqtt

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTopicBuilder_Params(t *testing.T) {
	tb := NewTopicBuilder("job-123")
	got := tb.Params()
	want := "compute/jobs/job-123/params"
	if got != want {
		t.Errorf("Params(): got %q, want %q", got, want)
	}
}

func TestTopicBuilder_Status(t *testing.T) {
	tb := NewTopicBuilder("job-123")
	got := tb.Status()
	want := "compute/jobs/job-123/status"
	if got != want {
		t.Errorf("Status(): got %q, want %q", got, want)
	}
}

func TestTopicBuilder_Result(t *testing.T) {
	tb := NewTopicBuilder("job-123")
	got := tb.Result()
	want := "compute/jobs/job-123/result"
	if got != want {
		t.Errorf("Result(): got %q, want %q", got, want)
	}
}

func TestTopicBuilder_Logs(t *testing.T) {
	tb := NewTopicBuilder("job-123")
	got := tb.Logs()
	want := "compute/jobs/job-123/logs"
	if got != want {
		t.Errorf("Logs(): got %q, want %q", got, want)
	}
}

func TestTopicPatternConstants(t *testing.T) {
	// All patterns must use '+' wildcard to match any job ID
	if !strings.Contains(TopicPatternResults, "+") {
		t.Errorf("TopicPatternResults %q: expected '+' wildcard", TopicPatternResults)
	}
	if !strings.Contains(TopicPatternStatus, "+") {
		t.Errorf("TopicPatternStatus %q: expected '+' wildcard", TopicPatternStatus)
	}
	if !strings.Contains(TopicPatternLogs, "+") {
		t.Errorf("TopicPatternLogs %q: expected '+' wildcard", TopicPatternLogs)
	}

	// Patterns must match the topic format produced by TopicBuilder
	tb := NewTopicBuilder("test-job")
	if !matchesMQTTPattern(TopicPatternResults, tb.Result()) {
		t.Errorf("TopicPatternResults %q should match %q", TopicPatternResults, tb.Result())
	}
	if !matchesMQTTPattern(TopicPatternStatus, tb.Status()) {
		t.Errorf("TopicPatternStatus %q should match %q", TopicPatternStatus, tb.Status())
	}
	if !matchesMQTTPattern(TopicPatternLogs, tb.Logs()) {
		t.Errorf("TopicPatternLogs %q should match %q", TopicPatternLogs, tb.Logs())
	}
}

// matchesMQTTPattern checks if a topic matches an MQTT pattern with '+' wildcards
func matchesMQTTPattern(pattern, topic string) bool {
	patParts := strings.Split(pattern, "/")
	topParts := strings.Split(topic, "/")
	if len(patParts) != len(topParts) {
		return false
	}
	for i, p := range patParts {
		if p != "+" && p != topParts[i] {
			return false
		}
	}
	return true
}

func TestJobResult_JSONRoundTrip(t *testing.T) {
	original := JobResult{
		JobID:     "job-xyz",
		Success:   true,
		WorkerID:  "test-worker-1",
		Timestamp: time.Unix(1700000000, 0).UTC(),
		Metrics: &Metrics{
			GPUUtilization: 85.5,
			MemoryUsedMB:   4096,
			FLOPs:          1000000,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got JobResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got.JobID != original.JobID {
		t.Errorf("JobID: got %q, want %q", got.JobID, original.JobID)
	}
	if got.Success != original.Success {
		t.Errorf("Success: got %v, want %v", got.Success, original.Success)
	}
	if got.WorkerID != original.WorkerID {
		t.Errorf("WorkerID: got %q, want %q", got.WorkerID, original.WorkerID)
	}
	if got.Metrics == nil {
		t.Fatal("Metrics: got nil, want non-nil")
	}
	if got.Metrics.GPUUtilization != original.Metrics.GPUUtilization {
		t.Errorf("Metrics.GPUUtilization: got %f, want %f", got.Metrics.GPUUtilization, original.Metrics.GPUUtilization)
	}
	if got.Metrics.MemoryUsedMB != original.Metrics.MemoryUsedMB {
		t.Errorf("Metrics.MemoryUsedMB: got %d, want %d", got.Metrics.MemoryUsedMB, original.Metrics.MemoryUsedMB)
	}
}

func TestJobLog_JSONRoundTrip(t *testing.T) {
	original := JobLog{
		JobID:     "job-log-1",
		Level:     "warn",
		Message:   "GPU memory approaching limit",
		Timestamp: time.Unix(1700000000, 0).UTC(),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var got JobLog
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	if got.Level != original.Level {
		t.Errorf("Level: got %q, want %q", got.Level, original.Level)
	}
	if got.Message != original.Message {
		t.Errorf("Message: got %q, want %q", got.Message, original.Message)
	}
	if got.JobID != original.JobID {
		t.Errorf("JobID: got %q, want %q", got.JobID, original.JobID)
	}
}
