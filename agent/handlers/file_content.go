package handlers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/typedduck/nestor/agent/executor"
)

// FileContentHandler handles creating files with inline content
type FileContentHandler struct{}

// NewFileContentHandler creates a new file content handler
func NewFileContentHandler() *FileContentHandler {
	return &FileContentHandler{}
}

// Execute creates a file with the specified content
func (h *FileContentHandler) Execute(action executor.Action,
	context *executor.ExecutionContext) executor.ActionResult {
	// Extract parameters
	destination, ok := action.Params["destination"].(string)
	if !ok {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: destination",
			Error:   "destination parameter not found or invalid",
		}
	}

	content, ok := action.Params["content"].(string)
	if !ok {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: content",
			Error:   "content parameter not found or invalid",
		}
	}

	// Optional parameters
	owner := getStringParam(action.Params, "owner", "")
	group := getStringParam(action.Params, "group", "")
	mode := getStringParam(action.Params, "mode", "0644")

	// Dry run mode
	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would create file %s with %d bytes of content",
				destination, len(content)),
		}
	}

	fs := context.FS

	// Check if file exists and has same content
	exists, sameContent, err := h.checkFile(fs, destination, content)
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to check file %s", destination),
			Error:   err.Error(),
		}
	}

	if exists && sameContent {
		// File exists with correct content, just update permissions if needed
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
			Message: fmt.Sprintf("File %s already exists with correct content", destination),
		}
	}

	// Create parent directory if it doesn't exist
	if err := fs.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Failed to create parent directory",
			Error:   err.Error(),
		}
	}

	// Write the file
	if err := fs.WriteFile(destination, []byte(content), 0644); err != nil {
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
			Changed: true, // File was created but permissions failed
			Message: "File created but failed to set permissions",
			Error:   err.Error(),
		}
	}

	action_verb := "created"
	if exists {
		action_verb = "updated"
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("File %s %s (%d bytes)", destination, action_verb, len(content)),
	}
}

// checkFile checks if a file exists and has the same content
func (h *FileContentHandler) checkFile(fs executor.FileSystem,
	path, content string) (exists bool, sameContent bool, err error) {
	data, err := fs.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}

	// File exists, check if content is the same
	return true, string(data) == content, nil
}
