package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

func dirContext(fs *executortest.MockFileSystem) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{},
		FS:         fs,
		Cmd:        executortest.NewMockCommandRunner(),
	}
}

func dirAction(path string) playbook.Action {
	return playbook.Action{
		ID:   "test-dir",
		Type: "directory.create",
		Params: map[string]any{
			"path": path,
		},
	}
}

func TestDirectoryCreate_MissingPath(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID:     "test",
		Type:   "directory.create",
		Params: map[string]any{},
	}
	result := h.Execute(action, dirContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestDirectoryCreate_DryRun(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	ctx := dirContext(fs)
	ctx.DryRun = true

	result := h.Execute(dirAction("/opt/app"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if fs.Exists("/opt/app") {
		t.Fatal("dry run should not create directory")
	}
}

func TestDirectoryCreate_DryRunRecursive(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	ctx := dirContext(fs)
	ctx.DryRun = true

	action := dirAction("/opt/app")
	action.Params["recursive"] = true

	result := h.Execute(action, ctx)
	if result.Status != "success" {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Message != "Would create directory /opt/app (recursive)" {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}

func TestDirectoryCreate_CreatesDirectory(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	result := h.Execute(dirAction("/opt/app"), dirContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for new directory")
	}
	if !fs.Exists("/opt/app") {
		t.Fatal("directory should exist in executor fs")
	}
}

func TestDirectoryCreate_CreatesDirectoryRecursive(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()

	action := dirAction("/opt/app/data/logs")
	action.Params["recursive"] = true

	result := h.Execute(action, dirContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true")
	}
	if !fs.Exists("/opt/app/data/logs") {
		t.Fatal("directory should exist")
	}
	if !fs.Exists("/opt/app/data") {
		t.Fatal("intermediate directory should exist")
	}
}

func TestDirectoryCreate_NonRecursiveFailsWithoutParent(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	// No parent dir /opt exists

	result := h.Execute(dirAction("/opt/app"), dirContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed without parent, got %s", result.Status)
	}
}

func TestDirectoryCreate_IdempotentExistingDir(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt/app", 0755)

	result := h.Execute(dirAction("/opt/app"), dirContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change for existing directory")
	}
}

func TestDirectoryCreate_FailsWhenPathIsFile(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)
	fs.AddFile("/opt/app", []byte("i am a file"), 0644)

	result := h.Execute(dirAction("/opt/app"), dirContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed when path is a file, got %s", result.Status)
	}
}

func TestDirectoryCreate_CustomMode(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	action := dirAction("/opt/secret")
	action.Params["mode"] = "0700"

	result := h.Execute(action, dirContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	mode, ok := fs.FileMode("/opt/secret")
	if !ok {
		t.Fatal("directory not found")
	}
	// Chmod sets 0700; ModeDir bit is also present
	if mode&0777 != 0700 {
		t.Fatalf("expected mode 0700, got %04o", mode&0777)
	}
}

func TestDirectoryCreate_InvalidMode(t *testing.T) {
	h := NewDirectoryCreateHandler()
	fs := executortest.NewMockFileSystem()

	action := dirAction("/opt/app")
	action.Params["mode"] = "notamode"

	result := h.Execute(action, dirContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed for invalid mode, got %s", result.Status)
	}
}
