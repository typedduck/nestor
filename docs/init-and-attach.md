# Nestor Initialization and Reattachment

This document explains how to initialize remote systems and reattach to running agents when SSH connections are lost.

## Overview

Nestor provides robust handling for:

1. **Initialization** - Setting up new remote systems for nestor
2. **Reattachment** - Reconnecting to agents after SSH connection loss
3. **Status Checking** - Monitoring remote agent execution

## CLI Commands

### nestor init

Initialize a remote system for nestor use.

**What it does:**
1. Transfers nestor-agent binary to remote system
2. Installs agent at `/usr/local/bin/nestor-agent` (configurable)
3. Adds controller's public key to authorized_keys
4. Sets up proper permissions

**Usage:**

```bash
# Basic initialization
nestor init user@server.example.com

# With custom options
nestor init \
  --agent ./build/nestor-agent \
  --ssh-key ~/.ssh/deploy_key \
  --remote-path /opt/nestor/agent \
  user@server.example.com
```

**Options:**
- `--agent` - Path to local agent binary (default: `./build/nestor-agent`)
- `--ssh-key` - Path to SSH private key (default: `~/.ssh/id_rsa`)
- `--signing-key` - Path to signing key (default: same as SSH key)
- `--remote-path` - Install path on remote system (default: `/usr/local/bin/nestor-agent`)

**Example:**

```bash
$ nestor init deploy@webserver-01.example.com
Initializing deploy@webserver-01.example.com for nestor...
🔌 Connecting to deploy@webserver-01.example.com...
📦 Installing nestor-agent...
🔐 Adding controller's public key...

✓ Initialization complete!
  - Agent installed at: /usr/local/bin/nestor-agent
  - Controller public key added to authorized_keys
  - System ready to receive playbooks
```

### nestor attach

Attach to a running or completed agent.

**What it does:**
1. Connects to remote system via SSH
2. Reads agent state file
3. Displays current execution status or final results

**Usage:**

```bash
# Attach and show current state
nestor attach user@server.example.com

# Attach and follow execution in real-time
nestor attach --follow user@server.example.com

# Attach and run the post: phase if remote succeeded
nestor attach --playbook deploy.yaml user@server.example.com

# Follow execution, then run the post: phase when done
nestor attach --follow --playbook deploy.yaml user@server.example.com

# Custom state file location
nestor attach --state-file /var/run/nestor.state user@server.example.com
```

**Options:**
- `--ssh-key` - Path to SSH private key (default: `~/.ssh/id_rsa`)
- `--state-file` - Path to state file on remote (default: `/tmp/nestor-agent.state`)
- `--follow` - Follow execution in real-time (like `tail -f`)
- `--playbook` - Playbook YAML file; if set and the remote execution succeeded, the `post:` phase is executed on the controller

**Example (completed execution):**

```bash
$ nestor attach deploy@webserver-01
Attaching to agent on deploy@webserver-01...
Connecting to deploy@webserver-01...
Retrieving agent state...
Retrieved state for playbook: webapp-deployment

━━━ Execution Result ━━━
Playbook: webapp-deployment
Status: completed
Started: 2024-02-14T10:30:15Z
Completed: 2024-02-14T10:32:45Z
Duration: 150.23 seconds

━━━ Actions ━━━
[action-001] package.install: Installed 3 packages [changed]
[action-002] directory.create: Created directory /opt/webapp [changed]
[action-003] file.upload: File uploaded [changed]
[action-004] file.template: File created from template [changed]

━━━ Summary ━━━
Total: 4
  Success: 4
  Failed: 0
  Skipped: 0
  Changed: 4
```

**Example (running execution):**

```bash
$ nestor attach --follow deploy@webserver-01
Attaching to agent on deploy@webserver-01 (follow mode)
Connecting to deploy@webserver-01...
Following agent execution...

✓ [action-001] package.update: Updated package cache [changed]
✓ [action-002] package.install: Installed 2 packages: nginx, postgresql [changed]
✓ [action-003] directory.create: Created directory /opt/webapp [changed]
... (continues as agent executes)

━━━ Execution Complete ━━━
Duration: 67.89 seconds
Total: 8, Success: 8, Changed: 6, Failed: 0
```

### nestor status

Check the status of a remote agent without attaching.

**Usage:**

```bash
nestor status user@server.example.com
```

**Example:**

```bash
$ nestor status deploy@webserver-01
Agent Status on deploy@webserver-01.example.com:
  Status: running
  Playbook: webapp-deployment
  Progress: 3/8 actions
  Running since: 2024-02-14T10:30:15Z
  Duration: 45s
```

## Resilient Execution Flow

### Normal Execution

```
Controller                          Remote Agent
    |                                    |
    |------- Deploy Playbook ----------->|
    |                                    | Executing...
    |<------ Stream Output --------------|
    |                                    | Action 1 ✓
    |                                    | Action 2 ✓
    |                                    | Action 3 ✓
    |<------ Final Results --------------|
    |                                    |
```

### Execution with Connection Loss

```
Controller                          Remote Agent
    |                                    |
    |------- Deploy Playbook ----------->|
    |                                    | Executing...
    |<------ Stream Output --------------|
    |                                    | Action 1 ✓
    |                                    | Save state
    X  SSH CONNECTION LOST               | Action 2 ✓
    .                                    | Save state
    .                                    | Action 3 ✓
    .                                    | Save state
    .                                    | Completed!
    |                                    |
    |------- Reattach ------------------>|
    |<------ Read State File ------------|
    |                                    |
   Shows completed results
    |
   (if --playbook provided and remote succeeded)
    |
   post: phase runs on controller
```

### How State Management Works

