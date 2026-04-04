package compute

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
}

func NewManager(mqttClient *mqtt.Client, nomadDispatcher *nomad.Dispatcher, config *Config) *Manager {
	if config == nil {
		config = &Config{
			DefaultTimeout: 5 * time.Minute,
		}
	}

	mgr := &Manager{
		mqttClient: mqttClient,
		nomad:      nomadDispatcher,
		config:     config,
	}

	mgr.subscribeToTopics()

	return mgr
}

func (m *Manager) subscribeToTopics() {
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternRequests, m.handleRequest); err != nil {
		log.Fatalf("Failed to subscribe to requests: %v", err)
	}
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternResults, m.handleResult); err != nil {
		log.Fatalf("Failed to subscribe to results: %v", err)
	}
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternStatus, m.handleStatus); err != nil {
		log.Fatalf("Failed to subscribe to status: %v", err)
	}
	if err := m.mqttClient.Subscribe(mqtt.TopicPatternLogs, m.handleLog); err != nil {
		log.Fatalf("Failed to subscribe to logs: %v", err)
	}
}

// handleRequest receives client job requests from compute/request/{client_id}/{correlation_id}.
func (m *Manager) handleRequest(topic string, payload []byte) error {
	// Extract client_id and correlation_id from topic
	// topic format: compute/request/{client_id}/{correlation_id}
	parts := strings.Split(topic, "/")
	if len(parts) != 4 {
		return fmt.Errorf("unexpected request topic format: %s", topic)
	}
	clientID := parts[2]
	correlationID := parts[3]

	var req Request
	if err := json.Unmarshal(payload, &req); err != nil {
		return m.sendError(clientID, correlationID, fmt.Sprintf("invalid request payload: %v", err))
	}
	if req.Operation == "" {
		return m.sendError(clientID, correlationID, "missing required field: operation")
	}

	req.ClientID = clientID
	req.CorrelationID = correlationID

	log.Printf("Request from client=%s correlation=%s operation=%s", clientID, correlationID, req.Operation)

	job, err := m.submit(context.Background(), &req)
	if err != nil {
		return m.sendError(clientID, correlationID, fmt.Sprintf("job submission failed: %v", err))
	}

	// Wait for result in background — client is already subscribed to the response topic
	go func() {
		timeout := m.config.DefaultTimeout
		var response interface{}

		select {
		case result := <-job.resultChan:
			if result.Success {
				response = map[string]interface{}{
					"correlation_id": correlationID,
					"job_id":         job.ID,
					"success":        true,
					"result":         result.Result,
					"duration_ms":    result.DurationMS,
					"worker_id":      result.WorkerID,
				}
			} else {
				response = map[string]interface{}{
					"correlation_id": correlationID,
					"job_id":         job.ID,
					"success":        false,
					"error":          result.Error,
				}
			}
		case <-time.After(timeout):
			response = map[string]interface{}{
				"correlation_id": correlationID,
				"job_id":         job.ID,
				"success":        false,
				"error":          fmt.Sprintf("job timed out after %v", timeout),
			}
		}

		if err := m.mqttClient.Publish(mqtt.TopicResponse(clientID, correlationID), response, false); err != nil {
			log.Printf("Failed to publish response to client=%s correlation=%s: %v", clientID, correlationID, err)
		}
	}()

	return nil
}

func (m *Manager) sendError(clientID, correlationID, msg string) error {
	response := map[string]interface{}{
		"correlation_id": correlationID,
		"success":        false,
		"error":          msg,
	}
	if err := m.mqttClient.Publish(mqtt.TopicResponse(clientID, correlationID), response, false); err != nil {
		log.Printf("Failed to publish error to client=%s correlation=%s: %v", clientID, correlationID, err)
	}
	return fmt.Errorf("%s", msg)
}

func (m *Manager) submit(ctx context.Context, req *Request) (*Job, error) {
	jobID := uuid.New().String()

	job := &Job{
		ID:            jobID,
		Operation:     req.Operation,
		ClientID:      req.ClientID,
		CorrelationID: req.CorrelationID,
		Status:        "pending",
		CreatedAt:     time.Now(),
		resultChan:    make(chan *mqtt.JobResult, 1),
	}

	m.jobs.Store(jobID, job)

	// Publish retained params — worker reads this on startup
	params := make(map[string]interface{})
	for k, v := range req.Payload {
		params[k] = v
	}
	params["job_id"] = jobID

	if err := m.mqttClient.Publish(mqtt.TopicParams(jobID), params, true); err != nil {
		m.jobs.Delete(jobID)
		return nil, fmt.Errorf("mqtt publish params failed: %w", err)
	}

	dispatchResult, err := m.nomad.Dispatch(req.Operation, jobID, nil)
	if err != nil {
		m.jobs.Delete(jobID)
		return nil, fmt.Errorf("nomad dispatch failed: %w", err)
	}

	job.Status = "dispatched"
	job.NomadEvalID = dispatchResult.EvalID
	job.NomadJobID = dispatchResult.DispatchedJobID

	log.Printf("Dispatched job=%s operation=%s nomad_eval=%s", jobID, req.Operation, dispatchResult.EvalID)

	return job, nil
}

func (m *Manager) handleResult(topic string, payload []byte) error {
	var result mqtt.JobResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return fmt.Errorf("unmarshal result failed: %w", err)
	}

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

	log.Printf("Job %s completed: status=%s duration=%dms", result.JobID, job.Status, result.DurationMS)

	if job.resultChan != nil {
		select {
		case job.resultChan <- &result:
		default:
		}
	}

	return nil
}

func (m *Manager) handleStatus(topic string, payload []byte) error {
	var status mqtt.JobStatus
	if err := json.Unmarshal(payload, &status); err != nil {
		return fmt.Errorf("unmarshal status failed: %w", err)
	}

	value, ok := m.jobs.Load(status.JobID)
	if !ok {
		return nil // unknown job, ignore
	}

	job := value.(*Job)
	job.Status = status.Status
	job.Progress = status.Progress

	log.Printf("Job %s status: %s (%.0f%%)", status.JobID, status.Status, status.Progress*100)

	return nil
}

func (m *Manager) handleLog(topic string, payload []byte) error {
	var entry mqtt.JobLog
	if err := json.Unmarshal(payload, &entry); err != nil {
		return fmt.Errorf("unmarshal log failed: %w", err)
	}

	log.Printf("[%s] %s: %s", entry.JobID, entry.Level, entry.Message)
	return nil
}
