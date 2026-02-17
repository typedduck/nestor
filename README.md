# nestor

**A pragmatic SSH-based provisioning tool for the 80% use case**

nestor is a lightweight infrastructure provisioning and configuration management tool written in Go. It provides a simple, SSH-only approach to server provisioning, configuration management, and application deployment without the complexity of traditional configuration management systems.

## Overview

nestor follows a controller-agent architecture where the controller runs on your local machine and orchestrates provisioning tasks on remote machines through a minimal agent. All communication happens exclusively over SSH, making it easy to integrate into existing infrastructure without opening additional network ports or deploying complex server components.

### Key Features

- **SSH-only communication** - No additional network services or agents to manage
- **Single static binaries** - Both controller and agent are standalone Go executables with no runtime dependencies
- **Resilient execution** - Agent detaches on connection loss and allows controller reconnection to resume or retrieve results
- **Idempotent actions** - Built-in actions are designed to be safely re-runnable
- **Signed playbooks** - Cryptographic verification of playbook integrity and authenticity
- **Simple deployment** - Bootstrap remote systems with a single init command
- **Cross-platform** - Runs on Linux, macOS, and other Unix-like systems
- **80% solution** - Covers most common provisioning needs without the complexity of enterprise tools

## Why nestor?

Unlike heavyweight configuration management tools (Ansible, Puppet, SaltStack), nestor is designed for the 80% use case:

- **Simpler to install** - No Python dependencies, no agent daemons, no master servers
- **Simpler to configure** - Straightforward module system with clear action semantics
- **Faster to get started** - SSH access and sudo privileges are the only requirements
- **Easier to debug** - Direct SSH execution makes troubleshooting transparent

nestor is ideal for:

- Server provisioning after IaaS tools (Terraform, CloudFormation) have created infrastructure
- Configuration management for small to medium deployments
- Application deployment pipelines
- Disaster recovery and system restoration
- Environments where SSH is already the primary access method

## Architecture

nestor consists of three main components:

### Controller

The controller runs on your local machine (or CI/CD server) and:

- Executes modules to assemble provisioning tasks
- Packages actions and resources into signed playbooks
- Transfers playbooks to remote systems via SSH
- Executes the agent on remote systems with root privileges (via sudo)
- Manages connection lifecycle and can reattach to running agents

### Agent

The agent is a single static Go binary deployed to remote systems that:

- Unpacks and validates playbook archives
- Verifies playbook signatures and manifest checksums
- Executes actions sequentially as defined in playbook.json
- Detaches from SSH if the connection is lost
- Reports execution status back to the controller
- Runs with root privileges to perform system-level operations

### Playbook

A playbook is a compressed tar archive containing:

- `playbook.json` - Defines the actions to execute and their environment
- `manifest` - SHA256 checksums of all included files
- `upload/` - Directory containing file resources to be deployed
- Cryptographic signature from the controller's private key

## Core Concepts

### Modules

Modules run on the controller side and assemble the provisioning workflow. They:

- Define what needs to be done (install packages, configure services, deploy files)
- Generate parametrized actions in the correct execution order
- Are extensible via a plugin system (planned feature)

Example module types:
- Package management (apt, yum, dnf)
- File operations (upload, template, permissions)
- Service management (systemd, init (planned feature))
- Command execution (shell scripts, one-off commands)

### Actions

Actions are the atomic execution units implemented in the agent. They:

- Perform specific system operations (install package, copy file, start service)
- Are idempotent - safe to run multiple times
- Are assembled by modules with appropriate parameters
- Execute in the order defined by the playbook

### Execution Flow

