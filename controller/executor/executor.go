package executor

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/typedduck/nestor/controller/ssh"
	"github.com/typedduck/nestor/playbook"
)

// Executor coordinates playbook execution from the controller side
type Executor struct {
	workDir        string
	sshKeyPath     string
	signingKeyPath string
	knownHostsPath string
	agentPath      string
	dryRun         bool
}

// Config holds executor configuration
type Config struct {
	WorkDir        string // Working directory for temporary files
	SSHKeyPath     string // Path to SSH private key for authentication
	SigningKeyPath string // Path to private key for signing playbooks
	KnownHostsPath string // Path to SSH known_hosts file
	AgentPath      string // Path to nestor-agent on remote system
	DryRun         bool   // If true, show what would be done without executing
}

var (
	DefaultWorkDir        string
	DefaultSshKeyPath     string
	DefaultKnownHostsPath string
	DefaultAgentPath      string
)

const (
	SshPort uint16 = 22
)

func InitRemote(host, agentBinaryPath string, config *Config) error {
	exec, err := New(config)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	if err := exec.InitRemote(host, agentBinaryPath); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	return nil
}

// Deployment groups the three execution phases of a nestor apply run.
type Deployment struct {
	Pre         *playbook.Playbook // nil if no pre: section
	Remote      *playbook.Playbook // always required
	Post        *playbook.Playbook // nil if no post: section
	PlaybookDir string             // dir for resolving file paths in pre/post
}

func Deploy(d *Deployment, host string, config *Config) error {
	exec, err := New(config)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	if err := exec.Deploy(d, host); err != nil {
		return fmt.Errorf("deployment failed: %w", err)
	}

	return nil
}

// New creates a new executor with the given configuration
func New(config *Config) (*Executor, error) {
	// Set defaults
	if config.WorkDir == "" {
		config.WorkDir = DefaultWorkDir
	}
	if config.SSHKeyPath == "" {
		config.SSHKeyPath = DefaultSshKeyPath
	}
	if config.SigningKeyPath == "" {
		// Use SSH key for signing by default
		config.SigningKeyPath = config.SSHKeyPath
	}
	if config.KnownHostsPath == "" {
		config.KnownHostsPath = DefaultKnownHostsPath
	}
	if config.AgentPath == "" {
		config.AgentPath = DefaultAgentPath
	}

	// Create work directory
	if err := os.MkdirAll(config.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work directory: %w", err)
	}

	return &Executor{
		workDir:        mustExpandPath(config.WorkDir),
		sshKeyPath:     mustExpandPath(config.SSHKeyPath),
		signingKeyPath: mustExpandPath(config.SigningKeyPath),
		knownHostsPath: mustExpandPath(config.KnownHostsPath),
		agentPath:      mustExpandPath(config.AgentPath),
		dryRun:         config.DryRun,
	}, nil
}

// connectSSH establishes an SSH connection to the remote host
func (e *Executor) connectSSH(user, hostname string, port uint16) (*ssh.Client, error) {
	config := &ssh.Config{
		Host:       hostname,
		User:       user,
		Port:       port,
		KeyPath:    e.sshKeyPath,
		KnownHosts: e.knownHostsPath,
		AgentPath:  e.agentPath,
	}

	return ssh.New(config)
}

var (
	ErrEmptyHost     = errors.New("host string cannot be empty")
	ErrEmptyHostname = errors.New("hostname cannot be empty")
	ErrEmptyPort     = errors.New("port cannot be empty")
	ErrEmptyUsername = errors.New("username cannot be empty")
)

// mustExpandPath calls expandPath and panics if this function returns an error.
func mustExpandPath(path string) string {
	path, err := expandPath(path)
	if err != nil {
		log.Fatalf("[FATAL] failed to expand path: %v", err)
	}
	return path
}

// expandPath cleans the given path and expands a leading tilde (~) with the
// current user's home directory. It returns an error if the path references
// another user's home directory (~username) or if the home directory cannot
// be determined.
//
// Examples:
//   - "~" -> "/home/user"
//   - "~/documents" -> "/home/user/documents"
//   - "~other/file" -> error (other users not supported)
//   - "/tmp/file" -> "/tmp/file" (cleaned)
func expandPath(path string) (string, error) {
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

// parseHost parses an SSH-style host string into its components.
// The expected format is [<username>@]<hostname>[:<port>].
// If username is omitted, it defaults to the current user.
// If port is omitted, it defaults to 22.
//
// Examples:
//
//	parseHost("example.com") -> currentUser, "example.com", 22, nil
//	parseHost("user@example.com") -> "user", "example.com", 22, nil
//	parseHost("example.com:2222") -> currentUser, "example.com", 2222, nil
//	parseHost("user@example.com:2222") -> "user", "example.com", 2222, nil
func parseHost(host string) (username, hostname string, port uint16, err error) {
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

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	workDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	DefaultWorkDir = workDir
	DefaultSshKeyPath = filepath.Join(homeDir, ".ssh", "id_ed25519")
	DefaultKnownHostsPath = filepath.Join(homeDir, ".ssh", "known_hosts")
	DefaultAgentPath = filepath.Join(homeDir, ".local", "bin", "nestor-agent")
}
