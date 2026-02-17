package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Client wraps an SSH connection for remote operations
type Client struct {
	host      string
	user      string
	client    *ssh.Client
	keyPath   string
	agentPath string
}

// Config holds SSH client configuration
type Config struct {
	Host           string // hostname or IP
	User           string // SSH username
	Port           uint16 // SSH port (default: 22)
	KeyPath        string // path to SSH private key
	KnownHosts     string // path to known_hosts file
	AgentPath      string // path to nestor-agent on remote system
	ConnectTimeout time.Duration
}

// New creates a new SSH client
func New(config *Config) (*Client, error) {
	// Set defaults
	if config.Port == 0 {
		config.Port = 22
	}
	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}
	if config.AgentPath == "" {
		return nil, errors.New("missing agent path")
	}

	client := &Client{
		host:      fmt.Sprintf("%s:%d", config.Host, config.Port),
		user:      config.User,
		keyPath:   config.KeyPath,
		agentPath: config.AgentPath,
	}

	// Load SSH key
	key, err := loadPrivateKey(config.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load SSH key: %w", err)
	}

	// Load known hosts
	var hostKeyCallback ssh.HostKeyCallback
	if config.KnownHosts != "" {
		hostKeyCallback, err = knownhosts.New(config.KnownHosts)
		if err != nil {
			return nil, fmt.Errorf("failed to load known_hosts: %w", err)
		}
	} else {
		// WARNING: This is insecure and should only be used for development
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	// Create SSH client config
	sshConfig := &ssh.ClientConfig{
		User: config.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         config.ConnectTimeout,
	}

	// Connect
	sshClient, err := ssh.Dial("tcp", client.host, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", client.host, err)
	}

	client.client = sshClient
	return client, nil
}

// Close closes the SSH connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// TransferFile transfers a file to the remote system using SCP
func (c *Client) TransferFile(localPath, remotePath string) error {
	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Create SCP session
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if err := c.RunCommand(fmt.Sprintf("mkdir -p %s", remoteDir)); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Start SCP receive on remote side
	go func() {
		stdin, _ := session.StdinPipe()
		defer stdin.Close()

		// Send file header
		fmt.Fprintf(stdin, "C0644 %d %s\n", stat.Size(), filepath.Base(remotePath))

		// Send file content
		io.Copy(stdin, localFile)

		// Send end of file
		fmt.Fprint(stdin, "\x00")
	}()

	// Execute SCP command
	if err := session.Run(fmt.Sprintf("scp -t %s", remotePath)); err != nil {
		return fmt.Errorf("scp failed: %w", err)
	}

	return nil
}

// RunCommand executes a command on the remote system and returns the output
func (c *Client) RunCommand(command string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Run command
	output, err := session.CombinedOutput(command)
	if err != nil {
		return fmt.Errorf("command failed: %s: %w", string(output), err)
	}

	return nil
}

// RunCommandWithOutput executes a command and returns stdout and stderr
func (c *Client) RunCommandWithOutput(command string) (stdout, stderr string, err error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	err = session.Run(command)
	return stdoutBuf.String(), stderrBuf.String(), err
}

// ExecuteAgent executes the nestor-agent on the remote system
//
// This method:
// 1. Checks if the agent is installed
// 2. Executes the agent with nohup for detachment (survives SSH disconnect)
// 3. Streams output back to the controller
// 4. Returns the exit code
//
// The agent is launched with nohup so it continues running even if the SSH
// connection is lost. Output is redirected to a log file.
func (c *Client) ExecuteAgent(playbookPath string, dryRun bool) (exitCode int, output string, err error) {
	// Check if agent exists
	session, err := c.client.NewSession()
	if err != nil {
		return -1, "", fmt.Errorf("failed to create session: %w", err)
	}

	checkCmd := fmt.Sprintf("test -f %s && test -x %s", c.agentPath, c.agentPath)
	if err := session.Run(checkCmd); err != nil {
		session.Close()
		return -1, "", fmt.Errorf("nestor-agent not found or not executable at %s", c.agentPath)
	}
	session.Close()

	// Determine log file path
	logFile := strings.TrimSuffix(playbookPath, ".tar.gz") + ".log"
	stateFile := strings.TrimSuffix(playbookPath, ".tar.gz") + ".state"

	// Build agent command with nohup for detachment
	// nohup ensures the process continues even if SSH disconnects
	// </dev/null redirects stdin to prevent hanging
	// Output goes to log file for later retrieval
	agentCmd := fmt.Sprintf(
		"sudo nohup %s --playbook=%s --state=%s --log=%s </dev/null >%s 2>&1 &",
		c.agentPath,
		playbookPath,
		stateFile,
		logFile,
		logFile,
	)

	if dryRun {
		agentCmd = fmt.Sprintf(
			"sudo nohup %s --playbook=%s --dry-run --log=%s </dev/null >%s 2>&1 &",
			c.agentPath,
			playbookPath,
			logFile,
			logFile,
		)
	}

	// Execute agent
	session, err = c.client.NewSession()
	if err != nil {
		return -1, "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	if err := session.Run(agentCmd); err != nil {
		return -1, "", fmt.Errorf("failed to start agent: %w", err)
	}

	// Give the agent a moment to start
	time.Sleep(500 * time.Millisecond)

	// Verify agent started successfully
	running, err := c.IsAgentRunning()
	if err != nil {
		return -1, "", fmt.Errorf("failed to verify agent started: %w", err)
	}
	if !running {
		// Agent failed to start, read log for error
		logContent, _ := c.ReadFile(logFile)
		return -1, logContent, fmt.Errorf("agent failed to start, see log: %s", logFile)
	}

	// Return success - agent is running in background
	// Controller can use Attach() or AttachAndFollow() to monitor
	return 0, fmt.Sprintf("Agent started successfully. Log: %s, State: %s", logFile, stateFile), nil
}

