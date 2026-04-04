// go-orchestrator/pkg/compute/manager.go
package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"gpu-compute-orchestrator/pkg/mqtt"
	"gpu-compute-orchestrator/pkg/nomad"

	"github.com/google/uuid"
)

type Manager struct {
	mqttClient *mqtt.Client
	nomad      *nomad.Dispatcher
	jobs       sync.Map // map[string]*Job
	config     *Config
}

type Config struct {
	DefaultTimeout time.Duration
	MaxRetries     int
}

func NewManager(mqttClient *mqtt.Client, nomadDispatcher *nomad.Dispatcher, config *Config) *Manager {
	if config == nil {
		config = &Config{
			DefaultTimeout: 5 * time.Minute,
			MaxRetries:     3,
		}
	}

	mgr := &Manager{
		mqttClient: mqttClient,
		nomad:      nomadDispatcher,
		config:     config,
	}

	// Subscribe to result and status topics
	mgr.subscribeToTopics()

	return mgr
}

func (m *Manager) subscribeToTopics() {
	// Subscribe to results
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternResults, m.handleResult); err != nil {
		log.Fatalf("Failed to subscribe to results: %v", err)
	}

	// Subscribe to status updates
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternStatus, m.handleStatus); err != nil {
		log.Fatalf("Failed to subscribe to status: %v", err)
	}

	// Subscribe to logs
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternLogs, m.handleLog); err != nil {
		log.Fatalf("Failed to subscribe to logs: %v", err)
	}
}

func (m *Manager) handleResult(topic string, payload []byte) error {
	log.Printf("Received result payload: %s", string(payload))

	var result mqtt.JobResult
	if err := json.Unmarshal(payload, &result); err != nil {
		log.Printf("Failed to unmarshal result JSON: %v", err)
		log.Printf("Raw payload was: %s", string(payload))
		return fmt.Errorf("unmarshal result failed: %w", err)
	}

	log.Printf("Successfully parsed result: JobID=%s, Success=%v, WorkerID=%s",
		result.JobID, result.Success, result.WorkerID)

	value, ok := m.jobs.Load(result.JobID)
	if !ok {
		return fmt.Errorf("unknown job ID: %s", result.JobID)
	}

	job := value.(*Job)
	now := time.Now()
	if result.Success {
		job.Status = "completed"
	} else {
		job.Status = "failed"
	}
	job.Result = result.Result
	job.Error = result.Error
	job.CompletedAt = &now
	job.DurationMS = result.DurationMS

	if result.Metrics != nil {
		job.Metrics = result.Metrics
	}

	log.Printf("Job %s completed: status=%s duration=%dms",
		result.JobID, job.Status, result.DurationMS)

	// Notify any waiters
	if job.resultChan != nil {
		select {
		case job.resultChan <- &result:
		default:
		}
	}

	return nil
}

func (m *Manager) handleStatus(topic string, payload []byte) error {
	log.Printf("Received status payload: %s", string(payload))

	var status mqtt.JobStatus
	if err := json.Unmarshal(payload, &status); err != nil {
		log.Printf("Failed to unmarshal status JSON: %v", err)
		log.Printf("Raw status payload was: %s", string(payload))
		return fmt.Errorf("unmarshal status failed: %w", err)
	}

	value, ok := m.jobs.Load(status.JobID)
	if !ok {
		log.Printf("Received status for unknown job ID: %s", status.JobID)
		return nil // Ignore unknown jobs
	}

	job := value.(*Job)
	job.Status = status.Status
	job.Progress = status.Progress

	log.Printf("Job %s status: %s (progress: %.1f%%)",
		status.JobID, status.Status, status.Progress*100)

	return nil
}

func (m *Manager) handleLog(topic string, payload []byte) error {
	var logEntry mqtt.JobLog
	if err := json.Unmarshal(payload, &logEntry); err != nil {
		return fmt.Errorf("unmarshal log failed: %w", err)
	}

	log.Printf("[%s] %s: %s", logEntry.JobID, logEntry.Level, logEntry.Message)
	return nil
}

func (m *Manager) Submit(ctx context.Context, req *Request) (*Job, error) {
	jobID := uuid.New().String()

	job := &Job{
		ID:         jobID,
		Operation:  req.Operation,
		Payload:    req.Payload,
		Status:     "pending",
		CreatedAt:  time.Now(),
		resultChan: make(chan *mqtt.JobResult, 1),
	}

	m.jobs.Store(jobID, job)

	// Step 1: Publish params to MQTT (RETAINED)
	// Format: {job_id: "...", ...payload_fields}
	params := make(map[string]interface{})
	for k, v := range req.Payload {
		params[k] = v
	}
	params["job_id"] = jobID

	if err := m.mqttClient.Publish(mqtt.TopicParams(jobID), params, true); err != nil {
		m.jobs.Delete(jobID)
		return nil, fmt.Errorf("mqtt publish params failed: %w", err)
	}

	log.Printf("Published params for job %s (%d payload fields)", jobID, len(req.Payload))

	// Step 2: Dispatch to Nomad
	meta := make(map[string]string)
	if req.Operation != "" {
		meta["operation"] = req.Operation
	}
	if req.Priority != "" {
		meta["priority"] = req.Priority
	}

	dispatchResult, err := m.nomad.Dispatch(jobID, meta)
	if err != nil {
		m.jobs.Delete(jobID)
		return nil, fmt.Errorf("nomad dispatch failed: %w", err)
	}

	job.Status = "dispatched"
	job.NomadEvalID = dispatchResult.EvalID
	job.NomadJobID = dispatchResult.DispatchedJobID

	log.Printf("Dispatched job %s to Nomad (EvalID: %s)", jobID, dispatchResult.EvalID)

	return job, nil
}

func (m *Manager) SubmitAndWait(ctx context.Context, req *Request) (*Job, error) {
	job, err := m.Submit(ctx, req)
	if err != nil {
		return nil, err
	}

	// Wait for result with timeout
	timeout := m.config.DefaultTimeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	select {
	case <-job.resultChan:
		return job, nil
	case <-time.After(timeout):
		job.Status = "timeout"
		return job, fmt.Errorf("job timeout after %v", timeout)
	case <-ctx.Done():
		return job, ctx.Err()
	}
}

func (m *Manager) GetJob(jobID string) (*Job, bool) {
	value, ok := m.jobs.Load(jobID)
	if !ok {
		return nil, false
	}
	return value.(*Job), true
}

func (m *Manager) ListJobs() []*Job {
	jobs := make([]*Job, 0)
	m.jobs.Range(func(key, value interface{}) bool {
		jobs = append(jobs, value.(*Job))
		return true
	})
	return jobs
}

func (m *Manager) CancelJob(jobID string) error {
	value, ok := m.jobs.Load(jobID)
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}

	job := value.(*Job)

	// Stop Nomad job if it was dispatched
	if job.NomadJobID != "" {
		if err := m.nomad.StopJob(job.NomadJobID, false); err != nil {
			return fmt.Errorf("nomad stop failed: %w", err)
		}
	}

	job.Status = "cancelled"
	now := time.Now()
	job.CompletedAt = &now

	log.Printf("Cancelled job %s", jobID)
	return nil
}
