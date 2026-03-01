package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

func fileContext(fs *executortest.MockFileSystem) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{},
		FS:         fs,
		Cmd:        executortest.NewMockCommandRunner(),
	}
}

func fileAction(destination, content string) playbook.Action {
	return playbook.Action{
		ID:   "test-file",
		Type: "file.content",
		Params: map[string]any{
			"destination": destination,
			"content":     content,
		},
	}
}

func TestFileContent_MissingDestination(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID:   "test",
		Type: "file.content",
		Params: map[string]any{
			"content": "hello",
		},
	}
	result := h.Execute(action, fileContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileContent_MissingContent(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID:   "test",
		Type: "file.content",
		Params: map[string]any{
			"destination": "/etc/test.conf",
		},
	}
	result := h.Execute(action, fileContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileContent_DryRun(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	ctx := fileContext(fs)
	ctx.DryRun = true

	result := h.Execute(fileAction("/etc/test.conf", "hello"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if fs.Exists("/etc/test.conf") {
		t.Fatal("dry run should not create file")
	}
}

func TestFileContent_CreatesNewFile(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	result := h.Execute(fileAction("/etc/test.conf", "hello world"), fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for new file")
	}

	data, ok := fs.FileContent("/etc/test.conf")
	if !ok {
		t.Fatal("file not found in executor fs")
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", string(data))
	}
}

func TestFileContent_CreatesParentDirectories(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	// No parent dirs exist, MkdirAll should create them

	result := h.Execute(fileAction("/opt/app/config/test.conf", "data"), fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !fs.Exists("/opt/app/config") {
		t.Fatal("parent directory should have been created")
	}
}

func TestFileContent_IdempotentSameContent(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/test.conf", []byte("hello"), 0644)

	result := h.Execute(fileAction("/etc/test.conf", "hello"), fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change for same content")
	}
}

func TestFileContent_UpdatesDifferentContent(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/test.conf", []byte("old content"), 0644)

	result := h.Execute(fileAction("/etc/test.conf", "new content"), fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for different content")
	}

	data, _ := fs.FileContent("/etc/test.conf")
	if string(data) != "new content" {
		t.Fatalf("expected 'new content', got '%s'", string(data))
	}
}

func TestFileContent_SetsMode(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	action := fileAction("/etc/secret.conf", "secret")
	action.Params["mode"] = "0600"

	result := h.Execute(action, fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	mode, ok := fs.FileMode("/etc/secret.conf")
	if !ok {
		t.Fatal("file not found")
	}
	if mode != 0600 {
		t.Fatalf("expected mode 0600, got %04o", mode)
	}
}

func TestFileContent_MessageShowsCreated(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	result := h.Execute(fileAction("/etc/new.conf", "data"), fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Message != "File /etc/new.conf created (4 bytes)" {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}

func TestFileContent_MessageShowsUpdated(t *testing.T) {
	h := NewFileContentHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/existing.conf", []byte("old"), 0644)

	result := h.Execute(fileAction("/etc/existing.conf", "new"), fileContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Message != "File /etc/existing.conf updated (3 bytes)" {
		t.Fatalf("unexpected message: %s", result.Message)
	}
}
