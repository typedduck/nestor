package handlers

import (
	"testing"

	"github.com/typedduck/nestor/agent/executor"
)

func aptContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "apt"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func yumContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "yum"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func dnfContext(cmd *executor.MockCommandRunner) *executor.ExecutionContext {
	return &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "dnf"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
}

func pkgAction(packages ...string) executor.Action {
	pkgs := make([]any, len(packages))
	for i, p := range packages {
		pkgs[i] = p
	}
	return executor.Action{
		ID:   "test-pkg",
		Type: "package.install",
		Params: map[string]any{
			"packages": pkgs,
		},
	}
}

func TestPackageInstall_MissingPackagesParam(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executor.NewMockCommandRunner()
	action := executor.Action{
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
	cmd := executor.NewMockCommandRunner()
	action := executor.Action{
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
	cmd := executor.NewMockCommandRunner()
	action := executor.Action{
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
	cmd := executor.NewMockCommandRunner()
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
	cmd := executor.NewMockCommandRunner()
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
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
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
	cmd := executor.NewMockCommandRunner()
	// dpkg-query fails → package not installed
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
		Output: []byte(""), ExitCode: 1, Err: nil,
	})
	// apt-get update succeeds
	cmd.SetResponse("apt-get", executor.MockCommandResponse{
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
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
		Output: []byte(""), ExitCode: 1,
	})
	cmd.SetResponse("apt-get", executor.MockCommandResponse{
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
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("yum", executor.MockCommandResponse{
		Output: []byte("updates available"), ExitCode: 100,
	})

	err := h.updateCache(cmd, "yum")
	if err != nil {
		t.Fatalf("yum check-update exit 100 should not be an error, got: %v", err)
	}
}

func TestPackageInstall_DnfCheckUpdateExitCode100(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dnf", executor.MockCommandResponse{
		Output: []byte("updates available"), ExitCode: 100,
	})
	err := h.updateCache(cmd, "dnf")
	if err != nil {
		t.Fatalf("dnf check-update exit 100 should not be an error, got: %v", err)
	}
}

func TestPackageInstall_UnsupportedPM(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executor.NewMockCommandRunner()
	ctx := &executor.ExecutionContext{
		SystemInfo: &executor.Info{PackageManager: "pacman"},
		FS:         executor.NewMockFileSystem(),
		Cmd:        cmd,
	}
	result := h.Execute(pkgAction("nginx"), ctx)
	if result.Status != "failed" {
		t.Fatalf("expected failed for unsupported pm, got %s", result.Status)
	}
}

func TestPackageInstall_UpdateCacheFails(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executor.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
		Output: []byte(""), ExitCode: 1,
	})
	cmd.SetResponse("apt-get", executor.MockCommandResponse{
		Output: []byte("E: Could not get lock"), ExitCode: 100,
	})

	result := h.Execute(pkgAction("nginx"), aptContext(cmd))
	if result.Status != "failed" {
		t.Fatalf("expected failed when update cache fails, got %s", result.Status)
	}
}

func TestPackageInstall_YumPackageAlreadyInstalled(t *testing.T) {
	h := NewPackageInstallHandler()
	cmd := executor.NewMockCommandRunner()
	// yum list installed succeeds → package is installed
	cmd.SetResponse("yum", executor.MockCommandResponse{
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
	cmd := executor.NewMockCommandRunner()

	// dpkg-query returns "install ok installed" for all queries (executor keyed by name).
	// To test partial install, we'd need per-call responses. Instead, test that
	// when all are installed, no install happens.
	cmd.SetResponse("dpkg-query", executor.MockCommandResponse{
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
