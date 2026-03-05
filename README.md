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
- **YAML playbooks** - Write and run playbooks without writing Go code using the `nestor apply` command
- **Controller phases** - `pre:` and `post:` sections run commands, scripts, and file operations on the controller before packaging or after remote completion (build artefacts, fetch secrets, send notifications)
- **Local execution** - Run a playbook on the local machine with `nestor local` — no SSH, no agent, no packaging
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

## The Name

nestor takes its name from King Nestor of Pylos, a figure from Homer's Iliad. Among the Greek leaders at Troy, Nestor stood apart not as the strongest warrior, but as the wisest counselor. He understood that most battles are won not through overwhelming force, but through sound judgment, clear coordination, and knowing which fights are worth having.

This tool aspires to the same pragmatism. It won't cover every edge case in your infrastructure, and it doesn't try to. It covers the 80% -- sensibly, reliably, and without making you learn a new paradigm to get there.

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
- Service management (systemd, sysvinit, openrc)
- Command execution (shell scripts, one-off commands)

### Actions

Actions are the atomic execution units implemented in the agent. They:

- Perform specific system operations (install package, copy file, start service)
- Are idempotent - safe to run multiple times
- Are assembled by modules with appropriate parameters
- Execute in the order defined by the playbook

### Execution Flow

```
┌──────────────────────┐
│  Controller          │
│                      │
│  1. pre: phase       │  ← runs locally on controller (optional)
│     (commands,       │
│      scripts, files) │
│                      │
│  2. Assemble remote  │
│     playbook         │
│                      │
│  3. Sign playbook    │
└──────────┬───────────┘
           │ SSH
           │ scp playbook.tar.gz
           │ ssh sudo ./nestor-agent
           ▼
┌────────────────────┐
│ Remote Host        │
│                    │
│  ┌──────────────┐  │
│  │ Agent        │  │
│  │              │  │
│  │ 1. Verify    │  │
│  │    signature │  │
│  │              │  │
│  │ 2. Validate  │  │
│  │    manifest  │  │
│  │              │  │
│  │ 3. Execute   │  │
│  │    actions   │  │
│  │              │  │
│  │ 4. Report    │  │
│  └──────────────┘  │
└──────────┬─────────┘
           │ SSH (results)
           ▼
┌──────────────────────┐
│  Controller          │
│                      │
│  4. post: phase      │  ← runs locally on controller (optional)
│     (commands,       │     only when remote succeeded
│      scripts, files) │
└──────────────────────┘
```

## Module API Design

Modules in nestor use a fluent, builder-style API to construct provisioning workflows. Here's how the core module types work:

### Playbook Builder

```go
package main

import (
    "github.com/typedduck/nestor/modules"
    "github.com/typedduck/nestor/playbook/builder"
)

func main() {
    // Create a new playbook
    b := builder.New("webserver-setup")

    // Set environment variables available to all actions
    b.SetEnv("APP_VERSION", "1.2.3")
    b.SetEnv("DEPLOY_USER", "appuser")

    // Add actions using module functions
    modules.Package(b, "install", "nginx", "postgresql", "redis")
    modules.File(b, "/etc/nginx/nginx.conf",
        modules.FromTemplate("templates/nginx.conf.tmpl"),
        modules.Owner("root", "root"),
        modules.Mode(0644))

    modules.Service(b, "nginx", "start")

    // b.Playbook() returns the assembled playbook for packaging and transfer
}
```

### Package Module

```go
// Install packages
modules.Package(b, "install", "vim", "git", "htop")

// Remove packages
modules.Package(b, "remove", "apache2")

// Update package cache
modules.Package(b, "update")

// Upgrade all packages
modules.Package(b, "upgrade")
```

The Package module generates actions that:
- Detect the package manager (apt, yum, dnf, brew)
- Update the cache when needed
- Install or remove packages idempotently

### File Module

```go
// Simple file content
modules.File(b, "/etc/motd",
    modules.Content("Welcome to the server\n"))

// From template with variables
modules.File(b, "/etc/app/config.yml",
    modules.FromTemplate("config.yml.tmpl"),
    modules.TemplateVars(map[string]string{
        "DBHost": "db.example.com",
        "DBPort": "5432",
    }))

// Upload local file
modules.File(b, "/usr/local/bin/myapp",
    modules.FromFile("./build/myapp"),
    modules.Mode(0755))

// Directory creation
modules.Directory(b, "/var/app/data",
    modules.Owner("appuser", "appgroup"),
    modules.Mode(0750),
    modules.Recursive(true))

// Symlink
modules.Symlink(b,
    "/etc/nginx/sites-enabled/myapp",
    "/etc/nginx/sites-available/myapp")

// Remove file or directory
modules.Remove(b, "/opt/webapp-old", modules.Recursive(true))
```

