package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

func unknownPMContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "unknown"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func rmAction(packages ...string) playbook.Action {
	pkgs := make([]any, len(packages))
	for i, p := range packages {
		pkgs[i] = p
	}
	return playbook.Action{
		ID:   "test-pkg-remove",
		Type: "package.remove",
		Params: map[string]any{
			"packages": pkgs,
		},
	}
}

// --- PackageRemoveHandler ---

func TestPackageRemove_MissingPackagesParam(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{
		ID: "test", Type: "package.remove",
		Params: map[string]any{},
	}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageRemove_UnknownPackageManager(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	result := h.Execute(rmAction("nginx"), unknownPMContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageRemove_DryRun(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := aptContext(cmd)
	ctx.DryRun = true
	result := h.Execute(rmAction("nginx"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestPackageRemove_PackageNotInstalled(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	// dpkg-query fails → package not installed
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{ExitCode: 1})
	result := h.Execute(rmAction("nginx"), aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when package not installed")
	}
}

func TestPackageRemove_AptPackageInstalled(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
		Output: []byte("install ok installed"), ExitCode: 0,
	})
	cmd.SetResponse("apt-get", executor.MockCommandResponse{ExitCode: 0})
	result := h.Execute(rmAction("nginx"), aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after removal")
	}
}

func TestPackageRemove_RemoveFails(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
		Output: []byte("install ok installed"), ExitCode: 0,
	})
	cmd.SetResponse("apt-get", executor.MockCommandResponse{
		Output: []byte("E: dpkg was interrupted"), ExitCode: 1,
	})
	result := h.Execute(rmAction("nginx"), aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageRemove_YumPackageInstalled(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	// yum list installed succeeds → installed; yum remove also succeeds
	cmd.SetResponse("yum", executor.MockCommandResponse{ExitCode: 0})
	result := h.Execute(rmAction("nginx"), yumContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after removal")
	}
	// Verify two yum calls: list installed + remove
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 yum calls, got %d", len(calls))
	}
}

func TestPackageRemove_YumPackageNotInstalled(t *testing.T) {
	h := NewPackageRemoveHandler()
	cmd := executor.NewMockCommandRunner()
	// yum list installed fails → not installed
	cmd.SetResponse("yum", executor.MockCommandResponse{ExitCode: 1})
	result := h.Execute(rmAction("nginx"), yumContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when package not installed")
	}
	// Only one call: list installed check; no remove
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 yum call, got %d", len(calls))
	}
}

// --- PackageUpdateHandler ---

func TestPackageUpdate_UnknownPackageManager(t *testing.T) {
	h := NewPackageUpdateHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{ID: "test", Type: "package.update", Params: map[string]any{}}
	result := h.Execute(action, unknownPMContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageUpdate_DryRun(t *testing.T) {
	h := NewPackageUpdateHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := aptContext(cmd)
	ctx.DryRun = true
	action := playbook.Action{ID: "test", Type: "package.update", Params: map[string]any{}}
	result := h.Execute(action, ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestPackageUpdate_AptSucceeds(t *testing.T) {
	h := NewPackageUpdateHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executor.MockCommandResponse{ExitCode: 0})
	action := playbook.Action{ID: "test", Type: "package.update", Params: map[string]any{}}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
}

func TestPackageUpdate_AptFails(t *testing.T) {
	h := NewPackageUpdateHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executor.MockCommandResponse{
		Output: []byte("E: failed"), ExitCode: 1,
	})
	action := playbook.Action{ID: "test", Type: "package.update", Params: map[string]any{}}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageUpdate_YumExitCode100(t *testing.T) {
	h := NewPackageUpdateHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("yum", executor.MockCommandResponse{
		Output: []byte("updates available"), ExitCode: 100,
	})
	action := playbook.Action{ID: "test", Type: "package.update", Params: map[string]any{}}
	result := h.Execute(action, yumContext(cmd))
	if result.Status != "success" {
		t.Fatalf("yum exit 100 should be OK, got %s: %s", result.Status, result.Error)
	}
}

func TestPackageUpdate_DnfExitCode100(t *testing.T) {
	h := NewPackageUpdateHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dnf", executor.MockCommandResponse{
		Output: []byte("updates available"), ExitCode: 100,
	})
	action := playbook.Action{ID: "test", Type: "package.update", Params: map[string]any{}}
	result := h.Execute(action, dnfContext(cmd))
	if result.Status != "success" {
		t.Fatalf("dnf exit 100 should be OK, got %s: %s", result.Status, result.Error)
	}
}

// --- PackageUpgradeHandler ---

func TestPackageUpgrade_UnknownPackageManager(t *testing.T) {
	h := NewPackageUpgradeHandler()
	cmd := executor.NewMockCommandRunner()
	action := playbook.Action{ID: "test", Type: "package.upgrade", Params: map[string]any{}}
	result := h.Execute(action, unknownPMContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageUpgrade_DryRun(t *testing.T) {
	h := NewPackageUpgradeHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := aptContext(cmd)
	ctx.DryRun = true
	action := playbook.Action{ID: "test", Type: "package.upgrade", Params: map[string]any{}}
	result := h.Execute(action, ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestPackageUpgrade_AptSucceeds(t *testing.T) {
	h := NewPackageUpgradeHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executor.MockCommandResponse{ExitCode: 0})
	action := playbook.Action{ID: "test", Type: "package.upgrade", Params: map[string]any{}}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	// Verify DEBIAN_FRONTEND env was set
	calls := cmd.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Opts == nil || len(calls[0].Opts.Env) == 0 {
		t.Error("upgrade call should have DEBIAN_FRONTEND env")
	}
}

func TestPackageUpgrade_AptFails(t *testing.T) {
	h := NewPackageUpgradeHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executor.MockCommandResponse{
		Output: []byte("E: error"), ExitCode: 1,
	})
	action := playbook.Action{ID: "test", Type: "package.upgrade", Params: map[string]any{}}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageUpgrade_YumSucceeds(t *testing.T) {
	h := NewPackageUpgradeHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("yum", executor.MockCommandResponse{ExitCode: 0})
	action := playbook.Action{ID: "test", Type: "package.upgrade", Params: map[string]any{}}
	result := h.Execute(action, yumContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
}

func TestPackageUpgrade_DnfSucceeds(t *testing.T) {
	h := NewPackageUpgradeHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dnf", executor.MockCommandResponse{ExitCode: 0})
	action := playbook.Action{ID: "test", Type: "package.upgrade", Params: map[string]any{}}
	result := h.Execute(action, dnfContext(cmd))
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
}
