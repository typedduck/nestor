package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

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

	// Check if file exists and has same content
	exists, sameContent, err := h.checkFile(destination, content)
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
		if err := h.setPermissions(destination, owner, group, mode); err != nil {
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
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Failed to create parent directory",
			Error:   err.Error(),
		}
	}

	// Write the file
	if err := os.WriteFile(destination, []byte(content), 0644); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to write file %s", destination),
			Error:   err.Error(),
		}
	}

	// Set permissions
	if err := h.setPermissions(destination, owner, group, mode); err != nil {
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
func (h *FileContentHandler) checkFile(path, content string) (exists bool, sameContent bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}

	// File exists, check if content is the same
	return true, string(data) == content, nil
}

// setPermissions sets the owner, group, and mode for a file
func (h *FileContentHandler) setPermissions(path, owner, group, modeStr string) error {
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

// chown changes the owner and group of a file
func (h *FileContentHandler) chown(path, owner, group string) error {
	// Get current file info
	stat, err := os.Stat(path)
	if err != nil {
		return err
	}

	sys := stat.Sys().(*syscall.Stat_t)
	uid := int(sys.Uid)
	gid := int(sys.Gid)

	// Look up owner UID if specified
	if owner != "" {
		// TODO: Implement user lookup
		// For now, assume owner is already a numeric UID
		if ownerUID, err := strconv.Atoi(owner); err == nil {
			uid = ownerUID
		}
	}

	// Look up group GID if specified
	if group != "" {
		// TODO: Implement group lookup
		// For now, assume group is already a numeric GID
		if groupGID, err := strconv.Atoi(group); err == nil {
			gid = groupGID
		}
	}

	return os.Chown(path, uid, gid)
}

// getStringParam gets a string parameter with a default value
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if val, ok := params[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return defaultValue
}

// computeFileHash computes the SHA256 hash of a file
func computeFileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
