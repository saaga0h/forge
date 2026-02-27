package nomad

import (
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/nomad/api"
)

type Dispatcher struct {
	client *api.Client
	config *Config
}

type Config struct {
	Address string
	JobName string // Parameterized job name
}

type DispatchResult struct {
	EvalID          string
	DispatchedJobID string
}

func NewDispatcher(config *Config) (*Dispatcher, error) {
	nomadConfig := api.DefaultConfig()
	nomadConfig.Address = config.Address

	client, err := api.NewClient(nomadConfig)
	if err != nil {
		return nil, fmt.Errorf("nomad client creation failed: %w", err)
	}

	// Verify connection
	_, err = client.Status().Leader()
	if err != nil {
		return nil, fmt.Errorf("nomad connection failed: %w", err)
	}

	log.Printf("Connected to Nomad at %s", config.Address)

	return &Dispatcher{
		client: client,
		config: config,
	}, nil
}

func (d *Dispatcher) Dispatch(jobID string, meta map[string]string) (*DispatchResult, error) {
	if meta == nil {
		meta = make(map[string]string)
	}
	meta["job_id"] = jobID

	jobs := d.client.Jobs()

	start := time.Now()
	resp, _, err := jobs.Dispatch(
		d.config.JobName,
		meta,
		nil, // No payload
		"",  // No ID prefix
		&api.WriteOptions{},
	)

	if err != nil {
		return nil, fmt.Errorf("dispatch failed: %w", err)
	}

	duration := time.Since(start)
	log.Printf("Dispatched job %s in %v (EvalID: %s, DispatchedJobID: %s)",
		jobID, duration, resp.EvalID, resp.DispatchedJobID)

	return &DispatchResult{
		EvalID:          resp.EvalID,
		DispatchedJobID: resp.DispatchedJobID,
	}, nil
}

func (d *Dispatcher) GetJobStatus(jobID string) (*JobStatus, error) {
	jobs := d.client.Jobs()

	job, _, err := jobs.Info(jobID, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("job info failed: %w", err)
	}

	status := &JobStatus{
		ID:     *job.ID,
		Name:   *job.Name,
		Status: *job.Status,
		Type:   *job.Type,
	}

	// Get allocations
	allocs, _, err := jobs.Allocations(*job.ID, false, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("allocations query failed: %w", err)
	}

	for _, alloc := range allocs {
		allocStatus := AllocationStatus{
			ID:           alloc.ID,
			NodeID:       alloc.NodeID,
			ClientStatus: alloc.ClientStatus,
			CreateTime:   time.Unix(0, alloc.CreateTime),
		}

		if alloc.DeploymentStatus != nil {
			allocStatus.Healthy = alloc.DeploymentStatus.Healthy
		}

		status.Allocations = append(status.Allocations, allocStatus)
	}

	return status, nil
}

func (d *Dispatcher) GetEvaluation(evalID string) (*EvalStatus, error) {
	evals := d.client.Evaluations()

	eval, _, err := evals.Info(evalID, &api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("eval info failed: %w", err)
	}

	return &EvalStatus{
		ID:                eval.ID,
		Status:            eval.Status,
		StatusDescription: eval.StatusDescription,
		CreateTime:        time.Unix(0, eval.CreateTime),
		ModifyTime:        time.Unix(0, eval.ModifyTime),
	}, nil
}

func (d *Dispatcher) StopJob(jobID string, purge bool) error {
	jobs := d.client.Jobs()

	_, _, err := jobs.Deregister(jobID, purge, &api.WriteOptions{})
	if err != nil {
		return fmt.Errorf("job stop failed: %w", err)
	}

	log.Printf("Stopped job %s (purge: %v)", jobID, purge)
	return nil
}

func (d *Dispatcher) ListNodes() ([]*NodeInfo, error) {
	nodes := d.client.Nodes()

	nodeList, _, err := nodes.List(&api.QueryOptions{})
	if err != nil {
		return nil, fmt.Errorf("nodes list failed: %w", err)
	}

	result := make([]*NodeInfo, 0, len(nodeList))
	for _, node := range nodeList {
		info := &NodeInfo{
			ID:     node.ID,
			Name:   node.Name,
			Status: node.Status,
		}

		// Check for GPU resources
		if node.NodeResources != nil && node.NodeResources.Devices != nil {
			for _, device := range node.NodeResources.Devices {
				if device.Type == "gpu" {
					info.HasGPU = true
					info.GPUCount = len(device.Instances)
					break
				}
			}
		}

		result = append(result, info)
	}

	return result, nil
}