```
┌──────────────┐
│  Controller  │
│              │
│  1. Execute  │
│     modules  │
│              │
│  2. Assemble │
│     playbook │
│              │
│  3. Sign     │
│     playbook │
└──────┬───────┘
       │ SSH
       │ scp playbook.tar.gz
       │ ssh sudo ./nestor-agent
       ▼
┌───────────────────┐
│ Remote Host       │
│                   │
│  ┌─────────────┐  │
│  │ Agent       │  │
│  │             │  │
│  │ 1. Verify   │  |
│  │    signature|  |
│  │             │  │
│  │ 2. Validate |  |
│  │    manifest │  |
│  │             │  │
│  │ 3. Execute  |  |
│  │    actions  │  |
│  │             │  │
│  │ 4. Report   │  |
│  └─────────────┘  │
└───────────────────┘
```

## Module API Design

Modules in nestor use a fluent, builder-style API to construct provisioning workflows. Here's how the core module types work:

### Playbook Builder

```go
package main

import (
		"github.com/yourusername/nestor/executor"
    "github.com/yourusername/nestor/modules"
    "github.com/yourusername/nestor/playbook"
)

func main() {
    // Create a new playbook
    pb := playbook.New("webserver-setup")
    
    // Set environment variables available to all actions
    pb.SetEnv("APP_VERSION", "1.2.3")
    pb.SetEnv("DEPLOY_USER", "appuser")
    
    // Add actions using module functions
    modules.Package(pb, "install", "nginx", "postgresql", "redis")
    modules.File(pb, "/etc/nginx/nginx.conf", 
        modules.FromTemplate("templates/nginx.conf.tmpl"),
        modules.Owner("root", "root"),
        modules.Mode(0644))
    
    modules.Service(pb, "nginx", "running", "enabled")
    
    // Execute on remote host
    err := executor.Deploy(pb, "user@webserver-01.example.com", &executor.Config{})
    if err != nil {
        panic(err)
    }
}
```

### Package Module

```go
// Install packages
modules.Package(pb, "install", "vim", "git", "htop")

// Remove packages
modules.Package(pb, "remove", "apache2")

// Update package cache
modules.Package(pb, "update")

// Upgrade all packages
modules.Package(pb, "upgrade")
```

The Package module generates actions like:
- Detect package manager (apt, yum, dnf)
- Update cache if needed
- Install/remove packages idempotently

### File Module

```go
// Simple file content
modules.File(pb, "/etc/motd", 
    modules.Content("Welcome to the server\n"))

// From template with variables
modules.File(pb, "/etc/app/config.yml",
    modules.FromTemplate("config.yml.tmpl"),
    modules.TemplateVars(map[string]string{
        "DBHost": "db.example.com",
        "DBPort": "5432",
    }))

// Upload local file
modules.File(pb, "/usr/local/bin/myapp",
    modules.FromFile("./build/myapp"),
    modules.Mode(0755))

// Directory creation
modules.Directory(pb, "/var/app/data",
    modules.Owner("appuser", "appgroup"),
    modules.Mode(0750),
    modules.Recursive(true))

// Symlink
modules.Symlink(pb, "/etc/nginx/sites-enabled/myapp",
    "/etc/nginx/sites-available/myapp")
```

### Service Module

```go
// Start and enable service
modules.Service(pb, "nginx", "running", "enabled")

// Stop and disable service
modules.Service(pb, "apache2", "stopped", "disabled")

// Reload service (for config changes)
modules.Service(pb, "nginx", "reloaded")

// Restart service
modules.Service(pb, "postgresql", "restarted")
```

The Service module:
- Detects init system (systemd, init.d)
- Manages service state idempotently
- Handles enable/disable at boot

### Command Module

```go
// Simple command execution
modules.Command(pb, "echo 'Setup complete' >> /var/log/setup.log")

// Command with working directory
modules.Command(pb, "npm install",
    modules.WorkDir("/var/app"),
    modules.RunAs("appuser"))

// Script execution
modules.Script(pb, 
    modules.FromFile("scripts/database-migration.sh"),
    modules.Interpreter("/bin/bash"),
    modules.OnlyIf("test -f /var/app/needs-migration"))

// Conditional execution
modules.Command(pb, "systemctl restart app",
    modules.Unless("systemctl is-active app"))
```

### User & Group Module

