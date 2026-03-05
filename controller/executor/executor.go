package executor

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/typedduck/nestor/controller/ssh"
	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/util"
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
		workDir:        util.MustExpandPath(config.WorkDir),
		sshKeyPath:     util.MustExpandPath(config.SSHKeyPath),
		signingKeyPath: util.MustExpandPath(config.SigningKeyPath),
		knownHostsPath: util.MustExpandPath(config.KnownHostsPath),
		agentPath:      util.MustExpandPath(config.AgentPath),
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
