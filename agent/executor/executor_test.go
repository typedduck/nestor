package executor_test

import (
	"testing"
	"time"

	"github.com/typedduck/nestor/agent/executor"
)

// stubHandler is a configurable test handler.
type stubHandler struct {
	result executor.ActionResult
}

func (h *stubHandler) Execute(action executor.Action, ctx *executor.ExecutionContext) executor.ActionResult {
	return h.result
}

func testPlaybook(actions ...executor.Action) *executor.Playbook {
	return &executor.Playbook{
		Name:        "test-playbook",
		Version:     "1.0",
		Created:     time.Now(),
		Controller:  "test",
		Environment: map[string]string{"ENV": "test"},
		Actions:     actions,
		ExtractPath: "/tmp/test",
	}
}

func testEngine(pb *executor.Playbook, fs *executor.MockFileSystem,
	cmd *executor.MockCommandRunner) *executor.Engine {
	return executor.New(pb, &executor.Info{
		OS:             "linux",
		Distribution:   "ubuntu",
		PackageManager: "apt",
		InitSystem:     "executord",
		Architecture:   "amd64",
	}, "", fs, cmd)
}

func TestEngine_ExecuteEmpty(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook()
	engine := testEngine(pb, fs, cmd)

	result, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if result.Summary.Total != 0 {
		t.Fatalf("expected 0 total, got %d", result.Summary.Total)
	}
}

func TestEngine_ExecuteSuccess(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "test.action", Params: map[string]any{}},
	)
	engine := testEngine(pb, fs, cmd)
	engine.RegisterHandler("test.action", &stubHandler{
		result: executor.ActionResult{Status: "success", Changed: true, Message: "done"},
	})

	result, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	if result.Summary.Success != 1 {
		t.Fatalf("expected 1 success, got %d", result.Summary.Success)
	}
	if result.Summary.Changed != 1 {
		t.Fatalf("expected 1 changed, got %d", result.Summary.Changed)
	}
}

func TestEngine_ExecuteStopsOnFailure(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "fail.action", Params: map[string]any{}},
		executor.Action{ID: "a2", Type: "test.action", Params: map[string]any{}},
	)
	engine := testEngine(pb, fs, cmd)
	engine.RegisterHandler("fail.action", &stubHandler{
		result: executor.ActionResult{Status: "failed", Message: "boom", Error: "error"},
	})
	engine.RegisterHandler("test.action", &stubHandler{
		result: executor.ActionResult{Status: "success", Message: "ok"},
	})

	result, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action (stopped on failure), got %d", len(result.Actions))
	}
	if result.Summary.Failed != 1 {
		t.Fatalf("expected 1 failed, got %d", result.Summary.Failed)
	}
}

func TestEngine_UnknownActionType(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "no.handler", Params: map[string]any{}},
	)
	engine := testEngine(pb, fs, cmd)

	result, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	if result.Actions[0].Error == "" {
		t.Fatal("expected error message for unknown action type")
	}
}

func TestEngine_ContextHasFSAndCmd(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "check.context", Params: map[string]any{}},
	)
	engine := testEngine(pb, fs, cmd)

	var capturedCtx *executor.ExecutionContext
	engine.RegisterHandler("check.context", handlerFunc(func(action executor.Action, ctx *executor.ExecutionContext) executor.ActionResult {
		capturedCtx = ctx
		return executor.ActionResult{Status: "success", Message: "ok"}
	}))

	_, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedCtx == nil {
		t.Fatal("handler was not called")
	}
	if capturedCtx.FS == nil {
		t.Fatal("FS should not be nil in context")
	}
	if capturedCtx.Cmd == nil {
		t.Fatal("Cmd should not be nil in context")
	}
	if capturedCtx.Environment["ENV"] != "test" {
		t.Fatalf("expected ENV=test, got %s", capturedCtx.Environment["ENV"])
	}
}

func TestEngine_DryRun(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "check.dryrun", Params: map[string]any{}},
	)
	engine := testEngine(pb, fs, cmd)
	engine.SetDryRun(true)

	var dryRun bool
	engine.RegisterHandler("check.dryrun", handlerFunc(func(action executor.Action, ctx *executor.ExecutionContext) executor.ActionResult {
		dryRun = ctx.DryRun
		return executor.ActionResult{Status: "success", Message: "ok"}
	}))

	engine.Execute()
	if !dryRun {
		t.Fatal("expected DryRun=true in context")
	}
}

func TestEngine_SavesState(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "test.action", Params: map[string]any{}},
	)
	engine := executor.New(pb, &executor.Info{}, "/tmp/test.state", fs, cmd)
	engine.RegisterHandler("test.action", &stubHandler{
		result: executor.ActionResult{Status: "success", Message: "ok"},
	})

	// The state file write needs a parent directory
	fs.AddDir("/tmp", 0755)

	_, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fs.Exists("/tmp/test.state") {
		t.Fatal("state file should have been written")
	}
}

func TestLoadState(t *testing.T) {
	fs := executor.NewMockFileSystem()
	fs.AddDir("/tmp", 0755)
	stateJSON := `{"playbook_id":"test","status":"completed","summary":{"total":1,"success":1}}`
	fs.AddFile("/tmp/test.state", []byte(stateJSON), 0644)

	result, err := executor.LoadState("/tmp/test.state", fs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PlaybookID != "test" {
		t.Fatalf("expected playbook_id=test, got %s", result.PlaybookID)
	}
	if result.Summary.Success != 1 {
		t.Fatalf("expected 1 success, got %d", result.Summary.Success)
	}
}

func TestLoadState_FileNotFound(t *testing.T) {
	fs := executor.NewMockFileSystem()
	_, err := executor.LoadState("/tmp/nonexistent.state", fs)
	if err == nil {
		t.Fatal("expected error for missing state file")
	}
}

func TestEngine_MultipleActions(t *testing.T) {
	fs := executor.NewMockFileSystem()
	cmd := executor.NewMockCommandRunner()
	pb := testPlaybook(
		executor.Action{ID: "a1", Type: "test.action", Params: map[string]any{}},
		executor.Action{ID: "a2", Type: "test.action", Params: map[string]any{}},
		executor.Action{ID: "a3", Type: "test.action", Params: map[string]any{}},
	)
	engine := testEngine(pb, fs, cmd)
	engine.RegisterHandler("test.action", &stubHandler{
		result: executor.ActionResult{Status: "success", Changed: false, Message: "ok"},
	})

	result, err := engine.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary.Total != 3 {
		t.Fatalf("expected 3 total, got %d", result.Summary.Total)
	}
	if result.Summary.Success != 3 {
		t.Fatalf("expected 3 success, got %d", result.Summary.Success)
	}
	if len(result.Actions) != 3 {
		t.Fatalf("expected 3 action results, got %d", len(result.Actions))
	}
	// Verify IDs are set from the action
	if result.Actions[1].ID != "a2" {
		t.Fatalf("expected ID=a2, got %s", result.Actions[1].ID)
	}
}

// handlerFunc adapts a function to the Handler interface.
type handlerFunc func(executor.Action, *executor.ExecutionContext) executor.ActionResult

func (f handlerFunc) Execute(action executor.Action, ctx *executor.ExecutionContext) executor.ActionResult {
	return f(action, ctx)
}
