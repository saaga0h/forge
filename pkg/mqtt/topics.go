package mqtt

import "fmt"

const (
	// TopicPatternResults matches result messages from any job.
	TopicPatternResults = "compute/jobs/+/result"

	// TopicPatternStatus matches status updates from any job.
	TopicPatternStatus = "compute/jobs/+/status"

	// TopicPatternLogs matches log output from any job.
	TopicPatternLogs = "compute/jobs/+/logs"

	// TopicPatternRequests matches all incoming client job requests.
	TopicPatternRequests = "compute/request/+/+"
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

// TopicResponse returns the response topic for a client request.
func TopicResponse(clientID, correlationID string) string {
	return fmt.Sprintf("compute/response/%s/%s", clientID, correlationID)
}
