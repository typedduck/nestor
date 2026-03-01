package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

func cmdContext(fs *executortest.MockFileSystem, cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo:   &executor.Info{},
		PlaybookPath: testPlaybookPath,
		FS:           fs,
		Cmd:          cmd,
	}
}

// --- CommandExecuteHandler ---

func TestCommandExecute_MissingCommand(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestCommandExecute_DryRun(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := cmdContext(executortest.NewMockFileSystem(), cmd)
	ctx.DryRun = true
	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{"command": "echo hello"},
	}
	result := h.Execute(action, ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestCommandExecute_CreatesExistsSkips(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/tmp/done", []byte{}, 0644)

	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{
			"command": "touch /tmp/done",
			"creates": "/tmp/done",
		},
	}
	result := h.Execute(action, cmdContext(fs, cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when creates path exists")
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("should not run command when creates path exists")
	}
}

func TestCommandExecute_CreatesNotExistsRuns(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	// creates path does not exist
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{
			"command": "touch /tmp/done",
			"creates": "/tmp/done",
		},
	}
	result := h.Execute(action, cmdContext(fs, cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after command ran")
	}
	calls := cmd.Calls()
	if len(calls) != 1 || calls[0].Name != "/bin/sh" {
		t.Fatalf("expected /bin/sh call, got %v", calls)
	}
}

func TestCommandExecute_Succeeds(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{"command": "echo hello"},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	// Verify -c flag passed to sh
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Args[0] != "-c" {
		t.Errorf("expected first arg to be -c, got %s", calls[0].Args[0])
	}
	if calls[0].Args[1] != "echo hello" {
		t.Errorf("expected second arg to be command string, got %s", calls[0].Args[1])
	}
}

func TestCommandExecute_Fails(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{
		Output: []byte("error output"), ExitCode: 1,
	})

	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{"command": "false"},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestCommandExecute_WithEnv(t *testing.T) {
	h := NewCommandExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "command.execute",
		Params: map[string]any{
			"command": "echo $MY_VAR",
			"env":     []any{"MY_VAR=hello"},
		},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	calls := cmd.Calls()
	if calls[0].Opts == nil || len(calls[0].Opts.Env) == 0 {
		t.Error("expected env vars to be set")
	}
}

// --- ScriptExecuteHandler ---

func TestScriptExecute_MissingSource(t *testing.T) {
	h := NewScriptExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID: "test", Type: "script.execute",
		Params: map[string]any{},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestScriptExecute_DryRun(t *testing.T) {
	h := NewScriptExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := cmdContext(executortest.NewMockFileSystem(), cmd)
	ctx.DryRun = true
	action := playbook.Action{
		ID: "test", Type: "script.execute",
		Params: map[string]any{"source": "upload/setup.sh"},
	}
	result := h.Execute(action, ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestScriptExecute_CreatesExistsSkips(t *testing.T) {
	h := NewScriptExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/etc/app/setup.done", []byte{}, 0644)

	action := playbook.Action{
		ID: "test", Type: "script.execute",
		Params: map[string]any{
			"source":  "upload/setup.sh",
			"creates": "/etc/app/setup.done",
		},
	}
	result := h.Execute(action, cmdContext(fs, cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when creates path exists")
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("should not run script when creates path exists")
	}
}

func TestScriptExecute_Succeeds(t *testing.T) {
	h := NewScriptExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "script.execute",
		Params: map[string]any{"source": "upload/setup.sh"},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	// Verify /bin/sh called with resolved script path
	calls := cmd.Calls()
	if len(calls) != 1 || calls[0].Name != "/bin/sh" {
		t.Fatalf("expected /bin/sh call, got %v", calls)
	}
	expectedPath := testPlaybookPath + "/upload/setup.sh"
	if len(calls[0].Args) == 0 || calls[0].Args[0] != expectedPath {
		t.Errorf("expected script path %s as first arg, got %v", expectedPath, calls[0].Args)
	}
}

func TestScriptExecute_WithArgs(t *testing.T) {
	h := NewScriptExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "script.execute",
		Params: map[string]any{
			"source": "upload/setup.sh",
			"args":   []any{"--verbose", "--force"},
		},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	calls := cmd.Calls()
	// Args should be: [scriptPath, "--verbose", "--force"]
	if len(calls[0].Args) != 3 {
		t.Fatalf("expected 3 args (script + 2 flags), got %d: %v", len(calls[0].Args), calls[0].Args)
	}
}

func TestScriptExecute_Fails(t *testing.T) {
	h := NewScriptExecuteHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("/bin/sh", executortest.MockCommandResponse{
		Output: []byte("script error"), ExitCode: 1,
	})

	action := playbook.Action{
		ID: "test", Type: "script.execute",
		Params: map[string]any{"source": "upload/setup.sh"},
	}
	result := h.Execute(action, cmdContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}