```go
// Create user
modules.User(pb, "appuser",
    modules.UID(1001),
    modules.Group("appgroup"),
    modules.Home("/home/appuser"),
    modules.Shell("/bin/bash"))

// Create group
modules.Group(pb, "appgroup",
    modules.GID(1001))
```

### Advanced Module Example

```go
package main

import (
		"github.com/yourusername/nestor/executor"
    "github.com/yourusername/nestor/modules"
    "github.com/yourusername/nestor/playbook"
)

func DeployWebApp(pb *playbook.Playbook, version string) {
    appUser := "webapp"
    appDir := "/opt/webapp"
    
    // Setup user and directories
    modules.Group(pb, appUser, modules.GID(2000))
    modules.User(pb, appUser, 
        modules.UID(2000),
        modules.Group(appUser),
        modules.Home(appDir),
        modules.Shell("/bin/false"))
    
    modules.Directory(pb, appDir,
        modules.Owner(appUser, appUser),
        modules.Mode(0755))
    
    // Install dependencies
    modules.Package(pb, "install", 
        "nginx", "postgresql-client", "redis-tools")
    
    // Deploy application binary
    modules.File(pb, appDir + "/webapp",
        modules.FromFile("./build/webapp-" + version),
        modules.Owner(appUser, appUser),
        modules.Mode(0755))
    
    // Deploy configuration
    modules.File(pb, appDir + "/config.toml",
        modules.FromTemplate("config.toml.tmpl"),
        modules.TemplateVars(map[string]string{
            "Version": version,
            "DataDir": appDir + "/data",
        }),
        modules.Owner(appUser, appUser),
        modules.Mode(0640))
    
    // Setup systemd service
    modules.File(pb, "/etc/systemd/system/webapp.service",
        modules.FromTemplate("webapp.service.tmpl"))
    
    modules.Command(pb, "systemctl daemon-reload")
    
    // Configure nginx reverse proxy
    modules.File(pb, "/etc/nginx/sites-available/webapp",
        modules.FromTemplate("nginx-webapp.conf.tmpl"))
    
    modules.Symlink(pb, 
        "/etc/nginx/sites-enabled/webapp",
        "/etc/nginx/sites-available/webapp")
    
    // Start services
    modules.Service(pb, "nginx", "reloaded")
    modules.Service(pb, "webapp", "running", "enabled")
}

func main() {
    pb := playbook.New("webapp-deployment")
    pb.SetEnv("ENVIRONMENT", "production")
    
    DeployWebApp(pb, "v1.2.3")
    
    err := executor.Execute(pb, "deploy@app-server-01.example.com", &executor.Config{})
    if err != nil {
        panic(err)
    }
}
```

## Playbook Structure

When modules assemble actions, they create a playbook archive with the following structure:

### Archive Layout

```
playbook.tar.gz
├── playbook.json          # Action definitions and metadata
├── manifest               # SHA256 checksums of all files
└── upload/                # Files to be transferred
    ├── webapp-v1.2.3
    ├── config.toml.tmpl
    ├── webapp.service.tmpl
    └── nginx-webapp.conf.tmpl
```

### playbook.json Format

