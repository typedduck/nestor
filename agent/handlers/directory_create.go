package handlers

import (
	"fmt"
	"os"
	"strconv"

	"github.com/typedduck/nestor/agent/executor"
)

// DirectoryCreateHandler handles directory creation
type DirectoryCreateHandler struct{}

// NewDirectoryCreateHandler creates a new directory creation handler
func NewDirectoryCreateHandler() *DirectoryCreateHandler {
	return &DirectoryCreateHandler{}
}

// Execute creates a directory with the specified permissions
func (h *DirectoryCreateHandler) Execute(action executor.Action,
	context *executor.ExecutionContext) executor.ActionResult {
	// Extract parameters
	path, ok := action.Params["path"].(string)
	if !ok {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: path",
			Error:   "path parameter not found or invalid",
		}
	}

	// Optional parameters
	modeStr := getStringParam(action.Params, "mode", "0755")
	recursive := getBoolParam(action.Params, "recursive", false)
	owner := getStringParam(action.Params, "owner", "")
	group := getStringParam(action.Params, "group", "")

	// Parse mode
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Invalid mode parameter",
			Error:   fmt.Sprintf("failed to parse mode %s: %v", modeStr, err),
		}
	}

	// Dry run mode
	if context.DryRun {
		recursiveStr := ""
		if recursive {
			recursiveStr = " (recursive)"
		}
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would create directory %s%s", path, recursiveStr),
		}
	}

	// Check if directory already exists
	stat, err := os.Stat(path)
	if err == nil {
		// Directory exists
		if !stat.IsDir() {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: fmt.Sprintf("Path %s exists but is not a directory", path),
				Error:   "file exists",
			}
		}

		// Directory exists, update permissions if needed
		if err := h.setPermissions(path, owner, group, modeStr); err != nil {
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
			Message: fmt.Sprintf("Directory %s already exists", path),
		}
	}

	// Create directory
	if recursive {
		err = os.MkdirAll(path, os.FileMode(mode))
	} else {
		err = os.Mkdir(path, os.FileMode(mode))
	}

	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to create directory %s", path),
			Error:   err.Error(),
		}
	}

	// Set ownership
	if err := h.setPermissions(path, owner, group, modeStr); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: true, // Directory was created but permissions failed
			Message: "Directory created but failed to set permissions",
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Created directory %s", path),
	}
}

// setPermissions sets the owner, group, and mode for a directory
func (h *DirectoryCreateHandler) setPermissions(path, owner, group, modeStr string) error {
	// Set mode
	if modeStr != "" {
		mode, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode: %s", modeStr)
		}
		if err := os.Chmod(path, os.FileMode(mode)); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}

	// Set owner and group
	if owner != "" || group != "" {
		if err := h.chown(path, owner, group); err != nil {
			return fmt.Errorf("chown failed: %w", err)
		}
	}

	return nil
}

// chown changes the owner and group of a directory
func (h *DirectoryCreateHandler) chown(path, owner, group string) error {
	// Get current directory info
	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	// TODO: Implement proper user/group lookup
	// For now, this is a simplified version
	uid := -1
	gid := -1

	if owner != "" {
		if ownerUID, err := strconv.Atoi(owner); err == nil {
			uid = ownerUID
		}
	}

	if group != "" {
		if groupGID, err := strconv.Atoi(group); err == nil {
			gid = groupGID
		}
	}

	if uid != -1 || gid != -1 {
		return os.Chown(path, uid, gid)
	}

	return nil
}
