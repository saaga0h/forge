package compute

import (
	"gpu-compute-orchestrator/pkg/mqtt"
	"time"
)

// Request represents a compute job request
type Request struct {
	Operation string                 `json:"operation"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Priority  string                 `json:"priority,omitempty"` // low, normal, high
	Timeout   time.Duration          `json:"timeout,omitempty"`
}

// Job represents a compute job in the system
type Job struct {
	ID        string                 `json:"job_id"`
	Operation string                 `json:"operation"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
	Status      string                 `json:"status"` // pending, dispatched, starting, computing, completed, failed, timeout, cancelled
	Progress    float64                `json:"progress,omitempty"`
	Result      interface{}            `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	DurationMS  int64                  `json:"duration_ms,omitempty"`

	// Nomad info
	NomadEvalID string `json:"nomad_eval_id,omitempty"`
	NomadJobID  string `json:"nomad_job_id,omitempty"`

	// Metrics
	Metrics *mqtt.Metrics `json:"metrics,omitempty"`

	// Internal channel for result notification
	resultChan chan *mqtt.JobResult `json:"-"`
}

// IsComplete returns true if job is in a terminal state
func (j *Job) IsComplete() bool {
	return j.Status == "completed" ||
		j.Status == "failed" ||
		j.Status == "timeout" ||
		j.Status == "cancelled"
}

// Duration returns the job duration if completed
func (j *Job) Duration() time.Duration {
	if j.CompletedAt == nil {
		return time.Since(j.CreatedAt)
	}
	return j.CompletedAt.Sub(j.CreatedAt)
}
