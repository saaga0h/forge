package nomad

import "time"

type JobStatus struct {
	ID          string
	Name        string
	Status      string // pending, running, dead
	Type        string
	Allocations []AllocationStatus
}

type AllocationStatus struct {
	ID           string
	NodeID       string
	ClientStatus string // pending, running, complete, failed
	Healthy      *bool
	CreateTime   time.Time
}

type EvalStatus struct {
	ID                string
	Status            string // pending, complete, failed, blocked, cancelled
	StatusDescription string
	CreateTime        time.Time
	ModifyTime        time.Time
}

type NodeInfo struct {
	ID       string
	Name     string
	Status   string // ready, down, initializing
	HasGPU   bool
	GPUCount int
}
