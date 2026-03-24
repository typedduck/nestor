# Nestor Controller Implementation

This document describes the architecture and implementation of the nestor controller, which coordinates playbook packaging, signing, transfer, and execution.

## Overview

The controller is responsible for:

1. Packaging playbooks into signed tar.gz archives
2. Transferring playbooks to remote systems via SSH
3. Executing the agent on remote systems
4. Collecting and displaying results
5. Initializing new remote systems

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ User Code                                                   │
│                                                             │
│  pb := playbook.New("deployment")                           │
│  modules.Package(pb, "install", "nginx")                    │
│  modules.File(pb, "/etc/app.conf", ...)                     │
│  pb.Execute("user@server")                                  │
└────────────────────┬────────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────┐
│ Controller Executor (controller/executor/executor.go)       │
│                                                             │
│  Coordinates the complete execution flow:                   │
│  1. Package playbook                                        │
│  2. Sign playbook                                           │
│  3. Connect via SSH                                         │
│  4. Transfer playbook                                       │
│  5. Execute agent                                           │
│  6. Collect results                                         │
└────────────┬────────────────┬────────────────┬──────────────┘
             │                │                │
             ▼                ▼                ▼
        ┌────────────┐   ┌────────────┐   ┌────────────┐
        │  Packager  │   │   Signer   │   │ SSH Client │
        └────────────┘   └────────────┘   └────────────┘
```

## File Structure

```
controller/
│
├── executor/
│   └── executor.go          # Main coordinator
│
├── packager/
│   └── packager.go          # Creates tar.gz archives
│
├── signer/
│   └── signer.go            # Cryptographic signing
│
└── ssh/
    └── ssh.go               # SSH transfer and execution

cmd/nestor/
└── main.go                  # CLI entry point

examples/
└── controller_example.go    # Usage examples
```

## Components

### 1. Executor (controller/executor/executor.go)

The main coordinator that orchestrates the entire execution flow.

**Key Methods:**

```go
// Execute executes a playbook on a remote host
func (e *Executor) Execute(pb *playbook.Playbook, host string) error

// Init initializes a remote system for use with nestor
func (e *Executor) Init(host, agentBinaryPath string) error
```

**Execution Flow:**

```
0. pre: phase (optional)
   └─ Run commands, scripts, and file operations on the controller
      before packaging (e.g. build artefacts, fetch secrets)

1. Package playbook
   ├─ Create playbook.json
   ├─ Collect upload files
   ├─ Generate manifest with SHA256 checksums
   └─ Create tar.gz archive

2. Sign playbook
   └─ Sign archive with RSA private key

3. Connect to remote host
   └─ Establish SSH connection

4. Transfer playbook
   └─ SCP archive to /tmp on remote host

5. Execute agent
   ├─ Run: sudo nestor-agent --playbook=/tmp/playbook.tar.gz
   ├─ Stream output to console
   └─ Capture exit code

6. Report results
   └─ Display summary and duration

7. post: phase (optional, only on remote success)
   └─ Run commands, scripts, and file operations on the controller
      after remote completion (e.g. notifications, smoke tests)
```

### 2. Packager (controller/packager/packager.go)

Creates playbook archives ready for transfer.

**Key Features:**

- Creates tar.gz archives containing:
  - `playbook.json` - Action definitions
  - `manifest` - SHA256 checksums
  - `upload/` - Files referenced by actions
  
- Automatically collects files from actions:
  - `file.upload` actions → copies local files to upload/
  - `file.template` actions → copies templates to upload/

**Example:**

```go
packager := packager.New("/tmp/nestor")
pkg, err := packager.Package(playbook)

// Result:
// /tmp/nestor/deployment.tar.gz
// ├── playbook.json
// ├── manifest
// └── upload/
//     ├── app-binary
//     └── config.tmpl
```

### 3. Signer (controller/signer/signer.go)

Provides cryptographic signing and verification.

**Key Features:**

- Signs playbook archives with RSA-PSS
- Generates and manages key pairs
- Exports public keys for remote systems

**Signature Process:**

```
1. Compute SHA256 hash of archive
2. Sign hash with RSA private key (RSA-PSS)
3. Write signature to file
```

**Example:**

```go
signer, err := signer.New("/home/user/.ssh/nestor_key")
err = signer.Sign(pkg)

// Public key for remote system
publicKey, err := signer.GetPublicKey()
```

**Key Generation:**

```go
// Generate new key pair for nestor
privateKeyPath, publicKeyPath, err := signer.GenerateKeyPair("/home/user/.nestor")

// privateKeyPath: /home/user/.nestor/nestor_controller_key
// publicKeyPath:  /home/user/.nestor/nestor_controller_key.pub
```

### 4. SSH Client (controller/ssh/ssh.go)

Handles SSH connections, file transfers, and remote execution.

**Key Features:**

- SSH connection management
- SCP file transfer
- Remote command execution
- Agent installation and initialization

**Example:**

```go
client, err := ssh.New(&ssh.Config{
    Host:       "example.com",
    User:       "deploy",
    Port:       22,
    KeyPath:    "/home/user/.ssh/id_rsa",
    KnownHosts: "/home/user/.ssh/known_hosts",
})

// Transfer playbook
err = client.TransferFile("/tmp/playbook.tar.gz", "/tmp/playbook.tar.gz")

