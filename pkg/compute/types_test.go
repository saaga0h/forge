package compute

import (
	"testing"
	"time"
)

func TestJob_IsComplete_States(t *testing.T) {
	tests := []struct {
		status   string
		complete bool
	}{
		{"completed", true},
		{"failed", true},
		{"timeout", true},
		{"cancelled", true},
		{"pending", false},
		{"dispatched", false},
		{"starting", false},
		{"computing", false},
		{"", false},
	}

	for _, tt := range tests {
		j := &Job{Status: tt.status}
		got := j.IsComplete()
		if got != tt.complete {
			t.Errorf("IsComplete() for status %q: got %v, want %v", tt.status, got, tt.complete)
		}
	}
}

func TestJob_Duration_WithCompletedAt(t *testing.T) {
	created := time.Unix(1700000000, 0)
	completed := created.Add(5 * time.Minute)

	j := &Job{
		CreatedAt:   created,
		CompletedAt: &completed,
	}

	got := j.Duration()
	want := 5 * time.Minute

	if got != want {
		t.Errorf("Duration(): got %v, want %v", got, want)
	}
}

func TestJob_Duration_NoCompletedAt(t *testing.T) {
	j := &Job{
		CreatedAt:   time.Now().Add(-10 * time.Second),
		CompletedAt: nil,
	}

	got := j.Duration()

	if got <= 0 {
		t.Errorf("Duration() without CompletedAt: got %v, expected > 0", got)
	}
	// Should be approximately 10 seconds (within 1s tolerance)
	if got > 15*time.Second {
		t.Errorf("Duration() without CompletedAt: got %v, expected roughly 10s", got)
	}
}

func TestRequest_Defaults(t *testing.T) {
	r := Request{}

	// Zero value access should not panic
	_ = r.Operation
	_ = r.Payload
	_ = r.Priority
	_ = r.Timeout

	if r.Operation != "" {
		t.Errorf("Operation: expected empty string, got %q", r.Operation)
	}
	if r.Payload != nil {
		t.Errorf("Payload: expected nil, got %v", r.Payload)
	}
}
