package executor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/typedduck/nestor/util"
)

// AgentStatus represents the status of a remote agent
type AgentStatus struct {
	Status           string
	Message          string
	PlaybookID       string
	TotalActions     int
	CompletedActions int
	SuccessCount     int
	FailedCount      int
	ChangedCount     int
	StartTime        time.Time
	CompletedTime    time.Time
	Duration         float64
}

// CheckStatus checks the status of a remote agent without attaching
func (e *Executor) CheckStatus(host, stateFile string) (*AgentStatus, error) {
	// Parse host
	user, hostname, port, err := util.ParseHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host format: %w", err)
	}

	// Connect to remote host
	client, err := e.connectSSH(user, hostname, port)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Check if agent is running
	running, err := client.IsAgentRunning()
	if err != nil {
		return nil, fmt.Errorf("failed to check agent status: %w", err)
	}

	// Read state file if it exists
	stateJSON, err := client.ReadFile(stateFile)
	if err != nil {
		// No state file - agent never ran or state was cleaned up
		return &AgentStatus{
			Status:  "not_found",
			Message: "No agent state found",
		}, nil
	}

	// Parse state
	var result ExecutionResult
	if err := json.Unmarshal([]byte(stateJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	// Build status
	status := &AgentStatus{
		PlaybookID:       result.PlaybookID,
		TotalActions:     result.Summary.Total,
		CompletedActions: len(result.Actions),
		SuccessCount:     result.Summary.Success,
		FailedCount:      result.Summary.Failed,
		ChangedCount:     result.Summary.Changed,
		StartTime:        result.Started,
		CompletedTime:    result.Completed,
		Duration:         result.DurationSeconds,
	}

	if running {
		status.Status = "running"
	} else {
		status.Status = result.Status
	}

	return status, nil
}
