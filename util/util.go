package util

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	ErrEmptyHost     = errors.New("host string cannot be empty")
	ErrEmptyHostname = errors.New("hostname cannot be empty")
	ErrEmptyPort     = errors.New("port cannot be empty")
	ErrEmptyUsername = errors.New("username cannot be empty")
)

// MustExpandPath calls expandPath and panics if this function returns an error.
func MustExpandPath(path string) string {
	path, err := ExpandPath(path)
	if err != nil {
		log.Fatalf("[FATAL] failed to expand path: %v", err)
	}
	return path
}

// ExpandPath cleans the given path and expands a leading tilde (~) with the
// current user's home directory. It returns an error if the path references
// another user's home directory (~username) or if the home directory cannot
// be determined.
//
// Examples:
//   - "~" -> "/home/user"
//   - "~/documents" -> "/home/user/documents"
//   - "~other/file" -> error (other users not supported)
//   - "/tmp/file" -> "/tmp/file" (cleaned)
func ExpandPath(path string) (string, error) {
	// Handle empty path
	if path == "" {
		return "", nil
	}

	// If path doesn't start with ~, just clean and return
	if !strings.HasPrefix(path, "~") {
		return filepath.Clean(path), nil
	}

	// Get the home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Handle exactly "~"
	if path == "~" {
		return home, nil
	}

	// Check if it's "~<separator>..." (current user's home)
	if strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		// Replace ~ with home directory
		expanded := filepath.Join(home, path[2:])
		return filepath.Clean(expanded), nil
	}

	// Any other pattern starting with ~ is treated as another user's home
	return "", errors.New("expanding other users' home directories is not supported")
}

// ParseHost parses an SSH-style host string into its components.
// The expected format is [<username>@]<hostname>[:<port>].
// If username is omitted, it defaults to the current user.
// If port is omitted, it defaults to 22.
//
// Examples:
//
//	ParseHost("example.com") -> currentUser, "example.com", 22, nil
//	ParseHost("user@example.com") -> "user", "example.com", 22, nil
//	ParseHost("example.com:2222") -> currentUser, "example.com", 2222, nil
//	ParseHost("user@example.com:2222") -> "user", "example.com", 2222, nil
func ParseHost(host string) (username, hostname string, port uint16, err error) {
	if host == "" {
		return "", "", 0, ErrEmptyHost
	}

	// Default values
	user, err := user.Current()
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to retrieve current user: %w", err)
	}
	username = user.Username
	port = 22
	remaining := host

	// Extract username if present (everything before @)
	if atIndex := strings.Index(remaining, "@"); atIndex != -1 {
		username = remaining[:atIndex]
		remaining = remaining[atIndex+1:]

		if username == "" {
			return "", "", 0, ErrEmptyUsername
		}
	}

	// Extract port if present (everything after :)
	if colonIndex := strings.LastIndex(remaining, ":"); colonIndex != -1 {
		hostname = remaining[:colonIndex]
		portStr := remaining[colonIndex+1:]

		if portStr == "" {
			return "", "", 0, ErrEmptyPort
		}

		parsedPort, err := strconv.Atoi(portStr)
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid port number: %w", err)
		}

		if parsedPort < 1 || parsedPort > 65535 {
			return "", "", 0, fmt.Errorf("port must be between 1 and 65535, got %d", parsedPort)
		}

		port = uint16(parsedPort)
	} else {
		hostname = remaining
	}

	if hostname == "" {
		return "", "", 0, ErrEmptyHostname
	}

	return username, hostname, port, nil
}