```json
{
  "version": "1.0",
  "name": "webapp-deployment",
  "created": "2024-02-14T10:30:00Z",
  "controller": "user@laptop.local",
  "environment": {
    "ENVIRONMENT": "production",
    "APP_VERSION": "v1.2.3"
  },
  "actions": [
    {
      "id": "action-001",
      "type": "group.create",
      "params": {
        "name": "webapp",
        "gid": 2000
      }
    },
    {
      "id": "action-002",
      "type": "user.create",
      "params": {
        "name": "webapp",
        "uid": 2000,
        "group": "webapp",
        "home": "/opt/webapp",
        "shell": "/bin/false"
      }
    },
    {
      "id": "action-003",
      "type": "directory.create",
      "params": {
        "path": "/opt/webapp",
        "owner": "webapp",
        "group": "webapp",
        "mode": "0755",
        "recursive": true
      }
    },
    {
      "id": "action-004",
      "type": "package.install",
      "params": {
        "packages": ["nginx", "postgresql-client", "redis-tools"],
        "update_cache": true
      }
    },
    {
      "id": "action-005",
      "type": "file.upload",
      "params": {
        "source": "upload/webapp-v1.2.3",
        "destination": "/opt/webapp/webapp",
        "owner": "webapp",
        "group": "webapp",
        "mode": "0755"
      }
    },
    {
      "id": "action-006",
      "type": "file.template",
      "params": {
        "source": "upload/config.toml.tmpl",
        "destination": "/opt/webapp/config.toml",
        "variables": {
          "Version": "v1.2.3",
          "DataDir": "/opt/webapp/data"
        },
        "owner": "webapp",
        "group": "webapp",
        "mode": "0640"
      }
    },
    {
      "id": "action-007",
      "type": "file.upload",
      "params": {
        "source": "upload/webapp.service.tmpl",
        "destination": "/etc/systemd/system/webapp.service",
        "owner": "root",
        "group": "root",
        "mode": "0644"
      }
    },
    {
      "id": "action-008",
      "type": "command.execute",
      "params": {
        "command": "systemctl daemon-reload"
      }
    },
    {
      "id": "action-009",
      "type": "file.template",
      "params": {
        "source": "upload/nginx-webapp.conf.tmpl",
        "destination": "/etc/nginx/sites-available/webapp",
        "owner": "root",
        "group": "root",
        "mode": "0644"
      }
    },
    {
      "id": "action-010",
      "type": "file.symlink",
      "params": {
        "source": "/etc/nginx/sites-available/webapp",
        "destination": "/etc/nginx/sites-enabled/webapp"
      }
    },
    {
      "id": "action-011",
      "type": "service.reload",
      "params": {
        "name": "nginx"
      }
    },
    {
      "id": "action-012",
      "type": "service.start",
      "params": {
        "name": "webapp",
        "enabled": true
      }
    }
  ]
}
```

### manifest Format

```
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  playbook.json
5f2b8e5d4c3a1234567890abcdef1234567890abcdef1234567890abcdef1234  upload/webapp-v1.2.3
a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef0  upload/config.toml.tmpl
9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedcba  upload/webapp.service.tmpl
1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef  upload/nginx-webapp.conf.tmpl
```

### Action Types

The agent implements these action types:

**Package Management:**
- `package.install` - Install packages
- `package.remove` - Remove packages
- `package.update` - Update package cache
- `package.upgrade` - Upgrade installed packages

**File Operations:**
- `file.upload` - Upload file from playbook archive
- `file.content` - Create file with inline content
- `file.template` - Render template and create file
- `file.symlink` - Create symbolic link
- `directory.create` - Create directory (with recursive option)
- `file.remove` - Remove file or directory

**Service Management:**
- `service.start` - Start service (with optional enable)
- `service.stop` - Stop service (with optional disable)
- `service.restart` - Restart service
- `service.reload` - Reload service configuration
- `service.enable` - Enable service at boot
- `service.disable` - Disable service at boot

**User & Group Management:**
- `user.create` - Create user account
- `user.remove` - Remove user account
- `group.create` - Create group
- `group.remove` - Remove group

**Command Execution:**
- `command.execute` - Run shell command
- `script.execute` - Execute script from playbook archive

### Execution Result

After execution, the agent reports results back to the controller:

```json
{
  "playbook_id": "webapp-deployment",
  "status": "completed",
  "started": "2024-02-14T10:30:15Z",
  "completed": "2024-02-14T10:32:45Z",
  "duration_seconds": 150,
  "actions": [
    {
      "id": "action-001",
      "status": "success",
      "changed": true,
      "message": "Group 'webapp' created"
    },
    {
      "id": "action-002",
      "status": "success",
      "changed": true,
      "message": "User 'webapp' created"
    },
    {
      "id": "action-003",
      "status": "success",
      "changed": false,
      "message": "Directory '/opt/webapp' already exists with correct permissions"
    },
    {
      "id": "action-004",
      "status": "success",
      "changed": true,
      "message": "Installed 3 packages: nginx, postgresql-client, redis-tools"
    },
    {
      "id": "action-005",
      "status": "success",
      "changed": true,
      "message": "File uploaded to /opt/webapp/webapp"
    }
  ],
  "summary": {
    "total": 12,
    "success": 12,
    "failed": 0,
    "skipped": 0,
    "changed": 8
  }
}
```