// Execute agent
exitCode, output, err := client.ExecuteAgent("/tmp/playbook.tar.gz", false)
```

## Execution Flow

### Standard Deployment

```go
// 1. Create playbook
pb := playbook.New("deployment")
modules.Package(pb, "install", "nginx")
modules.File(pb, "/etc/nginx/nginx.conf", 
    modules.FromTemplate("nginx.conf.tmpl"))

// 2. Execute (this triggers the entire flow)
pb.Execute("user@server")
```

**What happens internally:**

```
Controller:
1. Runs pre: phase locally (if present)
2. Packages playbook into tar.gz
3. Generates manifest with checksums
4. Signs archive with private key
5. Connects to server via SSH
6. Transfers playbook.tar.gz to /tmp
7. Executes: sudo nestor-agent --playbook=/tmp/playbook.tar.gz

Remote Server (Agent):
8. Validates signature and manifest
9. Executes actions sequentially
10. Reports results

Controller:
11. Displays results and exit code
12. Runs post: phase locally (if present and remote succeeded)
```

### Custom Configuration

```go
// Create executor with custom settings
exec, err := executor.New(&executor.Config{
    WorkDir:        "/var/nestor",
    SSHKeyPath:     "/home/user/.ssh/deploy_key",
    SigningKeyPath: "/home/user/.ssh/nestor_signing_key",
    AgentPath:      "/opt/nestor/nestor-agent",
    DryRun:         false,
})

// Execute playbook
err = exec.Execute(pb, "deploy@server")
```

### Initialize Remote System

```go
exec, err := executor.New(&executor.Config{})

// Initialize remote system:
// 1. Transfer agent binary
// 2. Install at /usr/local/bin/nestor-agent
// 3. Add controller's public key to authorized_keys
err = exec.Init("user@newserver", "./build/nestor-agent")
```

## Security

### Key Management

**Controller Side:**

1. **SSH Private Key** - For SSH authentication
   - Location: `~/.ssh/id_rsa` (default)
   - Used by: SSH client for authentication

2. **Signing Private Key** - For signing playbooks
   - Location: Same as SSH key (default) or separate
   - Used by: Signer to create signatures

**Remote Side:**

1. **Authorized Keys** - For SSH authentication
   - Location: `~/.ssh/authorized_keys`
   - Contains: Controller's SSH public key

2. **Trusted Public Keys** - For signature verification
   - Location: Embedded in authorized_keys or separate file
   - Used by: Agent to verify playbook signatures

### Signature Verification Flow

```
Controller:
1. Sign playbook with private key
2. Transfer playbook + signature

Agent:
3. Read controller's public key from authorized_keys
4. Verify signature matches playbook
5. Reject if signature invalid
6. Execute if signature valid
```

## Configuration

### Default Configuration

```go
executor.Config{
    WorkDir:        "/tmp/nestor",
    SSHKeyPath:     "~/.ssh/id_rsa",
    SigningKeyPath: "~/.ssh/id_rsa",  // Same as SSH key
    KnownHostsPath: "~/.ssh/known_hosts",
    AgentPath:      "/usr/local/bin/nestor-agent",
    DryRun:         false,
}
```

### Environment Variables

```bash
# Override default SSH key
export NESTOR_SSH_KEY="/path/to/key"

# Override work directory
export NESTOR_WORK_DIR="/var/nestor"

# Override agent path on remote systems
export NESTOR_AGENT_PATH="/opt/nestor/agent"
```

## API Reference

### Executor

```go
// New creates a new executor
func New(config *Config) (*Executor, error)

// Execute executes a playbook on a remote host
func (e *Executor) Execute(pb *playbook.Playbook, host string) error

// Init initializes a remote system for nestor
func (e *Executor) Init(host, agentBinaryPath string) error
```

### Packager

```go
// New creates a new packager
func New(workDir string) *Packager

// Package creates a playbook archive
func (p *Packager) Package(pb *playbook.Playbook) (*Package, error)
```

### Signer

```go
// New creates a new signer
func New(privateKeyPath string) (*Signer, error)

// Sign signs a playbook package
func (s *Signer) Sign(pkg *packager.Package) error

// GetPublicKey returns the public key
func (s *Signer) GetPublicKey() (string, error)

// GenerateKeyPair generates a new RSA key pair
func GenerateKeyPair(outputDir string) (privateKeyPath, publicKeyPath string, err error)
```

### SSH Client

```go
// New creates a new SSH client
func New(config *Config) (*Client, error)

// TransferFile transfers a file via SCP
func (c *Client) TransferFile(localPath, remotePath string) error

// ExecuteAgent executes the nestor-agent
func (c *Client) ExecuteAgent(playbookPath string, dryRun bool) (exitCode int, output string, err error)

// InstallAgent installs the agent binary
func (c *Client) InstallAgent(localAgentPath string) error

// AddAuthorizedKey adds a public key to authorized_keys
func (c *Client) AddAuthorizedKey(publicKey string) error
```

## Testing

```bash
# Build controller and agent
make build

# Run controller tests
go test ./controller/... -v

# Test with real SSH
# (requires test server)
go run examples/controller/main.go
```

This controller implementation provides a robust foundation for deploying applications with nestor, handling all aspects of packaging, signing, transfer, and remote execution.
