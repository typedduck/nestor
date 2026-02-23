package executor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/typedduck/nestor/playbook"
)

// Playbook wraps the wire-format playbook with agent-side runtime fields.
type Playbook struct {
	playbook.Playbook
	ExtractPath string `json:"-"` // Path where playbook was extracted
}

// ExecutionResult represents the result of executing a playbook
type ExecutionResult struct {
	PlaybookID      string           `json:"playbook_id"`
	Status          string           `json:"status"` // "completed", "failed", "partial"
	Started         time.Time        `json:"started"`
	Completed       time.Time        `json:"completed"`
	DurationSeconds float64          `json:"duration_seconds"`
	Actions         []ActionResult   `json:"actions"`
	Summary         ExecutionSummary `json:"summary"`
}

// ActionResult represents the result of executing a single action
type ActionResult struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Status  string `json:"status"`  // "success", "failed", "skipped"
	Changed bool   `json:"changed"` // Whether the action made changes
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// ExecutionSummary provides a summary of execution results
type ExecutionSummary struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
	Changed int `json:"changed"`
}

// Handler defines the interface that all action handlers must implement
type Handler interface {
	// Execute runs the action and returns the result
	Execute(action playbook.Action, context *ExecutionContext) ActionResult
}

// ExecutionContext provides context to action handlers
type ExecutionContext struct {
	PlaybookPath string            // Path to extracted playbook
	Environment  map[string]string // Environment variables
	SystemInfo   *Info             // System information
	DryRun       bool              // Whether this is a dry run
	FS           FileSystem        // File system abstraction
	Cmd          CommandRunner     // Command execution abstraction
}

// Engine coordinates the execution of actions
type Engine struct {
	playbook   *Playbook
	systemInfo *Info
	handlers   map[string]Handler
	stateFile  string
	dryRun     bool
	fs         FileSystem
	cmd        CommandRunner
}

// New creates a new execution engine
func New(playbook *Playbook, sysInfo *Info, stateFile string,
	fs FileSystem, cmd CommandRunner) *Engine {
	return &Engine{
		playbook:   playbook,
		systemInfo: sysInfo,
		handlers:   make(map[string]Handler),
		stateFile:  stateFile,
		dryRun:     false,
		fs:         fs,
		cmd:        cmd,
	}
}

// SetDryRun sets whether this is a dry run
func (e *Engine) SetDryRun(dryRun bool) {
	e.dryRun = dryRun
}

// RegisterHandler registers a handler for an action type
func (e *Engine) RegisterHandler(actionType string, handler Handler) {
	e.handlers[actionType] = handler
}

// Execute runs all actions in the playbook
func (e *Engine) Execute() (*ExecutionResult, error) {
	result := &ExecutionResult{
		PlaybookID: e.playbook.Name,
		Status:     "running",
		Started:    time.Now(),
		Actions:    make([]ActionResult, 0, len(e.playbook.Actions)),
		Summary: ExecutionSummary{
			Total: len(e.playbook.Actions),
		},
	}

	// Create execution context
	context := &ExecutionContext{
		PlaybookPath: e.playbook.ExtractPath,
		Environment:  e.playbook.Environment,
		SystemInfo:   e.systemInfo,
		DryRun:       e.dryRun,
		FS:           e.fs,
		Cmd:          e.cmd,
	}

	// Execute each action sequentially
	for i, action := range e.playbook.Actions {
		fmt.Printf("[%d/%d] Executing %s (%s)...\n",
			i+1, len(e.playbook.Actions), action.ID, action.Type)

		actionResult := e.executeAction(action, context)
		result.Actions = append(result.Actions, actionResult)

		// Update summary
		switch actionResult.Status {
		case "success":
			result.Summary.Success++
			if actionResult.Changed {
				result.Summary.Changed++
			}
		case "failed":
			result.Summary.Failed++
		case "skipped":
			result.Summary.Skipped++
		}

		// Print result
		switch actionResult.Status {
		case "success":
			changeStatus := ""
			if actionResult.Changed {
				changeStatus = " [changed]"
			}
			fmt.Printf("  ✓ %s%s\n", actionResult.Message, changeStatus)
		case "failed":
			fmt.Printf("  ✗ %s: %s\n", actionResult.Message, actionResult.Error)
		default:
			fmt.Printf("  - %s [skipped]\n", actionResult.Message)
		}

		// Save state after each action
		if err := e.saveState(result); err != nil {
			fmt.Printf("Warning: Failed to save state: %v\n", err)
		}

		// Stop on first failure (can be made configurable)
		if actionResult.Status == "failed" {
			result.Status = "failed"
			break
		}
	}

	// Finalize result
	result.Completed = time.Now()
	result.DurationSeconds = result.Completed.Sub(result.Started).Seconds()

	if result.Status != "failed" {
		if result.Summary.Failed > 0 {
			result.Status = "partial"
		} else {
			result.Status = "completed"
		}
	}

	return result, nil
}

// executeAction executes a single action
func (e *Engine) executeAction(action playbook.Action, context *ExecutionContext) ActionResult {
	// Find handler for this action type
	handler, exists := e.handlers[action.Type]
	if !exists {
		return ActionResult{
			ID:      action.ID,
			Type:    action.Type,
			Status:  "failed",
			Changed: false,
			Message: "Unknown action type",
			Error:   fmt.Sprintf("no handler registered for action type: %s", action.Type),
		}
	}

	// Execute the handler
	result := handler.Execute(action, context)
	result.ID = action.ID
	result.Type = action.Type

	return result
}

// saveState saves the current execution state to disk
func (e *Engine) saveState(result *ExecutionResult) error {
	if e.stateFile == "" {
		return nil
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return e.fs.WriteFile(e.stateFile, data, 0644)
}

// LoadState loads the execution state from disk (for reattachment)
func LoadState(stateFile string, fs FileSystem) (*ExecutionResult, error) {
	data, err := fs.ReadFile(stateFile)
	if err != nil {
		return nil, err
	}

	var result ExecutionResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
