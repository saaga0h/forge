package compute

import (
	"gpu-compute-orchestrator/pkg/mqtt"
	"time"
)

// Request represents an incoming client job request.
type Request struct {
	Operation     string                 `json:"operation"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
	ClientID      string                 `json:"-"` // extracted from topic, not payload
	CorrelationID string                 `json:"-"` // extracted from topic, not payload
}

// Job represents a compute job in flight.
type Job struct {
	ID            string                 `json:"job_id"`
	Operation     string                 `json:"operation"`
	ClientID      string                 `json:"client_id"`
	CorrelationID string                 `json:"correlation_id"`
	Status        string                 `json:"status"` // pending, dispatched, starting, computing, completed, failed, timeout, cancelled
	Progress      float64                `json:"progress,omitempty"`
	Result        interface{}            `json:"result,omitempty"`
	Error         string                 `json:"error,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	CompletedAt   *time.Time             `json:"completed_at,omitempty"`
	DurationMS    int64                  `json:"duration_ms,omitempty"`
	NomadEvalID   string                 `json:"nomad_eval_id,omitempty"`
	NomadJobID    string                 `json:"nomad_job_id,omitempty"`
	Metrics       *mqtt.Metrics          `json:"metrics,omitempty"`

	// Internal channel for result notification
	resultChan chan *mqtt.JobResult `json:"-"`
}

// IsComplete returns true if job is in a terminal state.
func (j *Job) IsComplete() bool {
	return j.Status == "completed" ||
		j.Status == "failed" ||
		j.Status == "timeout" ||
		j.Status == "cancelled"
}