Templates use Go's `text/template` syntax. Undefined variables cause execution to fail immediately, surfacing errors early.

### Service Module

```go
// Start a service (no-op if already running)
modules.Service(b, "nginx", "start")

// Stop a service (no-op if already stopped)
modules.Service(b, "apache2", "stop")

// Reload service configuration (always acts)
modules.Service(b, "nginx", "reload")

// Restart a service (always acts)
modules.Service(b, "postgresql", "restart")
```

The Service module detects the init system (systemd, sysvinit, openrc) and executes the appropriate command.

### Command Module

```go
// Run a shell command via /bin/sh -c
modules.Command(b, "echo 'Setup complete' >> /var/log/setup.log")

// With a working directory
modules.Command(b, "make install", modules.Chdir("/opt/src/app"))

// Idempotent: skip if the given path already exists
modules.Command(b, "useradd -m deploy", modules.Creates("/home/deploy"))

// With extra environment variables
modules.Command(b, "make install",
    modules.Chdir("/opt/src/app"),
    modules.CommandEnv("DESTDIR=/opt", "PREFIX=/usr/local"))

// Execute a script from the playbook bundle
modules.Script(b, "scripts/database-migration.sh")

// Script with arguments and idempotency guard
modules.Script(b, "scripts/setup.sh",
    modules.ScriptArgs("--verbose"),
    modules.ScriptCreates("/etc/app/setup.done"))
```

The following options are planned for a future release:

```go
// Run command as a different user (planned)
modules.Command(b, "npm install", modules.RunAs("appuser"))

// Skip command if shell condition is true (planned)
modules.Command(b, "systemctl restart app",
    modules.Unless("systemctl is-active --quiet app"))

// Run command only if shell condition is true (planned)
modules.Command(b, "bundle install",
    modules.OnlyIf("test -f /opt/app/Gemfile"))
```

### Advanced Module Example

```go
package main

import (
    "log"

    "github.com/typedduck/nestor/controller/executor"
    "github.com/typedduck/nestor/modules"
    "github.com/typedduck/nestor/playbook/builder"
)

func deployWebApp(b *builder.Builder, version string) {
    appDir := "/opt/webapp"

    // Install dependencies
    modules.Package(b, "install", "nginx", "postgresql-client", "redis-tools")

    // Create application directory
    modules.Directory(b, appDir, modules.Mode(0755))

    // Deploy application binary
    modules.File(b, appDir+"/webapp",
        modules.FromFile("./build/webapp-"+version),
        modules.Mode(0755))

    // Deploy configuration from template
    modules.File(b, appDir+"/config.toml",
        modules.FromTemplate("config.toml.tmpl"),
        modules.TemplateVars(map[string]string{
            "Version": version,
            "DataDir": appDir + "/data",
        }),
        modules.Mode(0640))

    // Setup systemd service
    modules.File(b, "/etc/systemd/system/webapp.service",
        modules.FromTemplate("webapp.service.tmpl"))

    modules.Command(b, "systemctl daemon-reload")

    // Configure nginx reverse proxy
    modules.File(b, "/etc/nginx/sites-available/webapp",
        modules.FromTemplate("nginx-webapp.conf.tmpl"))

    modules.Symlink(b,
        "/etc/nginx/sites-enabled/webapp",
        "/etc/nginx/sites-available/webapp")

    // Start services
    modules.Service(b, "nginx", "reload")
    modules.Service(b, "webapp", "restart")
}

func main() {
    b := builder.New("webapp-deployment")
    b.SetEnv("ENVIRONMENT", "production")

    deployWebApp(b, "v1.2.3")

    err := executor.Deploy(&executor.Deployment{Remote: b.Playbook()}, "deploy@app-server-01.example.com", &executor.Config{
        SSHKeyPath: "~/.ssh/deploy_key",
    })
    if err != nil {
        log.Fatalf("deployment failed: %v", err)
    }
}
```

## YAML Playbooks

In addition to the programmatic Go API, nestor supports a declarative YAML format. YAML playbooks are ideal for operations engineers who want to write and run provisioning tasks without a Go toolchain.

### Format Overview

