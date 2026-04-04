package mqtt

import (
	"time"
)

// JobStatus represents status updates from compute container
type JobStatus struct {
	JobID     string    `json:"job_id"`
	Status    string    `json:"status"` // starting, computing, completed, failed
	Message   string    `json:"message,omitempty"`
	Progress  float64   `json:"progress,omitempty"` // 0.0 to 1.0
	Timestamp time.Time `json:"timestamp"`
}

// JobResult represents computation result from a worker
type JobResult struct {
	JobID      string      `json:"job_id"`
	Success    bool        `json:"success"`
	Result     interface{} `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
	WorkerID   string      `json:"worker_id,omitempty"`
	DurationMS int64       `json:"duration_ms,omitempty"`
	Timestamp  time.Time   `json:"timestamp"`
	Metrics    *Metrics    `json:"metrics,omitempty"`
}

// Metrics provides compute statistics
type Metrics struct {
	GPUUtilization float64 `json:"gpu_utilization_percent,omitempty"`
	MemoryUsedMB   int64   `json:"memory_used_mb,omitempty"`
	FLOPs          int64   `json:"flops,omitempty"`
}

// JobLog represents log entries from compute container
type JobLog struct {
	JobID     string    `json:"job_id"`
	Level     string    `json:"level"` // debug, info, warn, error
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

