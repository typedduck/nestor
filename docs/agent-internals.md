# Agent Internals

This document covers the agent's internal interfaces, types, and CLI. For the high-level architecture and action types see the [README](../README.md). For how the agent survives SSH disconnects see [agent-daemonization.md](agent-daemonization.md).

## Handler Interface

All action handlers implement a single interface defined in `agent/executor/executor.go`:

```go
type Handler interface {
    Execute(action playbook.Action, context *ExecutionContext) ActionResult
}
```

`ExecutionContext` provides everything a handler needs at runtime:

```go
type ExecutionContext struct {
    PlaybookPath string            // Path to extracted playbook archive
    Environment  map[string]string // Environment variables from playbook.json
    SystemInfo   *Info             // Detected system capabilities
    DryRun       bool              // When true: report what would change, don't act
    FS           FileSystem        // File system abstraction (real or test double)
    Cmd          CommandRunner     // Command execution abstraction (real or test double)
}
```

`ActionResult` is what every handler returns:

```go
type ActionResult struct {
    ID      string `json:"id"`
    Type    string `json:"type"`
    Status  string `json:"status"`  // "success", "failed", "skipped"
    Changed bool   `json:"changed"` // true if the action modified system state
    Message string `json:"message"`
    Error   string `json:"error,omitempty"`
}
```

## Handler Registration

`agent/handlers/register.go` contains two registration functions used at startup:

- `RegisterAll(engine)` — registers every handler; used by the remote agent binary
- `RegisterLocal(engine)` — registers all handlers except service handlers; used by `nestor local`

Service handlers are excluded from local execution because managing system services on a developer's machine is outside the intended scope.

Current handler registrations:

```
package.install       → PackageInstallHandler
package.remove        → PackageRemoveHandler
package.update        → PackageUpdateHandler
package.upgrade       → PackageUpgradeHandler
file.content          → FileContentHandler
file.template         → FileTemplateHandler
file.upload           → FileUploadHandler
file.symlink          → SymlinkHandler
file.remove           → FileRemoveHandler
directory.create      → DirectoryCreateHandler
command.execute       → CommandExecuteHandler
script.execute        → ScriptExecuteHandler
service.start         → ServiceStartHandler         (RegisterAll only)
service.stop          → ServiceStopHandler          (RegisterAll only)
service.reload        → ServiceReloadHandler        (RegisterAll only)
service.restart       → ServiceRestartHandler       (RegisterAll only)
service.daemon-reload → ServiceDaemonReloadHandler  (RegisterAll only)
```

## System Detection

At startup the agent calls `executor.DetectSystem()`, which populates an `Info` struct:

```go
type Info struct {
    OS             string // "linux", "darwin"
    Distribution   string // "ubuntu", "debian", "centos", etc.
    PackageManager string // "apt", "yum", "dnf", "brew"
    InitSystem     string // "systemd", "sysvinit", "openrc", "launchctl"
    Architecture   string // "amd64", "arm64"
}
```

Handlers receive this via `ExecutionContext.SystemInfo` and use it to select the right command (e.g. `apt-get` vs `yum`).

## Agent Binary CLI Flags

```
nestor-agent [flags]

  -playbook string
        Path to the playbook archive (default "/tmp/playbook.tar.gz")
  -state string
        Path to the state file for detach/reattach (default "/tmp/nestor-agent.state")
  -log string
        Log file path (default: stderr)
  -dry-run
        Show what would be done without making changes
  -detach-on-error
        Detach on SSH connection error
```

The agent must run as root (`sudo`). It exits non-zero if any action fails.

## State File

The agent writes a JSON state file after each completed action. This allows the controller to read progress or final results after reattaching.

```json
{
  "playbook_id": "webapp-deployment",
  "status": "running",
  "started": "2024-02-14T10:30:15Z",
  "completed": "0001-01-01T00:00:00Z",
  "duration_seconds": 0,
  "actions": [
    {
      "id": "action-001",
      "type": "package.install",
      "status": "success",
      "changed": true,
      "message": "Installed 2 packages: nginx, postgresql"
    },
    {
      "id": "action-002",
      "type": "directory.create",
      "status": "running",
      "changed": false,
      "message": ""
    }
  ],
  "summary": {
    "total": 8,
    "success": 1,
    "failed": 0,
    "skipped": 0,
    "changed": 1
  }
}
```

`status` transitions: `running` → `completed` or `failed`. The `summary.changed` count is the number of actions that modified system state (idempotent re-runs of already-applied playbooks will have `changed: 0` for all actions).

## Testability

The `FileSystem` and `CommandRunner` interfaces in `agent/executor/system.go` exist specifically to allow handler unit tests to inject test doubles without hitting the real filesystem or running real shell commands. The `agent/executor/executortest` package provides a mock engine for testing handler registration and dispatch.
