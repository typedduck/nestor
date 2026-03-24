# Agent Daemonization and Process Management

This document explains how the nestor agent survives SSH connection loss and continues running in the background.

## The Problem

When you execute a command over SSH, it runs as a child process of the SSH session. When the SSH connection drops:

1. The SSH server sends **SIGHUP** (hangup signal) to all processes in the session
2. By default, processes terminate when they receive SIGHUP
3. The agent would stop mid-execution

This is unacceptable for long-running deployments where network issues are common.

## The Solution: nohup

The controller launches the agent using `nohup` (no hangup), which:

1. **Ignores SIGHUP signals** - Process continues when SSH drops
2. **Detaches from terminal** - No longer tied to SSH session
3. **Redirects output** - Writes to log file instead of SSH stdout
4. **Runs in background** - Returns control to controller immediately

## How It Works

### Controller Execution Command

```bash
sudo nohup /usr/local/bin/nestor-agent \
  --playbook=/tmp/playbook.tar.gz \
  --state=/tmp/playbook.state \
  --log=/tmp/playbook.log \
  </dev/null >/tmp/playbook.log 2>&1 &
```

**Breaking it down:**

- `sudo` - Run with root privileges
- `nohup` - Ignore hangup signals
- `/usr/local/bin/nestor-agent` - The agent binary
- `--playbook=/tmp/playbook.tar.gz` - Playbook to execute
- `--state=/tmp/playbook.state` - Where to save state
- `--log=/tmp/playbook.log` - Where to write logs
- `</dev/null` - Redirect stdin from /dev/null (no input)
- `>/tmp/playbook.log 2>&1` - Redirect stdout and stderr to log
- `&` - Run in background

### What Happens

```
┌─────────────────────────────────────────────────────────────┐
│ Controller                                                  │
│                                                             │
│ 1. SSH connect to remote                                    │
│ 2. Execute: nohup nestor-agent ... &                        │
│ 3. Agent starts as background process                       │
│ 4. Controller verifies agent is running                     │
│ 5. Controller starts tailing log file                       │
└────────────┬────────────────────────────────────────────────┘
             │
             │ SSH session active
             ▼
┌─────────────────────────────────────────────────────────────┐
│ Remote System                                               │
│                                                             │
│ ┌───────────────────────────────────────────────┐           │
│ │ nestor-agent (PID 12345)                      │           │
│ │                                               │           │
│ │ - Running with nohup (ignores SIGHUP)         │           │
│ │ - Detached from SSH session                   │           │
│ │ - Writes to /tmp/playbook.log                 │           │
│ │ - Saves state to /tmp/playbook.state          │           │
│ │                                               │           │
│ │   [Action 1] ✓ Installed nginx                │           │
│ │   [Action 2] ✓ Created directory              │           │
│ │   [Action 3] ✓ Uploaded file                  │           │
│ │   ...                                         │           │
│ └───────────────────────────────────────────────┘           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
             │
             │ SSH connection drops
             X
             .
             . Agent continues running!
             .
             ▼
┌─────────────────────────────────────────────────────────────┐
│ Remote System                                               │
│                                                             │
│ ┌───────────────────────────────────────────────┐           │
│ │ nestor-agent (PID 12345) - STILL RUNNING      │           │
│ │                                               │           │
│ │   [Action 4] ✓ Configured nginx               │           │
│ │   [Action 5] ✓ Started service                │           │
│ │   [Action 6] ✓ Completed!                     │           │
│ │                                               │           │
│ │ State file: Updated after each action         │           │
│ │ Log file: Complete execution log              │           │
│ └───────────────────────────────────────────────┘           │
│                                                             │
└─────────────────────────────────────────────────────────────┘
             │
             │ Controller reconnects
             ▼
┌─────────────────────────────────────────────────────────────┐
│ Controller                                                  │
│                                                             │
│ 1. SSH connect to remote                                    │
│ 2. Read /tmp/playbook.state                                 │
│ 3. Read /tmp/playbook.log                                   │
│ 4. Display complete results                                 │
└─────────────────────────────────────────────────────────────┘
```

## Process Lifecycle

### 1. Agent Startup

```go
// In SSH client
agentCmd := fmt.Sprintf(
    "sudo nohup %s --playbook=%s --state=%s --log=%s </dev/null >%s 2>&1 &",
    c.agentPath,
    playbookPath,
    stateFile,
    logFile,
    logFile,
)

session.Run(agentCmd)
```

The agent:
- Starts as a background process (`&`)
- Detached from controlling terminal (nohup)
- PID saved in process table
- No longer connected to SSH session

### 2. During Execution

The agent:
- Executes actions sequentially
- Writes output to log file
- Saves state after each action
- Continues regardless of SSH status

