package executor

import (
	"log"
	"time"
)

// ExecutionResult represents the result of playbook execution
// This is returned by Attach() and parsed from the agent's state file
type ExecutionResult struct {
	PlaybookID      string           `json:"playbook_id"`
	Status          string           `json:"status"`
	Started         time.Time        `json:"started"`
	Completed       time.Time        `json:"completed"`
	DurationSeconds float64          `json:"duration_seconds"`
	Actions         []ActionResult   `json:"actions"`
	Summary         ExecutionSummary `json:"summary"`
}

func (result *ExecutionResult) Display() {
	log.Println("======== execution result ========")
	log.Printf("[INFO ] playbook: %s", result.PlaybookID)
	log.Printf("[INFO ] status: %s", result.Status)

	if !result.Started.IsZero() {
		log.Printf("[INFO ] started: %s", result.Started.Format(time.RFC3339))
	}

	if !result.Completed.IsZero() {
		log.Printf("[INFO ] completed: %s", result.Completed.Format(time.RFC3339))
		log.Printf("[INFO ] duration: %.2f seconds", result.DurationSeconds)
	}

	log.Println("======== actions ========")
	for _, action := range result.Actions {
		action.Display()
	}

	result.Summary.Display()
}

// ActionResult represents the result of a single action
type ActionResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`
	Changed bool   `json:"changed"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

func (result *ActionResult) Display() {
	status := "✓"
	switch result.Status {
	case "failed":
		status = "✗"
	case "skipped":
		status = "-"
	}

	changed := ""
	if result.Changed {
		changed = " [changed]"
	}

	log.Printf("[INFO ] %s [%s] %s: %s%s",
		status, result.ID, result.Type, result.Message, changed)

	if result.Error != "" {
		log.Printf("[ERROR] %s", result.Error)
	}
}

// ExecutionSummary provides summary statistics
type ExecutionSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Changed int `json:"changed"`
}

func (summary *ExecutionSummary) Display() {
	log.Println("======== summary ========")
	log.Printf("[INFO ] total: %d", summary.Total)
	log.Printf("[INFO ] success: %d", summary.Success)
	log.Printf("[INFO ] failed: %d", summary.Failed)
	log.Printf("[INFO ] skipped: %d", summary.Skipped)
	log.Printf("[INFO ] changed: %d", summary.Changed)
}
