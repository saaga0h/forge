package anchors

import (
	"context"
	"fmt"
	"log"
	"time"

	"gpu-compute-orchestrator/pkg/compute"
	"gpu-compute-orchestrator/pkg/mqtt"
	"gpu-compute-orchestrator/pkg/nomad"
)

// DiffusionConfig contains parameters for diffusion computation
type DiffusionConfig struct {
	MaxIterations        int     `json:"max_iterations"`
	ConvergenceThreshold float64 `json:"convergence_threshold"`
	LearningRate         float64 `json:"learning_rate"`
	UseGPU               bool    `json:"use_gpu"`
}

// DefaultDiffusionConfig returns default configuration
func DefaultDiffusionConfig() DiffusionConfig {
	return DiffusionConfig{
		MaxIterations:        50,
		ConvergenceThreshold: 0.0001,
		LearningRate:         0.05,
		UseGPU:               true,
	}
}

// DiffusionManager handles semantic anchor diffusion jobs
type DiffusionManager struct {
	computeMgr *compute.Manager
	mqttClient *mqtt.Client
	dispatcher *nomad.Dispatcher
}

// NewDiffusionManager creates a new diffusion manager
func NewDiffusionManager(
	computeMgr *compute.Manager,
	mqttClient *mqtt.Client,
	dispatcher *nomad.Dispatcher,
) *DiffusionManager {
	return &DiffusionManager{
		computeMgr: computeMgr,
		mqttClient: mqttClient,
		dispatcher: dispatcher,
	}
}

// SubmitDiffusionJob submits an anchor dataset for diffusion processing
func (dm *DiffusionManager) SubmitDiffusionJob(
	ctx context.Context,
	dataset *AnchorDataset,
	config DiffusionConfig,
) (*compute.Job, error) {

	// Validate dataset
	if len(dataset.Anchors) == 0 {
		return nil, fmt.Errorf("dataset is empty")
	}

	log.Printf("Submitting diffusion job with %d anchors", len(dataset.Anchors))

	// Format dataset as CSV for compute worker
	payload := dataset.FormatAsCSV()

	// Create compute request
	req := &compute.Request{
		Operation: "semantic_diffusion",
		Parameters: map[string]interface{}{
			"anchors":               payload,
			"max_iterations":        config.MaxIterations,
			"convergence_threshold": config.ConvergenceThreshold,
			"learning_rate":         config.LearningRate,
			"use_gpu":               config.UseGPU,
		},
	}

	// Submit job through compute manager
	job, err := dm.computeMgr.Submit(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to submit diffusion job: %w", err)
	}

	log.Printf("Diffusion job submitted: %s", job.ID)

	return job, nil
}

// SubmitDiffusionJobAndWait submits job and waits for completion
func (dm *DiffusionManager) SubmitDiffusionJobAndWait(
	ctx context.Context,
	dataset *AnchorDataset,
	config DiffusionConfig,
	timeout time.Duration,
) (*DiffusionResult, error) {

	job, err := dm.SubmitDiffusionJob(ctx, dataset, config)
	if err != nil {
		return nil, err
	}

	log.Printf("Waiting for diffusion job %s to complete...", job.ID)

	// Wait for completion
	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctxWithTimeout.Done():
			return nil, fmt.Errorf("timeout waiting for job completion")

		case <-ticker.C:
			// Check job status
			currentJob, exists := dm.computeMgr.GetJob(job.ID)
			if !exists {
				return nil, fmt.Errorf("job not found")
			}

			switch currentJob.Status {
			case "completed":
				log.Printf("Job %s completed successfully", job.ID)

				// Parse result
				if currentJob.Result == nil {
					return nil, fmt.Errorf("job completed but no result available")
				}

				resultText, ok := currentJob.Result.(string)
				if !ok {
					return nil, fmt.Errorf("unexpected result type")
				}

				result, err := ParseDiffusionResult(resultText)
				if err != nil {
					return nil, fmt.Errorf("failed to parse result: %w", err)
				}

				result.DurationMs = float64(currentJob.DurationMS)
				return result, nil

			case "failed":
				return nil, fmt.Errorf("job failed: %s", currentJob.Error)

			case "cancelled":
				return nil, fmt.Errorf("job was cancelled")

			default:
				// Still running, continue waiting
				log.Printf("Job %s status: %s", job.ID, currentJob.Status)
			}
		}
	}
}

// ProcessAnchorFile is a convenience function to process an anchor CSV file
func (dm *DiffusionManager) ProcessAnchorFile(
	ctx context.Context,
	filepath string,
	config DiffusionConfig,
	timeout time.Duration,
) (*DiffusionResult, error) {

	log.Printf("Loading anchor data from %s", filepath)

	// Parse CSV
	dataset, err := ParseAnchorCSV(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse anchor file: %w", err)
	}

	stats := dataset.Statistics()
	log.Printf("Loaded dataset: %+v", stats)

	// Submit and wait
	return dm.SubmitDiffusionJobAndWait(ctx, dataset, config, timeout)
}
