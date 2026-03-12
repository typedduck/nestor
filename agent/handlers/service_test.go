package handlers

import (
	"strings"
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

// helpers

func systemdContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "systemd"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func sysvinitContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "sysvinit"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func openrcContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{InitSystem: "openrc"},
		FS:         executortest.NewMockFileSystem(),
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

func svcActionRunAs(name, op, runAs string) playbook.Action {
	return playbook.Action{
		ID:     "test-svc",
		Type:   "service." + op,
		Params: map[string]any{"name": name, "run_as": runAs},
	}
}

// serviceIsActive helper: sets response for the status check command
func setSystemdActive(cmd *executortest.MockCommandRunner, active bool) {
	exitCode := 1
	if active {
		exitCode = 0
	}
	cmd.SetResponse("systemctl", executortest.MockCommandResponse{ExitCode: exitCode})
}

// --- ServiceStartHandler tests ---

func TestServiceStart_MissingName(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.start", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceStart_UnknownInitSystem(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
	// First call: is-active returns non-zero (not running).
	// Second call: start returns 0 (success).
	cmd.SetResponses("systemctl",
		executortest.MockCommandResponse{ExitCode: 1}, // is-active → not active
		executortest.MockCommandResponse{ExitCode: 0}, // start → success
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
	cmd := executortest.NewMockCommandRunner()
	// is-active: not running
	cmd.SetResponse("systemctl", executortest.MockCommandResponse{
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
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.stop", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceStop_UnknownInitSystem(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.restart", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceRestart_UnknownInitSystem(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("systemctl", executortest.MockCommandResponse{ExitCode: 0})
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
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("systemctl", executortest.MockCommandResponse{
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
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{ID: "t", Type: "service.reload", Params: map[string]any{}}
	result := h.Execute(action, systemdContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestServiceReload_UnknownInitSystem(t *testing.T) {
	h := NewServiceReloadHandler()
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
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
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("systemctl", executortest.MockCommandResponse{ExitCode: 0})
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
	cmd := executortest.NewMockCommandRunner()
	// For sysvinit, the command name is the init.d script path.
	// First call: status → not active (exit 1). Second call: start → success (exit 0).
	cmd.SetResponses("/etc/init.d/nginx",
		executortest.MockCommandResponse{ExitCode: 1}, // status → not active
		executortest.MockCommandResponse{ExitCode: 0}, // start → success
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

// --- RunAs tests ---

func TestServiceRestart_RunAs_RequiresSystemd(t *testing.T) {
	for _, initSystem := range []string{"sysvinit", "openrc"} {
		t.Run(initSystem, func(t *testing.T) {
			h := NewServiceRestartHandler()
			cmd := executortest.NewMockCommandRunner()
			ctx := &executor.ExecutionContext{
				SystemInfo: &executor.Info{InitSystem: initSystem},
				FS:         executortest.NewMockFileSystem(),
				Cmd:        cmd,
			}
			result := h.Execute(svcActionRunAs("myapp", "restart", "alice"), ctx)
			if result.Status != "failed" {
				t.Fatalf("expected failed for RunAs on %s, got %s", initSystem, result.Status)
			}
			if len(cmd.Calls()) != 0 {
				t.Fatal("no commands should be invoked when RunAs is rejected")
			}
		})
	}
}

func TestServiceRestart_RunAs_Systemd_UsesSudo(t *testing.T) {
	h := NewServiceRestartHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("sudo", executortest.MockCommandResponse{ExitCode: 0})
	result := h.Execute(svcActionRunAs("myapp", "restart", "alice"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected Changed=true for restart")
	}
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Name != "sudo" {
		t.Errorf("expected 'sudo', got %s", calls[0].Name)
	}
	if len(calls[0].Args) < 2 || calls[0].Args[0] != "-u" || calls[0].Args[1] != "alice" {
		t.Errorf("expected sudo -u alice ..., got %v", calls[0].Args)
	}
}

func TestServiceReload_RunAs_Systemd_UsesSudo(t *testing.T) {
	h := NewServiceReloadHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("sudo", executortest.MockCommandResponse{ExitCode: 0})
	result := h.Execute(svcActionRunAs("myapp", "reload", "alice"), systemdContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	calls := cmd.Calls()
	if len(calls) != 1 || calls[0].Name != "sudo" {
		t.Fatalf("expected 1 sudo call, got %v", calls)
	}
	// Verify the shell command contains --user and the service name.
	shellCmd := calls[0].Args[len(calls[0].Args)-1]
	for _, want := range []string{"--user", "reload", "myapp"} {
		if !strings.Contains(shellCmd, want) {
			t.Errorf("shell command %q missing %q", shellCmd, want)
		}
	}
}

func TestServiceStart_RunAs_Systemd_BothCallsUseSudo(t *testing.T) {
	h := NewServiceStartHandler()
	cmd := executortest.NewMockCommandRunner()
	// Both is-active check and start call go through "sudo".
	cmd.SetResponses("sudo",
		executortest.MockCommandResponse{ExitCode: 1}, // is-active --quiet → not active
		executortest.MockCommandResponse{ExitCode: 0}, // start → success
	)
	result := h.Execute(svcActionRunAs("myapp", "start", "alice"), systemdContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v: %s", result.Status, result.Changed, result.Error)
	}
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 sudo calls, got %d", len(calls))
	}
	for i, call := range calls {
		if call.Name != "sudo" {
			t.Errorf("call[%d]: expected sudo, got %s", i, call.Name)
		}
		if call.Args[0] != "-u" || call.Args[1] != "alice" {
			t.Errorf("call[%d]: expected -u alice, got %v", i, call.Args)
		}
	}
}

// --- openrc smoke tests ---

func TestServiceStop_Openrc(t *testing.T) {
	h := NewServiceStopHandler()
	cmd := executortest.NewMockCommandRunner()
	// For openrc, status check: rc-service nginx status returns 0 (running)
	cmd.SetResponse("rc-service", executortest.MockCommandResponse{ExitCode: 0})
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
