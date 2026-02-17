package executor

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// Attach attaches to a running or completed agent and retrieves its state
//
// This is used when the SSH connection was lost during execution.
// The agent continues running and saves state after each action.
// The controller can reconnect and retrieve the current state or final results.
func (e *Executor) Attach(host, stateFile string) (*ExecutionResult, error) {
	log.Printf("[INFO ] attaching to agent on %s", host)

	// Parse host
	user, hostname, port, err := parseHost(host)
	if err != nil {
		return nil, fmt.Errorf("invalid host format: %w", err)
	}

	// Connect to remote host
	log.Printf("[INFO ] connecting to %s...", host)
	client, err := e.connectSSH(user, hostname, port)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	// Retrieve state file
	log.Println("[INFO ] retrieving agent state...")
	stateJSON, err := client.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse state
	var result ExecutionResult
	if err := json.Unmarshal([]byte(stateJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse state: %w", err)
	}

	log.Printf("[INFO ] retrieved state for playbook: %s", result.PlaybookID)
	return &result, nil
}

// AttachAndFollow attaches to a running agent and follows execution in real-time
//
// This streams the agent's output as it executes, similar to tail -f.
// Useful for monitoring long-running playbooks.
func (e *Executor) AttachAndFollow(host, stateFile string) error {
	log.Printf("[INFO ] attaching to agent on %s (follow mode)", host)

	// Parse host
	user, hostname, port, err := parseHost(host)
	if err != nil {
		return fmt.Errorf("invalid host format: %w", err)
	}

	// Connect to remote host
	log.Printf("[INFO ] connecting to %s...", host)
	client, err := e.connectSSH(user, hostname, port)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer client.Close()

	log.Println("[INFO ] following agent execution...")
	log.Println()

	// Check if agent is running
	running, err := client.IsAgentRunning()
	if err != nil {
		return fmt.Errorf("failed to check agent status: %w", err)
	}

	if !running {
		// Agent not running, just show final state
		log.Println("[INFO ] agent is not currently running, showing final state:")
		result, err := e.Attach(host, stateFile)
		if err != nil {
			return err
		}
		result.Display()
		return nil
	}

	// Follow execution by polling state file
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastActionCount int

	for {
		<-ticker.C

		// Check if agent still running
		running, err := client.IsAgentRunning()
		if err != nil {
			log.Printf("[WARN ] failed to check agent status: %v", err)
			continue
		}

		// Read current state
		stateJSON, err := client.ReadFile(stateFile)
		if err != nil {
			log.Printf("[WARN ] failed to read state: %v", err)
			continue
		}

		var result ExecutionResult
		if err := json.Unmarshal([]byte(stateJSON), &result); err != nil {
			log.Printf("[WARN ] failed to parse state: %v", err)
			continue
		}

		// Display new actions
		if len(result.Actions) > lastActionCount {
			for i := lastActionCount; i < len(result.Actions); i++ {
				action := result.Actions[i]
				action.Display()
			}
			lastActionCount = len(result.Actions)
		}

		// If agent finished, show summary and exit
		if !running {
			log.Println("======== execution complete ========")
			result.Summary.Display()
			break
		}
	}

	return nil
}