```yaml
name: webserver-setup

environment:
  ENVIRONMENT: production

vars:
  app_port: "8080"
  db_host: db.example.com

# Optional: runs on the controller before packaging (command, script, file only)
pre:
  - command: make build
  - command:
      run: ./fetch-secrets.sh
      creates: secrets/db.env

actions:
  - package: update           # short form (update/upgrade)
  - package: upgrade
  - package:
      install: [nginx, vim]
  - package:
      remove: [apache2]

  - file:
      path: /etc/motd
      content: "Welcome\n"
      mode: "0644"
      owner: root
      group: root

  - file:
      path: /opt/app/config.toml
      template: config.toml.tmpl
      vars:
        DBHost: "${db_host}"
        Port: "${app_port}"
      owner: webapp
      mode: "0640"

  - file:
      path: /opt/app/bin/app
      upload: ./build/app
      mode: "0755"

  - directory:
      path: /opt/app
      owner: webapp
      group: webapp
      mode: "0755"
      recursive: true

  - symlink:
      dest: /etc/nginx/sites-enabled/app
      target: /etc/nginx/sites-available/app

  - remove:
      path: /opt/app-old
      recursive: true

  - command: echo hello        # short form
  - command:
      run: useradd -m deploy
      creates: /home/deploy
      env: [KEY=value]
      chdir: /tmp

  - script:
      source: scripts/setup.sh
      args: [--verbose]
      creates: /etc/setup.done

  - service:
      name: nginx
      action: start

# Optional: runs on the controller after remote succeeds (command, script, file only)
post:
  - command: ./notify-slack.sh deployed ${ENVIRONMENT}
```

### Variable Substitution

Variables defined in the `vars:` section are substituted using `${var_name}` syntax before the YAML is parsed, so they can appear in any string value:

```yaml
vars:
  domain: example.com
  port: "443"

actions:
  - file:
      path: /etc/nginx/conf.d/app.conf
      template: nginx.conf.tmpl
      vars:
        Domain: "${domain}"
        Port: "${port}"
```

Variables passed via `--var` flags on the command line override values from `vars:`.

> **Note:** Quote mode strings (`"0755"`, `"0644"`) to prevent YAML from interpreting them as decimal numbers.

### Controller Phases: `pre:` and `post:`

A playbook may optionally include `pre:` and `post:` sections that run **on the controller** before packaging and after remote completion, respectively.

```yaml
pre:
  - command: make build           # build binary before packaging
  - command:
      run: vault read -field=value secret/db > secrets/db.env
      creates: secrets/db.env     # skip if already fetched

actions:
  - file:
      path: /opt/app/bin/app
      upload: ./build/app         # artifact produced by pre:
      mode: "0755"
  - service:
      name: app
      action: restart

post:
  - command: ./scripts/notify.sh "deployed to ${ENVIRONMENT}"
  - script:
      source: scripts/smoke-test.sh
      args: ["--host", "app.example.com"]
```

**Allowed action kinds in `pre:` and `post:`:** `command`, `script`, `file`.
Actions that only make sense on a remote Linux system — `package`, `service`, `directory`, `symlink`, `remove` — are rejected at load time.

**Execution order:**
1. `pre:` runs on the controller (never in dry-run)
2. Remote playbook is packaged, signed, transferred, and executed
3. `post:` runs on the controller **only if the remote phase succeeded**

**Restrictions:**
- `--dry-run` is rejected when `pre:` or `post:` sections are present
- `nestor local` does not support `pre:` or `post:`; use the `actions:` section only

### `nestor apply` Command

```
nestor apply [options] <playbook.yaml> user@host
```

| Flag | Default | Description |
|---|---|---|
| `-var key=value` | — | Set a playbook variable; may be repeated |
| `-ssh-key path` | `~/.ssh/id_ed25519` | SSH private key for authentication |
| `-signing-key path` | same as `-ssh-key` | Key used to sign the playbook |
| `-known-hosts path` | `~/.ssh/known_hosts` | SSH known_hosts file |
| `-dry-run` | false | Package and sign without deploying. Not supported when `pre:` or `post:` sections are present. |

**Examples:**

```bash
# Apply a playbook
nestor apply webserver.yaml deploy@app01.example.com

# Override a variable at runtime
nestor apply webserver.yaml deploy@app01.example.com \
  -var app_port=9090 \
  -var db_host=db2.example.com

# Package and sign only (no SSH connection)
nestor apply --dry-run webserver.yaml deploy@app01.example.com
```

A complete example playbook is available at [`examples/yaml/webserver.yaml`](examples/yaml/webserver.yaml).

### `nestor local` Command

Run a playbook directly on the local machine — no SSH connection, no agent binary, no packaging or signing. The playbook directory must contain `playbook.yaml`; any upload files are resolved relative to that directory.

```
nestor local [options] <dir>
```

| Flag | Default | Description |
|---|---|---|
| `-var key=value` | — | Set a playbook variable; may be repeated |
| `-dry-run` | false | Show what would be done without making changes |

**Examples:**

```bash
# Run a playbook in the current directory
nestor local .

# Dry-run to preview actions
nestor local --dry-run /path/to/myplaybook

# Override a variable at runtime
nestor local /path/to/myplaybook -var version=1.2.3
```

**Typical directory layout:**

```
myplaybook/
├── playbook.yaml
└── uploads/
    └── myapp.conf
```

**Use cases:**

- Bootstrapping a freshly cloned development environment
- Applying a template repository's provisioning playbook locally
- Testing playbook logic before deploying to a remote host

