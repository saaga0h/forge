package mqtt

import "fmt"

const (
	// TopicPatternResults matches result messages from any job.
	TopicPatternResults = "compute/jobs/+/result"

	// TopicPatternStatus matches status updates from any job.
	TopicPatternStatus = "compute/jobs/+/status"

	// TopicPatternLogs matches log output from any job.
	TopicPatternLogs = "compute/jobs/+/logs"
)

// TopicParams returns the params topic for a job (retained, orchestrator → worker).
func TopicParams(jobID string) string {
	return fmt.Sprintf("compute/jobs/%s/params", jobID)
}

// TopicStatus returns the status topic for a job (worker → orchestrator).
func TopicStatus(jobID string) string {
	return fmt.Sprintf("compute/jobs/%s/status", jobID)
}

// TopicResult returns the result topic for a job (worker → orchestrator).
func TopicResult(jobID string) string {
	return fmt.Sprintf("compute/jobs/%s/result", jobID)
}

// TopicLogs returns the logs topic for a job (worker → orchestrator).
func TopicLogs(jobID string) string {
	return fmt.Sprintf("compute/jobs/%s/logs", jobID)
}
