package handlers

import (
	"fmt"
	"os"
	"strings"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// --- SymlinkHandler ---

// SymlinkHandler creates or replaces a symbolic link.
type SymlinkHandler struct{}

func NewSymlinkHandler() *SymlinkHandler { return &SymlinkHandler{} }

func (h *SymlinkHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	destination := getStringParam(action.Params, "destination", "")
	if destination == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: destination",
			Error:   "destination parameter not found or empty",
		}
	}

	source := getStringParam(action.Params, "source", "")
	if source == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: source",
			Error:   "source parameter not found or empty",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would create symlink %s -> %s", destination, source),
		}
	}

	cmd := context.Cmd

	// Check if symlink already points to the correct target
	output, _, err := cmd.CombinedOutput("readlink", nil, destination)
	if err == nil {
		current := strings.TrimRight(string(output), "\n")
		if current == source {
			return executor.ActionResult{
				Status:  "success",
				Changed: false,
				Message: fmt.Sprintf("Symlink %s already points to %s", destination, source),
			}
		}
	}

	// Create or replace the symlink
	if err := cmd.Run("ln", nil, "-sf", source, destination); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to create symlink %s -> %s", destination, source),
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Created symlink %s -> %s", destination, source),
	}
}

// --- FileRemoveHandler ---

// FileRemoveHandler removes a file or directory.
type FileRemoveHandler struct{}

func NewFileRemoveHandler() *FileRemoveHandler { return &FileRemoveHandler{} }

func (h *FileRemoveHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	path := getStringParam(action.Params, "path", "")
	if path == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: path",
			Error:   "path parameter not found or empty",
		}
	}

	recursive := getBoolParam(action.Params, "recursive", false)

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would remove %s", path),
		}
	}

	// Idempotency check: skip if path does not exist
	if _, err := context.FS.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return executor.ActionResult{
				Status:  "success",
				Changed: false,
				Message: fmt.Sprintf("Path %s does not exist", path),
			}
		}
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to stat %s", path),
			Error:   err.Error(),
		}
	}

	cmd := context.Cmd
	var args []string
	if recursive {
		args = []string{"-rf", path}
	} else {
		args = []string{"-f", path}
	}

	if err := cmd.Run("rm", nil, args...); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to remove %s", path),
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Removed %s", path),
	}
}
