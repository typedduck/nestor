package handlers

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// FileUploadHandler copies a file from the playbook bundle to the destination.
type FileUploadHandler struct{}

func NewFileUploadHandler() *FileUploadHandler { return &FileUploadHandler{} }

func (h *FileUploadHandler) Execute(action playbook.Action,
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

	destination := getStringParam(action.Params, "destination", "")
	if destination == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: destination",
			Error:   "destination parameter not found or empty",
		}
	}

	owner := getStringParam(action.Params, "owner", "")
	group := getStringParam(action.Params, "group", "")
	mode := getStringParam(action.Params, "mode", "0644")

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would upload %s to %s", source, destination),
		}
	}

	fs := context.FS

	// Read source file from playbook bundle
	sourcePath := filepath.Join(context.PlaybookPath, source)
	data, err := fs.ReadFile(sourcePath)
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to read source file %s", source),
			Error:   err.Error(),
		}
	}

	// Idempotency check: compare destination content if it exists
	existing, err := fs.ReadFile(destination)
	if err == nil && bytes.Equal(existing, data) {
		// File exists with correct content; just update permissions
		if err := setPermissions(fs, destination, owner, group, mode); err != nil {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: "Failed to set permissions",
				Error:   err.Error(),
			}
		}
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: fmt.Sprintf("File %s already matches source", destination),
		}
	} else if err != nil && !os.IsNotExist(err) {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to read destination file %s", destination),
			Error:   err.Error(),
		}
	}

	// Create parent directory if needed
	if err := fs.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Failed to create parent directory",
			Error:   err.Error(),
		}
	}

	// Write destination file
	if err := fs.WriteFile(destination, data, 0644); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to write file %s", destination),
			Error:   err.Error(),
		}
	}

	// Set permissions
	if err := setPermissions(fs, destination, owner, group, mode); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: true, // File was written but permissions failed
			Message: "File uploaded but failed to set permissions",
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Uploaded %s to %s (%d bytes)", source, destination, len(data)),
	}
}