## Getting Started

### Prerequisites

**Controller:**
- Go 1.25+ (for building)
- SSH client
- SSH key pair

**Remote systems:**
- Linux operating system
- SSH server running
- User account with sudo privileges

### Installation

#### Build from source

```bash
# Clone the repository
git clone https://github.com/typedduck/nestor.git
cd nestor

# Build controller and agent
make build

# Binaries will be in build/
ls -l build/
# nestor                    (controller)
# nestor-agent              (agent, local system and architecture)
# nestor-agent-linux-amd64  (agent, Linux for AMD64)
# nestor-agent-linux-arm64  (agent, Linux for ARM64)
```

#### Quick Start

1. **Initialize a remote system:**

```bash
# Transfer your SSH public key and install the agent
nestor init user@remote-host
```

This command:
- Adds your SSH public key to the remote user's authorized_keys
- Uploads the nestor-agent binary to the remote system
- Sets up the necessary permissions

2. **Create a simple playbook:**

```go
// example/hello.go
package main

import (
		"github.com/yourusername/nestor/executor"
    "github.com/yourusername/nestor/modules"
    "github.com/yourusername/nestor/playbook"
)

func main() {
    // Create a new playbook
    pb := playbook.New("hello-world")
    
    // Install packages
    modules.Package(pb, "install", "vim", "git", "htop")
    
    // Create a file
    modules.File(pb, "/etc/motd", 
        modules.Content("Welcome to nestor-managed system\n"))
    
    // Ensure SSH service is running and enabled
    modules.Service(pb, "ssh", "running", "enabled")
    
    // Deploy and execute on remote host
    err := executor.Execute(pb, "user@remote-host", &executor.Config{})
    if err != nil {
        panic(err)
    }
}
```

3. **Execute the playbook:**

```bash
go run example/hello.go
```

The controller will:
- Assemble the playbook
- Transfer it to the remote host
- Execute the agent via SSH
- Display execution progress and results

### Reconnecting to a Detached Agent

If the SSH connection drops during execution, the agent continues running:

```bash
# Reattach to see current status or final results
nestor attach user@remote-host
```

## Security Model

nestor implements multiple security layers:

1. **SSH authentication** - All communication uses standard SSH key-based authentication
2. **Playbook signatures** - Each playbook is signed with the controller's private key
3. **Signature verification** - Agent verifies signatures against authorized controller keys
4. **Manifest validation** - SHA256 checksums ensure file integrity
5. **Encrypted transport** - All data transfer happens over SSH tunnels

The remote system must have the controller's public key in its authorized_keys file (established during `nestor init`). The agent validates that playbooks originate from a trusted controller before execution.

## Project Status

**Current:** Proof of Concept

nestor is in active early development. The core architecture is established, but APIs and features may change as the project evolves.

### Implemented

- ✅ Controller-agent architecture
- ✅ SSH-based communication
- ✅ Playbook packaging and transfer
- ✅ Agent detach/reattach capability
- ✅ Playbook signature verification
- ✅ Manifest validation

### Planned

- 🔄 Core action implementations (package, file, service, command)
- 🔄 Standard module library
- 📋 Plugin system for custom modules
- 📋 Parallel execution across multiple hosts
- 📋 Inventory management
- 📋 Dry-run mode
- 📋 Rollback capabilities
- 📋 Comprehensive documentation

## License

nestor is licensed under the [Apache License](https://www.apache.org/licenses/LICENSE-2.0), Version 2.0. See [LICENSE](./LICENSE)
