package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// helpers

func systemdContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "systemd"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func sysvinitContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "sysvinit"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func openrcContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "openrc"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func svcAction(name, op string) playbook.Action {
	return playbook.Action{
		ID:     "test-svc",
		Type:   "service." + op,
		Params: map[string]any{"name": name},
	}
}

// serviceIsActive helper: sets response for the status check command
func setSystemdActive(cmd *executor.MockCommandRunner, active bool) {
	exitCode := 1
	if active {
		exitCode = 0
	}
	cmd.SetResponse("systemctl", executor.MockCommandResponse{ExitCode: exitCode})
}

// --- ServiceStartHandler tests ---

func TestServiceStart_MissingName(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.start", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceStart_UnknownInitSystem(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "unknown"},
		Cmd:        cmd,
	}
	result := h.Execute(svcAction("nginx", "start"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceStart_DryRun(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := systemdContext(cmd)
	ctx.DryRun = true
	result := h.Execute(svcAction("nginx", "start"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestServiceStart_AlreadyRunning(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	setSystemdActive(cmd, true) // is-active returns 0
	result := h.Execute(svcAction("nginx", "start"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected Changed=false when service is already running")
	}
	// Only the status check should have been called, not start.
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (status check only), got %d", len(calls))
	}
}

func TestServiceStart_NotRunning_StartsService(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	// First call: is-active returns non-zero (not running).
	// Second call: start returns 0 (success).
	cmd.SetResponses("systemctl",
		executor.MockCommandResponse{ExitCode: 1}, // is-active → not active
		executor.MockCommandResponse{ExitCode: 0}, // start → success
	)
	result := h.Execute(svcAction("nginx", "start"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true after starting service")
	}
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (status check + start), got %d", len(calls))
	}
	if calls[1].Name != "systemctl" {
		t.Errorf("expected systemctl, got %s", calls[1].Name)
	}
	if len(calls[1].Args) < 1 || calls[1].Args[0] != "start" {
		t.Errorf("expected 'start' arg, got %v", calls[1].Args)
	}
}

func TestServiceStart_CommandFails(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	// is-active: not running
	cmd.SetResponse("systemctl", executor.MockCommandResponse{
		ExitCode: 1,
		Output:   []byte("failed"),
	})
	// Override: second call should also fail (systemctl start fails)
	// MockCommandRunner returns the same response for all calls to "systemctl"
	result := h.Execute(svcAction("nginx", "start"), systemdContext(cmd))
	// The start command returns the same error response as is-active, so it fails.
	if result.Status != "failed" {
		t.Fatalf("expected failed when start command fails, got %s", result.Status)
	}
}

// --- ServiceStopHandler tests ---

func TestServiceStop_MissingName(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.stop", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceStop_UnknownInitSystem(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "unknown"},
		Cmd:        cmd,
	}
	result := h.Execute(svcAction("nginx", "stop"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceStop_DryRun(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := systemdContext(cmd)
	ctx.DryRun = true
	result := h.Execute(svcAction("nginx", "stop"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestServiceStop_AlreadyStopped(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executor.NewMockCommandRunner()
	setSystemdActive(cmd, false) // is-active returns non-zero → stopped
	result := h.Execute(svcAction("nginx", "stop"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected Changed=false when service is already stopped")
	}
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (status check only), got %d", len(calls))
	}
}

func TestServiceStop_Running_StopsService(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executor.NewMockCommandRunner()
	setSystemdActive(cmd, true) // is-active returns 0 → running
	result := h.Execute(svcAction("nginx", "stop"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true after stopping service")
	}
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (status check + stop), got %d", len(calls))
	}
	if calls[1].Args[0] != "stop" {
		t.Errorf("expected 'stop' arg, got %v", calls[1].Args)
	}
}

// --- ServiceRestartHandler tests ---

func TestServiceRestart_MissingName(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.restart", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceRestart_UnknownInitSystem(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "unknown"},
		Cmd:        cmd,
	}
	result := h.Execute(svcAction("nginx", "restart"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceRestart_DryRun(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := systemdContext(cmd)
	ctx.DryRun = true
	result := h.Execute(svcAction("nginx", "restart"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestServiceRestart_AlwaysActs(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("systemctl", executor.MockCommandResponse{ExitCode: 0})
	result := h.Execute(svcAction("nginx", "restart"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true for restart")
	}
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call (restart, no status check), got %d", len(calls))
	}
	if calls[0].Args[0] != "restart" {
		t.Errorf("expected 'restart' arg, got %v", calls[0].Args)
	}
}

func TestServiceRestart_CommandFails(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("systemctl", executor.MockCommandResponse{
		ExitCode: 1,
		Output:   []byte("failed to restart"),
	})
	result := h.Execute(svcAction("nginx", "restart"), systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed when restart fails, got %s", result.Status)
	}
}

// --- ServiceReloadHandler tests ---

func TestServiceReload_MissingName(t *testing.T) {
	h := NewServiceReloadHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.reload", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceReload_UnknownInitSystem(t *testing.T) {
	h := NewServiceReloadHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "unknown"},
		Cmd:        cmd,
	}
	result := h.Execute(svcAction("nginx", "reload"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceReload_DryRun(t *testing.T) {
	h := NewServiceReloadHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := systemdContext(cmd)
	ctx.DryRun = true
	result := h.Execute(svcAction("nginx", "reload"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestServiceReload_AlwaysActs(t *testing.T) {
	h := NewServiceReloadHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("systemctl", executor.MockCommandResponse{ExitCode: 0})
	result := h.Execute(svcAction("nginx", "reload"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true for reload")
	}
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Args[0] != "reload" {
		t.Errorf("expected 'reload' arg, got %v", calls[0].Args)
	}
}

// --- sysvinit smoke tests ---

func TestServiceStart_Sysvinit(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executor.NewMockCommandRunner()
	// For sysvinit, the command name is the init.d script path.
	// First call: status → not active (exit 1). Second call: start → success (exit 0).
	cmd.SetResponses("/etc/init.d/nginx",
		executor.MockCommandResponse{ExitCode: 1}, // status → not active
		executor.MockCommandResponse{ExitCode: 0}, // start → success
	)
	result := h.Execute(svcAction("nginx", "start"), sysvinitContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v: %s", result.Status, result.Changed, result.Error)
	}
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	// Status check: /etc/init.d/nginx status
	if calls[0].Args[0] != "status" {
		t.Errorf("expected 'status' arg in call 0, got %v", calls[0].Args)
	}
	// Start call: /etc/init.d/nginx start
	if calls[1].Args[0] != "start" {
		t.Errorf("expected 'start' arg in call 1, got %v", calls[1].Args)
	}
}

// --- openrc smoke tests ---

func TestServiceStop_Openrc(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executor.NewMockCommandRunner()
	// For openrc, status check: rc-service nginx status returns 0 (running)
	cmd.SetResponse("rc-service", executor.MockCommandResponse{ExitCode: 0})
	result := h.Execute(svcAction("nginx", "stop"), openrcContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v: %s", result.Status, result.Changed, result.Error)
	}
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	// Status check: rc-service nginx status
	if calls[0].Args[1] != "status" {
		t.Errorf("expected 'status' in call 0 args, got %v", calls[0].Args)
	}
	// Stop call: rc-service nginx stop
	if calls[1].Args[1] != "stop" {
		t.Errorf("expected 'stop' in call 1 args, got %v", calls[1].Args)
	}
}