// InstallAgent transfers the agent binary to the remote system
func (c *Client) InstallAgent(localAgentPath string) error {
	// Transfer agent
	if err := c.TransferFile(localAgentPath, "/tmp/nestor-agent"); err != nil {
		return fmt.Errorf("failed to transfer agent: %w", err)
	}

	// Install agent with sudo
	installCmd := fmt.Sprintf("sudo mv /tmp/nestor-agent %s && sudo chmod +x %s",
		c.agentPath, c.agentPath)

	if err := c.RunCommand(installCmd); err != nil {
		return fmt.Errorf("failed to install agent: %w", err)
	}

	return nil
}

// AddAuthorizedKey adds a public key to the remote user's authorized_keys
//
// This is used during initialization to add the controller's public key
// so the agent can verify playbook signatures.
func (c *Client) AddAuthorizedKey(publicKey string) error {
	// Ensure .ssh directory exists
	if err := c.RunCommand("mkdir -p ~/.ssh && chmod 700 ~/.ssh"); err != nil {
		return fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Trim whitespace and ensure newline
	publicKey = strings.TrimSpace(publicKey) + "\n"

	// Check if key already exists
	checkCmd := fmt.Sprintf("grep -q '%s' ~/.ssh/authorized_keys 2>/dev/null",
		strings.TrimSpace(publicKey))

	session, _ := c.client.NewSession()
	if err := session.Run(checkCmd); err == nil {
		// Key already exists
		session.Close()
		return nil
	}
	session.Close()

	// Add key to authorized_keys
	addCmd := fmt.Sprintf("echo '%s' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys",
		strings.TrimSpace(publicKey))

	if err := c.RunCommand(addCmd); err != nil {
		return fmt.Errorf("failed to add authorized key: %w", err)
	}

	return nil
}

// ReadFile reads the contents of a file from the remote system
func (c *Client) ReadFile(path string) (string, error) {
	stdout, stderr, err := c.RunCommandWithOutput(fmt.Sprintf("cat %s", path))
	if err != nil {
		if strings.Contains(stderr, "No such file") {
			return "", fmt.Errorf("file not found: %s", path)
		}
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return stdout, nil
}

// IsAgentRunning checks if the nestor-agent process is currently running
func (c *Client) IsAgentRunning() (bool, error) {
	// Check for running nestor-agent process
	cmd := "pgrep -f nestor-agent"
	stdout, _, err := c.RunCommandWithOutput(cmd)

	if err != nil {
		// pgrep returns exit code 1 if no processes found
		// This is not an error, just means agent isn't running
		return false, nil
	}

	// If we got output, agent is running
	return strings.TrimSpace(stdout) != "", nil
}

// TailLogFile tails a log file and streams output to the console
//
// This is used to follow agent execution in real-time.
// It continues until the agent process completes.
func (c *Client) TailLogFile(logPath string, follow bool) error {
	tailCmd := fmt.Sprintf("tail -f %s", logPath)
	if !follow {
		tailCmd = fmt.Sprintf("cat %s", logPath)
	}

	session, err := c.client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Stream output to console
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	// Run tail command
	// This will block until the session is killed or tail exits
	if err := session.Run(tailCmd); err != nil {
		// Ignore errors from tail being interrupted
		if !strings.Contains(err.Error(), "exit status") {
			return fmt.Errorf("tail failed: %w", err)
		}
	}

	return nil
}

// GetLogPath returns the log file path for a playbook
func (c *Client) GetLogPath(playbookPath string) string {
	return strings.TrimSuffix(playbookPath, ".tar.gz") + ".log"
}

// GetStateFilePath returns the state file path for a playbook
func (c *Client) GetStateFilePath(playbookPath string) string {
	return strings.TrimSuffix(playbookPath, ".tar.gz") + ".state"
}

// loadPrivateKey loads an SSH private key from a file
func loadPrivateKey(path string) (ssh.Signer, error) {
	keyData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	key, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}
