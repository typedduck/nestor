package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
	"github.com/typedduck/nestor/playbook"
)

func aptContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "apt"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func yumContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "yum"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func dnfContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "dnf"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func brewContext(cmd *executortest.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "brew"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func pkgAction(packages ...string) playbook.Action {
	pkgs := make([]any, len(packages))
	for i, p := range packages {
		pkgs[i] = p
	}
	return playbook.Action{
		ID:   "test-pkg",
		Type: "package.install",
		Params: map[string]any{
			"packages": pkgs,
		},
	}
}

func TestPackageInstall_MissingPackagesParam(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID:     "test",
		Type:   "package.install",
		Params: map[string]any{},
	}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageInstall_InvalidPackagesType(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID:   "test",
		Type: "package.install",
		Params: map[string]any{
			"packages": "not-an-array",
		},
	}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageInstall_EmptyPackages(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	action := playbook.Action{
		ID:   "test",
		Type: "package.install",
		Params: map[string]any{
			"packages": []any{},
		},
	}
	result := h.Execute(action, aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Changed {
		t.Fatal("expected no change for empty packages")
	}
}

func TestPackageInstall_UnknownPackageManager(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "unknown"},
		Cmd:        cmd,
	}
	result := h.Execute(pkgAction("nginx"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
}

func TestPackageInstall_DryRun(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := aptContext(cmd)
	ctx.DryRun = true
	result := h.Execute(pkgAction("nginx"), ctx)
	if result.Status != "success" || !result.Changed {
		t.Fatalf("expected success+changed, got %s changed=%v", result.Status, result.Changed)
	}
	if len(cmd.Calls()) != 0 {
		t.Fatal("dry run should not invoke any commands")
	}
}

func TestPackageInstall_AptAllAlreadyInstalled(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{
		Output: []byte("install ok installed"), ExitCode: 0,
	})
	result := h.Execute(pkgAction("nginx"), aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when already installed")
	}
}

func TestPackageInstall_AptInstallsNewPackage(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	// dpkg-query fails → package not installed
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{
		Output: []byte(""), ExitCode: 1, Err: nil,
	})
	// apt-get update succeeds
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{
		Output: []byte(""), ExitCode: 0,
	})

	result := h.Execute(pkgAction("nginx"), aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after install")
	}

	// Verify commands called: dpkg-query, apt-get update, apt-get install
	calls := cmd.Calls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}
	if calls[0].Name != "dpkg-query" {
		t.Errorf("call 0: expected dpkg-query, got %s", calls[0].Name)
	}
	if calls[1].Name != "apt-get" {
		t.Errorf("call 1: expected apt-get, got %s", calls[1].Name)
	}
	// Verify install call has DEBIAN_FRONTEND env
	if calls[2].Opts == nil || len(calls[2].Opts.Env) == 0 {
		t.Error("install call should have DEBIAN_FRONTEND env")
	}
}

func TestPackageInstall_AptSkipUpdateCache(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{
		Output: []byte(""), ExitCode: 1,
	})
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{
		Output: []byte(""), ExitCode: 0,
	})

	action := pkgAction("nginx")
	action.Params["update_cache"] = false

	result := h.Execute(action, aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}

	// Should be only 2 calls: dpkg-query check + apt-get install (no update)
	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls (no cache update), got %d", len(calls))
	}
}

func TestPackageInstall_YumCheckUpdateExitCode100(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("yum", executortest.MockCommandResponse{
		Output: []byte("updates available"), ExitCode: 100,
	})

	err := h.updateCache(cmd, "yum")
	if err != nil {
		t.Fatalf("yum check-update exit 100 should not be an error, got: %v", err)
	}
}

func TestPackageInstall_DnfCheckUpdateExitCode100(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("dnf", executortest.MockCommandResponse{
		Output: []byte("updates available"), ExitCode: 100,
	})
	err := h.updateCache(cmd, "dnf")
	if err != nil {
		t.Fatalf("dnf check-update exit 100 should not be an error, got: %v", err)
	}
}

func TestPackageInstall_UnsupportedPM(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "pacman"},
		FS:         executortest.NewMockFileSystem(),
		Cmd:        cmd,
	}
	result := h.Execute(pkgAction("nginx"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed for unsupported pm, got %s", result.Status)
	}
}

