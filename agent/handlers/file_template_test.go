package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

func templateContext(fs *executortest.MockFileSystem) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo:   &executor.Info{},
		PlaybookPath: testPlaybookPath,
		FS:           fs,
		Cmd:          executortest.NewMockCommandRunner(),
	}
}

func templateAction(source, destination string) playbook.Action {
	return playbook.Action{
		ID:   "test-template",
		Type: "file.template",
		Params: map[string]any{
			"source":      source,
			"destination": destination,
		},
	}
}

func TestFileTemplate_MissingSource(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID: "test", Type: "file.template",
		Params: map[string]any{"destination": "/etc/foo"},
	}
	result := h.Execute(action, templateContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileTemplate_MissingDestination(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	action := playbook.Action{
		ID: "test", Type: "file.template",
		Params: map[string]any{"source": "upload/foo.tmpl"},
	}
	result := h.Execute(action, templateContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileTemplate_SourceNotFound(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	// Template source does not exist in playbook
	result := h.Execute(templateAction("upload/missing.tmpl", "/etc/foo"), templateContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestFileTemplate_DryRun(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/foo.tmpl", []byte("hello"), 0644)

	ctx := templateContext(fs)
	ctx.DryRun = true
	result := h.Execute(templateAction("upload/foo.tmpl", "/etc/foo"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if fs.Exists("/etc/foo") {
		t.Fatal("dry run should not write file")
	}
}

func TestFileTemplate_NoVariables(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/foo.tmpl", []byte("static content"), 0644)
	fs.AddDir("/etc", 0755)

	result := h.Execute(templateAction("upload/foo.tmpl", "/etc/foo"), templateContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for new file")
	}

	data, ok := fs.FileContent("/etc/foo")
	if !ok {
		t.Fatal("file not written")
	}
	if string(data) != "static content" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestFileTemplate_WithVariables(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/greeting.tmpl", []byte("Hello, {{.Name}}!"), 0644)
	fs.AddDir("/etc", 0755)

	action := templateAction("upload/greeting.tmpl", "/etc/greeting")
	action.Params["variables"] = map[string]any{"Name": "World"}

	result := h.Execute(action, templateContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	data, ok := fs.FileContent("/etc/greeting")
	if !ok {
		t.Fatal("file not written")
	}
	if string(data) != "Hello, World!" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

func TestFileTemplate_ParseError(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	// Invalid template syntax
	fs.AddFile(testPlaybookPath+"/upload/bad.tmpl", []byte("{{.Unclosed"), 0644)

	result := h.Execute(templateAction("upload/bad.tmpl", "/etc/bad"), templateContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed for parse error, got %s", result.Status)
	}
}

func TestFileTemplate_ExecuteError(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	// Template references undefined key — missingkey=error should trigger
	fs.AddFile(testPlaybookPath+"/upload/tmpl.tmpl", []byte("{{.UndefinedKey}}"), 0644)

	action := templateAction("upload/tmpl.tmpl", "/etc/out")
	action.Params["variables"] = map[string]any{"OtherKey": "value"}

	result := h.Execute(action, templateContext(fs))
	if result.Status != "failed" {
		t.Fatalf("expected failed for undefined key, got %s", result.Status)
	}
}

func TestFileTemplate_IdempotentSameContent(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/foo.tmpl", []byte("hello"), 0644)
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/foo", []byte("hello"), 0644)

	result := h.Execute(templateAction("upload/foo.tmpl", "/etc/foo"), templateContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change for identical rendered content")
	}
}

func TestFileTemplate_DifferentContent(t *testing.T) {
	h := NewFileTemplateHandler()
	fs := executortest.NewMockFileSystem()
	fs.AddDir(testPlaybookPath, 0755)
	fs.AddDir(testPlaybookPath+"/upload", 0755)
	fs.AddFile(testPlaybookPath+"/upload/foo.tmpl", []byte("new content"), 0644)
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/foo", []byte("old content"), 0644)

	result := h.Execute(templateAction("upload/foo.tmpl", "/etc/foo"), templateContext(fs))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true for different content")
	}

	data, _ := fs.FileContent("/etc/foo")
	if string(data) != "new content" {
		t.Fatalf("expected 'new content', got '%s'", string(data))
	}
}
