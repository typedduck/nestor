package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// extractStringSlice converts a params map value that may be []any or []string
// into a []string. Returns nil, false when the type is unrecognised.
func extractStringSlice(v any) ([]string, bool) {
	switch val := v.(type) {
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out, true
	case []string:
		return val, true
	default:
		return nil, false
	}
}

// buildCommandOpts constructs a CommandOpts from optional env/chdir params.
// Returns nil when neither is set.
func buildCommandOpts(params map[string]any) *executor.CommandOpts {
	var env []string
	if v, ok := params["env"]; ok {
		if sl, ok := extractStringSlice(v); ok {
			env = sl
		}
	}
	chdir := getStringParam(params, "chdir", "")

	if len(env) == 0 && chdir == "" {
		return nil
	}
	return &executor.CommandOpts{Env: env, Dir: chdir}
}

// --- CommandExecuteHandler ---

// CommandExecuteHandler runs an arbitrary shell command via /bin/sh -c.
type CommandExecuteHandler struct{}

func NewCommandExecuteHandler() *CommandExecuteHandler { return &CommandExecuteHandler{} }

func (h *CommandExecuteHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	command := getStringParam(action.Params, "command", "")
	if command == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: command",
			Error:   "command parameter not found or empty",
		}
	}

	creates := getStringParam(action.Params, "creates", "")

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would execute: %s", command),
		}
	}

	// Idempotency: skip if the creates path already exists
	if creates != "" {
		if _, err := context.FS.Stat(creates); err == nil {
			return executor.ActionResult{
				Status:  "success",
				Changed: false,
				Message: fmt.Sprintf("Skipping command: %s already exists", creates),
			}
		} else if !os.IsNotExist(err) {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: fmt.Sprintf("Failed to stat creates path %s", creates),
				Error:   err.Error(),
			}
		}
	}

	opts := buildCommandOpts(action.Params)
	output, _, err := context.Cmd.CombinedOutput("/bin/sh", opts, "-c", command)
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Command failed: %s", command),
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Executed: %s", command),
	}
}

// --- ScriptExecuteHandler ---

// ScriptExecuteHandler runs a script from the playbook bundle via /bin/sh.
type ScriptExecuteHandler struct{}

func NewScriptExecuteHandler() *ScriptExecuteHandler { return &ScriptExecuteHandler{} }

func (h *ScriptExecuteHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	source := getStringParam(action.Params, "source", "")
	if source == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: source",
			Error:   "source parameter not found or empty",
		}
	}

	creates := getStringParam(action.Params, "creates", "")

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would execute script: %s", source),
		}
	}

	// Idempotency: skip if the creates path already exists
	if creates != "" {
		if _, err := context.FS.Stat(creates); err == nil {
			return executor.ActionResult{
				Status:  "success",
				Changed: false,
				Message: fmt.Sprintf("Skipping script: %s already exists", creates),
			}
		} else if !os.IsNotExist(err) {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: fmt.Sprintf("Failed to stat creates path %s", creates),
				Error:   err.Error(),
			}
		}
	}

	scriptPath := filepath.Join(context.PlaybookPath, source)

	// Collect optional args
	var args []string
	if v, ok := action.Params["args"]; ok {
		if sl, ok := extractStringSlice(v); ok {
			args = sl
		}
	}

	opts := buildCommandOpts(action.Params)
	cmdArgs := append([]string{scriptPath}, args...)
	output, _, err := context.Cmd.CombinedOutput("/bin/sh", opts, cmdArgs...)
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Script failed: %s", source),
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Executed script: %s", source),
	}
}
