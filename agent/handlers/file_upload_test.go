package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

const testPlaybookPath = "/playbook"

func uploadContext(fs *executortest.MockFileSystem) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo:   &executor.Info{},
		PlaybookPath: testPlaybookPath,
		FS:           fs,
		Cmd:          executortest.NewMockCommandRunner(),
	}
}

func uploadAction(source, destination string) playbook.Action {
	return playbook.Action{
		ID:   "test-upload",
		Type: "file.upload",
		Params: map[string]any{
			"source":      source,
			"destination": destination,
		},
	}
}

func TestFileUpload_MissingSource(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID: "test", Type: "file.upload",
		Params: map[string]any{"destination": "/etc/foo"},
	}
	result := h.Execute(action, uploadContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileUpload_MissingDestination(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID: "test", Type: "file.upload",
		Params: map[string]any{"source": "upload/foo"},
	}
	result := h.Execute(action, uploadContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileUpload_DryRun(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	ctx := uploadContext(fs)
	ctx.DryRun = true
	result := h.Execute(uploadAction("upload/foo", "/etc/foo"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if fs.Exists("/etc/foo") {
		t.Fatal("dry run should not write file")
	}
}

func TestFileUpload_SourceNotFound(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	// Source file does not exist in playbook
	result := h.Execute(uploadAction("upload/missing", "/etc/foo"), uploadContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileUpload_NewFile(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	// Pre-populate source in playbook bundle
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/myfile", []byte("hello"), 0644)
	fs.AddDir("/etc", 0755)

	result := h.Execute(uploadAction("upload/myfile", "/etc/myfile"), uploadContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for new file")
	}

	data, ok := fs.FileContent("/etc/myfile")
	if !ok {
		t.Fatal("file not written")
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestFileUpload_IdempotentSameContent(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/myfile", []byte("hello"), 0644)
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/myfile", []byte("hello"), 0644)

	result := h.Execute(uploadAction("upload/myfile", "/etc/myfile"), uploadContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change for identical content")
	}
}

func TestFileUpload_DifferentContent(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/myfile", []byte("new content"), 0644)
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/myfile", []byte("old content"), 0644)

	result := h.Execute(uploadAction("upload/myfile", "/etc/myfile"), uploadContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for different content")
	}

	data, _ := fs.FileContent("/etc/myfile")
	if string(data) != "new content" {
		t.Fatalf("expected 'new content', got '%s'", string(data))
	}
}

func TestFileUpload_CreatesParentDirectory(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/myfile", []byte("data"), 0644)
	// No parent dir created yet

	result := h.Execute(uploadAction("upload/myfile", "/opt/app/myfile"), uploadContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !fs.Exists("/opt/app") {
		t.Fatal("parent directory should have been created")
	}
}

func TestFileUpload_CustomMode(t *testing.T) {
	h := NewFileUploadHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/secret", []byte("secret"), 0644)
	fs.AddDir("/etc", 0755)

	action := uploadAction("upload/secret", "/etc/secret")
	action.Params["mode"] = "0600"

	result := h.Execute(action, uploadContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	mode, ok := fs.FileMode("/etc/secret")
	if !ok {
		t.Fatal("file not found")
	}
	if mode != 0600 {
		t.Fatalf("expected mode 0600, got %04o", mode)
	}
}
