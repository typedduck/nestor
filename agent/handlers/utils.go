package handlers

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"syscall"

	"github.com/typedduck/nestor/agent/executor"
)

// chown changes the owner and group of a file
func chown(fs executor.FileSystem, path, owner, group string) error {
	// Get current file info
	stat, err := fs.Stat(path)
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

	return fs.Chown(path, uid, gid)
}

// computeFileHash computes the SHA256 hash of a file
func computeFileHash(fs executor.FileSystem, path string) (string, error) {
	f, err := fs.Open(path)
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

// getBoolParam gets a boolean parameter with a default value
func getBoolParam(params map[string]any, key string, defaultValue bool) bool {
	if val, ok := params[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return defaultValue
}

// getStringParam gets a string parameter with a default value
func getStringParam(params map[string]any, key, defaultValue string) string {
	if val, ok := params[key]; ok {
		if strVal, ok := val.(string); ok {
			return strVal
		}
	}
	return defaultValue
}

// setPermissions sets the owner, group, and mode for a file
func setPermissions(fs executor.FileSystem,
	path, owner, group, modeStr string) error {
	// Set mode
	if modeStr != "" {
		mode, err := strconv.ParseUint(modeStr, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid mode: %s", modeStr)
		}
		if err := fs.Chmod(path, os.FileMode(mode)); err != nil {
			return fmt.Errorf("chmod failed: %w", err)
		}
	}

	// Set owner and group
	if owner != "" || group != "" {
		if err := chown(fs, path, owner, group); err != nil {
			return fmt.Errorf("chown failed: %w", err)
		}
	}

	return nil
}