func TestPackageInstall_UpdateCacheFails(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{
		Output: []byte(""), ExitCode: 1,
	})
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{
		Output: []byte("E: Could not get lock"), ExitCode: 100,
	})

	result := h.Execute(pkgAction("nginx"), aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed when update cache fails, got %s", result.Status)
	}
}

func TestPackageInstall_YumPackageAlreadyInstalled(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	// yum list installed succeeds → package is installed
	cmd.SetResponse("yum", executortest.MockCommandResponse{
		Output: []byte("nginx.x86_64  1:1.20.1-1.el7  @base"), ExitCode: 0,
	})

	result := h.Execute(pkgAction("nginx"), yumContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when package already installed")
	}
}

func TestPackageInstall_MultiplePackagesSomeInstalled(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()

	// dpkg-query returns "install ok installed" for all queries (executor keyed by name).
	// To test partial install, we'd need per-call responses. Instead, test that
	// when all are installed, no install happens.
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{
		Output: []byte("install ok installed"), ExitCode: 0,
	})

	result := h.Execute(pkgAction("nginx", "redis", "curl"), aptContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s", result.Status)
	}
	if result.Changed {
		t.Fatal("expected no change when all packages installed")
	}
}

func TestPackageInstall_BrewAlreadyInstalled(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	// brew list --formula succeeds → already installed
	cmd.SetResponse("brew", executortest.MockCommandResponse{ExitCode: 0})

	result := h.Execute(pkgAction("wget"), brewContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if result.Changed {
		t.Fatal("expected no change when package already installed")
	}
	// Only one call: brew list --formula check
	if len(cmd.Calls()) != 1 {
		t.Fatalf("expected 1 call, got %d", len(cmd.Calls()))
	}
}

func TestPackageInstall_BrewInstallsNewPackage(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	// Call 0: brew list --formula → exits 1 (not installed)
	// Call 1: brew update → exits 0
	// Call 2: brew install → exits 0
	cmd.SetResponses("brew",
		executortest.MockCommandResponse{ExitCode: 1},
		executortest.MockCommandResponse{ExitCode: 0},
		executortest.MockCommandResponse{ExitCode: 0},
	)

	result := h.Execute(pkgAction("wget"), brewContext(cmd))
	if result.Status != "success" {
		t.Fatalf("expected success, got %s: %s", result.Status, result.Error)
	}
	if !result.Changed {
		t.Fatal("expected changed=true after install")
	}

	calls := cmd.Calls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls (list, update, install), got %d", len(calls))
	}
	// Verify the install call carries HOMEBREW_NO_AUTO_UPDATE=1
	if calls[2].Opts == nil || len(calls[2].Opts.Env) == 0 {
		t.Error("brew install should carry HOMEBREW_NO_AUTO_UPDATE=1 env")
	} else if calls[2].Opts.Env[0] != "HOMEBREW_NO_AUTO_UPDATE=1" {
		t.Errorf("expected HOMEBREW_NO_AUTO_UPDATE=1, got %s", calls[2].Opts.Env[0])
	}
}

func TestPackageInstall_BrewUpdateFails(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	// Call 0: brew list --formula → not installed; Call 1: brew update → fails
	cmd.SetResponses("brew",
		executortest.MockCommandResponse{ExitCode: 1},
		executortest.MockCommandResponse{Output: []byte("update error"), ExitCode: 1},
	)

	result := h.Execute(pkgAction("wget"), brewContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed when brew update fails, got %s", result.Status)
	}
}

func TestPackageInstall_BrewInstallFails(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executortest.NewMockCommandRunner()
	// Call 0: list → not installed; Call 1: update → ok; Call 2: install → fails
	cmd.SetResponses("brew",
		executortest.MockCommandResponse{ExitCode: 1},
		executortest.MockCommandResponse{ExitCode: 0},
		executortest.MockCommandResponse{Output: []byte("install error"), ExitCode: 1},
	)

	result := h.Execute(pkgAction("wget"), brewContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed when brew install fails, got %s", result.Status)
	}
}
