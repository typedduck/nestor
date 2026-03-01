package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

func opsContext(fs *executortest.MockFileSystem, cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{},
		FS:         fs,
		Cmd:        cmd,
	}
}

// --- SymlinkHandler ---

func TestSymlink_MissingDestination(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{"source": "/usr/bin/python3"},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestSymlink_MissingSource(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{"destination": "/usr/bin/python"},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestSymlink_DryRun(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := opsContext(executortest.NewMockFileSystem(), cmd)
	ctx.DryRun = true
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{
			"destination": "/usr/bin/python",
			"source":      "/usr/bin/python3",
		},
	}
	result := h.Execute(action, ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestSymlink_AlreadyCorrect(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	// readlink returns the correct target
	cmd.SetResponse("readlink", executortest.MockCommandResponse{
		Output: []byte("/usr/bin/python3\n"), ExitCode: 0,
	})
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{
			"destination": "/usr/bin/python",
			"source":      "/usr/bin/python3",
		},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when symlink already correct")
	}
	// Only readlink should have been called; ln should not
	calls := cmd.Calls()
	if len(calls) != 1 || calls[0].Name != "readlink" {
		t.Fatalf("expected only readlink call, got %v", calls)
	}
}

func TestSymlink_ReadlinkFails(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	// readlink fails (symlink doesn't exist)
	cmd.SetResponse("readlink", executortest.MockCommandResponse{ExitCode: 1})
	cmd.SetResponse("ln", executortest.MockCommandResponse{ExitCode: 0})
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{
			"destination": "/usr/bin/python",
			"source":      "/usr/bin/python3",
		},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after creating symlink")
	}
	// readlink + ln -sf
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[1].Name != "ln" {
		t.Errorf("expected ln call, got %s", calls[1].Name)
	}
}

func TestSymlink_WrongTarget(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	// readlink returns wrong target
	cmd.SetResponse("readlink", executortest.MockCommandResponse{
		Output: []byte("/usr/bin/python2\n"), ExitCode: 0,
	})
	cmd.SetResponse("ln", executortest.MockCommandResponse{ExitCode: 0})
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{
			"destination": "/usr/bin/python",
			"source":      "/usr/bin/python3",
		},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true when symlink was replaced")
	}
}

func TestSymlink_LnFails(t *testing.T) {
	h := NewSymlinkHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("readlink", executortest.MockCommandResponse{ExitCode: 1})
	cmd.SetResponse("ln", executortest.MockCommandResponse{ExitCode: 1})
	action := playbook.Action{
		ID: "test", Type: "file.symlink",
		Params: map[string]any{
			"destination": "/usr/bin/python",
			"source":      "/usr/bin/python3",
		},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

// --- FileRemoveHandler ---

func TestFileRemove_MissingPath(t *testing.T) {
	h := NewFileRemoveHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID: "test", Type: "file.remove",
		Params: map[string]any{},
	}
	result := h.Execute(action, opsContext(executortest.NewMockFileSystem(), cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileRemove_DryRun(t *testing.T) {
	h := NewFileRemoveHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := opsContext(executortest.NewMockFileSystem(), cmd)
	ctx.DryRun = true
	action := playbook.Action{
		ID: "test", Type: "file.remove",
		Params: map[string]any{"path": "/tmp/foo"},
	}
	result := h.Execute(action, ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestFileRemove_PathDoesNotExist(t *testing.T) {
	h := NewFileRemoveHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID: "test", Type: "file.remove",
		Params: map[string]any{"path": "/tmp/nonexistent"},
	}
	result := h.Execute(action, opsContext(fs, cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when path does not exist")
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("should not call rm when path does not exist")
	}
}

func TestFileRemove_NonRecursive(t *testing.T) {
	h := NewFileRemoveHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/tmp/foo", []byte("data"), 0644)
	cmd.SetResponse("rm", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "file.remove",
		Params: map[string]any{"path": "/tmp/foo"},
	}
	result := h.Execute(action, opsContext(fs, cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after removal")
	}

	calls := cmd.Calls()
	if len(calls) != 1 || calls[0].Name != "rm" {
		t.Fatalf("expected 1 rm call, got %v", calls)
	}
	// Verify -f flag used (not -rf)
	if len(calls[0].Args) < 1 || calls[0].Args[0] != "-f" {
		t.Errorf("expected -f flag, got args: %v", calls[0].Args)
	}
}

func TestFileRemove_Recursive(t *testing.T) {
	h := NewFileRemoveHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/tmp/mydir", 0755)
	cmd.SetResponse("rm", executortest.MockCommandResponse{ExitCode: 0})

	action := playbook.Action{
		ID: "test", Type: "file.remove",
		Params: map[string]any{
			"path":      "/tmp/mydir",
			"recursive": true,
		},
	}
	result := h.Execute(action, opsContext(fs, cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after removal")
	}

	calls := cmd.Calls()
	if len(calls) != 1 || calls[0].Name != "rm" {
		t.Fatalf("expected 1 rm call, got %v", calls)
	}
	// Verify -rf flag used
	if len(calls[0].Args) < 1 || calls[0].Args[0] != "-rf" {
		t.Errorf("expected -rf flag, got args: %v", calls[0].Args)
	}
}

func TestFileRemove_RmFails(t *testing.T) {
	h := NewFileRemoveHandler()
	cmd := executortest.NewMockCommandRunner()
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/tmp/locked", []byte("data"), 0644)
	cmd.SetResponse("rm", executortest.MockCommandResponse{ExitCode: 1})

	action := playbook.Action{
		ID: "test", Type: "file.remove",
		Params: map[string]any{"path": "/tmp/locked"},
	}
	result := h.Execute(action, opsContext(fs, cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}