```json
// State file updated after each action
{
  "playbook_id": "deployment",
  "status": "running",
  "actions": [
    {"id": "action-001", "status": "success", "changed": true},
    {"id": "action-002", "status": "success", "changed": true},
    {"id": "action-003", "status": "running"}
  ]
}
```

### 3. Monitoring (Controller)

The controller can:
- **Tail log file**: Stream execution output in real-time
- **Read state file**: Check current progress
- **Check process**: Verify agent is still running

```go
// Check if agent is running
running, _ := client.IsAgentRunning()  // pgrep -f nestor-agent

// Read current state
state, _ := client.ReadFile("/tmp/playbook.state")

// Follow execution
client.TailLogFile("/tmp/playbook.log", true)
```

### 4. SSH Connection Loss

When SSH drops:
- **Agent**: Continues executing (nohup protection)
- **Log file**: Continues growing
- **State file**: Continues updating
- **Controller**: Can reconnect anytime

### 5. Reattachment

Controller reconnects and:

```go
// Read final state
result, _ := exec.Attach("user@server", "/tmp/playbook.state")

// Display results
for _, action := range result.Actions {
    fmt.Printf("%s: %s\n", action.ID, action.Message)
}
```

## Advantages of This Approach

### 1. No Complex Daemonization Code

- No need for double-fork
- No need for setsid()
- No need for process group management
- `nohup` handles everything

### 2. Standard Unix Tool

- `nohup` is available on all Unix systems
- Well-tested and reliable
- Understood by system administrators

### 3. Clean Logs

- All output goes to log file
- Easy to debug
- Can be rotated like normal logs

### 4. Simple State Management

- State file is just a JSON file
- Easy to read and parse
- No complex IPC needed

## Alternative Approaches (Not Used)

### systemd Service

```bash
# Could use systemd, but adds complexity
sudo systemctl start nestor-agent@playbook.service
```

**Why not:**
- Requires service file deployment
- More complex setup
- Overkill for one-time deployments

### Screen/Tmux

```bash
# Could use screen/tmux
screen -dmS nestor sudo nestor-agent ...
```

**Why not:**
- Not always installed
- Requires screen/tmux knowledge
- More complex than nohup

### Agent Self-Daemonization

```go
// Agent could daemonize itself
if os.Getppid() != 1 {
    // Fork and exit
    // Complex double-fork process
}
```

**Why not:**
- Complex to implement correctly
- Platform-specific
- `nohup` does it better

## Verification

### Check Agent is Running

```bash
# On remote system
ps aux | grep nestor-agent
# Should show process running

pgrep -f nestor-agent
# Should return PID
```

### Check Files

```bash
# Log file should be growing
tail -f /tmp/playbook.log

# State file should be updating
watch -n 1 cat /tmp/playbook.state
```

### Controller Commands

```bash
# Check status
nestor status user@server

# Reattach
nestor attach user@server

# Follow execution
nestor attach --follow user@server
```

## Error Handling

### Agent Fails to Start

```go
// Controller verifies agent started
time.Sleep(500 * time.Millisecond)
running, _ := client.IsAgentRunning()
if !running {
    // Read log for error
    logContent, _ := client.ReadFile(logFile)
    return fmt.Errorf("agent failed to start: %s", logContent)
}
```

### Agent Crashes Mid-Execution

State file shows last completed action:

```json
{
  "status": "running",
  "actions": [
    {"id": "action-001", "status": "success"},
    {"id": "action-002", "status": "success"},
    {"id": "action-003", "status": "failed", "error": "..."}
  ]
}
```

Controller can read this and see what failed.

### Permission Issues

```bash
# nohup requires write permission for log file
# Controller ensures proper location:
/tmp/playbook.log  # World-writable directory
```

## Best Practices

### 1. Clean Up Old Files

```bash
# Remove old logs and state files
find /tmp -name "*.log" -mtime +7 -delete
find /tmp -name "*.state" -mtime +7 -delete
```

### 2. Monitor Disk Space

Long-running agents produce large logs:

```bash
# Rotate logs if needed
if [ $(stat -f%z /tmp/playbook.log) -gt 100000000 ]; then
    mv /tmp/playbook.log /tmp/playbook.log.old
fi
```

### 3. Use Unique Names

Controller generates unique names:

```bash
/tmp/webapp-deployment-20240214-103015.log
/tmp/webapp-deployment-20240214-103015.state
```

Prevents conflicts from concurrent deployments.

## Summary

The nohup approach provides:

✅ **Reliability** - Agent survives SSH drops  
✅ **Simplicity** - Standard Unix tool  
✅ **Transparency** - Easy to understand and debug  
✅ **Compatibility** - Works everywhere  
✅ **Clean logs** - All output in one file  
✅ **State tracking** - JSON file for current status  

This makes nestor robust for real-world deployments where network reliability cannot be guaranteed.