The agent uses `nohup` to survive SSH disconnections:

1. **Controller launches agent with `nohup`**
   ```bash
   sudo nohup nestor-agent --playbook=... --state=... --log=... &
   ```
   - Agent runs in background
   - Ignores SIGHUP (hangup signal)
   - Continues even if SSH drops

2. **Agent Saves State After Each Action**
   ```json
   {
     "playbook_id": "deployment",
     "status": "running",
     "actions": [
       {"id": "action-001", "status": "success", "changed": true},
       {"id": "action-002", "status": "success", "changed": true}
     ]
   }
   ```

3. **Agent Detaches on SSH Loss**
   - Continues executing in background
   - Updates state file after each action
   - Writes output to log file
   - Runs to completion

4. **Controller Can Reattach**
   - Reads state file via new SSH connection
   - Sees current progress or final results
   - Can tail log file for output
   - Can check if agent is still running

**See [AGENT_DAEMONIZATION.md](AGENT_DAEMONIZATION.md) for complete technical details.**

## Use Cases

### 1. Initial System Setup

```bash
# First time setting up a server
nestor init deploy@new-server.example.com

# Then deploy your application
go run deploy.go  # Your nestor playbook
```

### 2. Monitoring Long-Running Deployments

```bash
# Start deployment
go run deploy.go &

# In another terminal, monitor progress
nestor attach --follow deploy@server
```

### 3. Recovering from Connection Loss

```bash
# Deploy starts
go run deploy.go
# ... connection drops ...

# Later, check what happened
nestor status deploy@server

# Get full results
nestor attach deploy@server
```

### 4. Recovering with a `post:` Phase

If the original deployment included a `post:` section (e.g. send a notification or run a smoke test), and the SSH connection dropped before the controller could execute it, reattach with `--playbook` to pick up where it left off:

```bash
# Connection dropped during nestor apply — remote agent continued
# Once the agent finishes, attach and run post:
nestor attach --follow --playbook deploy.yaml deploy@server
```

The `post:` phase runs only if the remote execution completed successfully (`status: completed`, no failed actions). If the remote failed, only the execution result is shown.

> **Note:** If you attach multiple times with `--playbook`, the `post:` phase runs each time (provided the remote still reports success). Design `post:` actions to be idempotent where possible.

### 4. Batch Deployments

```bash
# Initialize multiple servers
for server in web-{01..05}; do
    nestor init deploy@$server.example.com &
done
wait

# Deploy to all (in your Go code)
# Each server processes independently
```

## Programmatic Usage

You can also use these features from Go code:

```go
import "github.com/typedduck/nestor/controller/executor"

exec, _ := executor.New(&executor.Config{})

// Initialize a server
exec.Init("deploy@server", "./build/nestor-agent")

// Check status
status, _ := exec.CheckStatus("deploy@server", "/tmp/nestor-agent.state")
fmt.Printf("Status: %s, Progress: %d/%d\n", 
    status.Status, status.CompletedActions, status.TotalActions)

// Attach and get results
result, _ := exec.Attach("deploy@server", "/tmp/nestor-agent.state")
fmt.Printf("Completed: %d success, %d failed\n",
    result.Summary.Success, result.Summary.Failed)
```

## State File Location

Default location: `/tmp/nestor-agent.state`

The state file contains:
- Playbook ID and metadata
- Current execution status
- Results of completed actions
- Summary statistics

**Customizing state file location:**

Agent side:
```bash
sudo nestor-agent --playbook=/tmp/playbook.tar.gz --state=/var/run/nestor.state
```

Controller side:
```bash
nestor attach --state-file=/var/run/nestor.state deploy@server
```

## Security Considerations

### Initialization
- Requires existing SSH access to the remote system
- Adds controller's public key to authorized_keys
- Agent binary transferred with secure permissions (755)

### Reattachment
- Uses standard SSH authentication
- State file readable only by agent user
- No additional permissions required

### Best Practices

1. **Separate SSH Keys**
   ```bash
   # Generate dedicated deploy key
   ssh-keygen -t rsa -b 4096 -f ~/.ssh/nestor_deploy_key
   
   # Use in nestor
   nestor init --ssh-key ~/.ssh/nestor_deploy_key deploy@server
   ```

2. **Dedicated Deploy User**
   ```bash
   # On remote system
   sudo useradd -m -s /bin/bash deploy
   sudo usermod -aG sudo deploy
   
   # Initialize
   nestor init deploy@server
   ```

3. **State File Cleanup**
   ```bash
   # Clean up old state files
   ssh deploy@server 'find /tmp -name "nestor-agent.state" -mtime +7 -delete'
   ```

## Troubleshooting

### Init fails with "permission denied"

```bash
# Ensure you have sudo access
ssh user@server 'sudo -v'

# If using a separate deploy user, ensure they're in sudoers
```

### Attach shows "file not found"

```bash
# Check if state file exists
ssh user@server 'ls -l /tmp/nestor-agent.state'

# Check if agent ever ran
ssh user@server 'ps aux | grep nestor-agent'
```

### Follow mode doesn't show updates

```bash
# Check if agent is actually running
nestor status user@server

# Try direct attach instead
nestor attach user@server
```

## Summary

Nestor's initialization and reattachment features provide:

✅ **Easy Setup** - One command initializes remote systems  
✅ **Resilient Execution** - Agent continues if SSH drops  
✅ **Full Recovery** - Reattach to see progress or results  
✅ **Real-time Monitoring** - Follow execution as it happens  
✅ **Status Checking** - Quick status without full attach  

These features make nestor suitable for unreliable network conditions and long-running deployments where maintaining a persistent SSH connection isn't practical.