`nestor local` executes the `actions:` section only. Playbooks that contain `pre:` or `post:` sections are rejected — those sections are intended for controller-side steps that complement a remote deployment, not a local run. All action types in `actions:` work as usual, including package installation via Homebrew on macOS, except `service.*` actions which are skipped (managing system services on a developer's machine is outside the intended scope).

> **Note:** Operations that write to system paths (e.g. `/etc`, `/usr/local`) still require elevated privileges. Run with `sudo` on Linux when needed; on macOS, Homebrew actions run without sudo.

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
      "type": "package.install",
      "params": {
        "packages": ["nginx", "postgresql-client", "redis-tools"],
        "update_cache": true
      }
    },
    ...
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
- `package.install` - Install packages (idempotent: skips already-installed packages)
- `package.remove` - Remove packages (idempotent: skips packages not installed)
- `package.update` - Update package cache
- `package.upgrade` - Upgrade all installed packages

**File Operations:**
- `file.content` - Create file with inline content
- `file.upload` - Upload file from playbook archive
- `file.template` - Render Go `text/template` and write file
- `file.symlink` - Create or replace symbolic link
- `file.remove` - Remove file or directory
- `directory.create` - Create directory (with recursive option)

**Service Management:**
- `service.start` - Start service (no-op if already running)
- `service.stop` - Stop service (no-op if already stopped)
- `service.restart` - Restart service
- `service.reload` - Reload service configuration

**Command Execution:**
- `command.execute` - Run shell command via `/bin/sh -c`
- `script.execute` - Execute script from playbook archive via `/bin/sh`

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
      "message": "Installed 3 packages: nginx, postgresql-client, redis-tools"
    },
    {
      "id": "action-002",
      "status": "success",
      "changed": false,
      "message": "Directory '/opt/webapp' already exists with correct permissions"
    },
    {
      "id": "action-003",
      "status": "success",
      "changed": true,
      "message": "Uploaded upload/webapp-v1.2.3 to /opt/webapp/webapp (8388608 bytes)"
    }
  ],
  "summary": {
    "total": 10,
    "success": 10,
    "failed": 0,
    "skipped": 0,
    "changed": 7
  }
}
```

## Getting Started

### Prerequisites

**Controller:**
- Go 1.24+ (for building)
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

Using YAML (no Go toolchain required):

```yaml
# hello.yaml
name: hello-world

actions:
  - package:
      install: [vim, git, htop]
  - file:
      path: /etc/motd
      content: "Welcome to nestor-managed system\n"
  - service:
      name: ssh
      action: start
```

Or using the Go API:

```go
// example/hello.go
package main

import (
    "log"

    "github.com/typedduck/nestor/controller/executor"
    "github.com/typedduck/nestor/modules"
    "github.com/typedduck/nestor/playbook/builder"
)

func main() {
    b := builder.New("hello-world")

    modules.Package(b, "install", "vim", "git", "htop")
    modules.File(b, "/etc/motd",
        modules.Content("Welcome to nestor-managed system\n"))
    modules.Service(b, "ssh", "start")

    err := executor.Deploy(&executor.Deployment{Remote: b.Playbook()}, "user@remote-host", &executor.Config{
        SSHKeyPath: "~/.ssh/id_ed25519",
    })
    if err != nil {
        log.Fatalf("deployment failed: %v", err)
    }
}
```

3. **Execute the playbook:**

```bash
# YAML playbook
nestor apply hello.yaml user@remote-host

# Go playbook
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

# Reattach, follow execution in real-time, then run the post: phase
nestor attach --follow --playbook deploy.yaml user@remote-host
```

The `--playbook` flag is required to execute the `post:` phase after reattaching. When provided, nestor loads the YAML, and if the remote execution completed successfully, runs the `post:` section on the controller — exactly as it would have during `nestor apply`.

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
- ✅ Package management (apt, yum, dnf, Homebrew on macOS)
- ✅ File operations (content, upload, template, symlink, remove, directory)
- ✅ Service management (systemd, sysvinit, openrc)
- ✅ Command and script execution
- ✅ Dry-run mode
- ✅ YAML playbook format with `nestor apply`
- ✅ Local execution with `nestor local` (no SSH, no agent — runs in-process on the local machine)
- ✅ Controller `pre:` and `post:` phases in YAML playbooks
- ✅ `nestor attach --playbook` to run `post:` phase after reattaching

### Planned

- 📋 User and group management
- 📋 Command options: `RunAs`, `Unless`, `OnlyIf`
- 📋 Plugin system for custom modules
- 📋 Parallel execution across multiple hosts
- 📋 Inventory management
- 📋 Rollback capabilities
- 📋 Comprehensive documentation

## License

nestor is licensed under the [Apache License](https://www.apache.org/licenses/LICENSE-2.0), Version 2.0. See [LICENSE](./LICENSE)
