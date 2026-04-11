//go:build integration

package agent_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook/builder"
)

func TestPackageInstall_InstallsPackage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	b := builder.New("test-install-" + fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := modules.Package(b, "install", "curl"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	pkg := buildPlaybookArchive(t, b.Playbook())

	if err := suite.copyToContainer(ctx, pkg.ArchivePath, "/tmp/test-install.tar.gz"); err != nil {
		t.Fatalf("copy archive: %v", err)
	}
	if err := suite.copyToContainer(ctx, pkg.SignaturePath, "/tmp/test-install.sig"); err != nil {
		t.Fatalf("copy signature: %v", err)
	}

	code, out, err := suite.exec(ctx,
		"/usr/local/bin/nestor-agent", "-playbook", "/tmp/test-install.tar.gz")
	if err != nil {
		t.Fatalf("exec agent: %v", err)
	}
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	// Verify curl is installed.
	code, _, err = suite.exec(ctx, "dpkg", "-l", "curl")
	if err != nil {
		t.Fatalf("exec dpkg: %v", err)
	}
	if code != 0 {
		t.Fatal("curl is not installed after agent run")
	}
}

func TestPackageInstall_Idempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	b := builder.New("test-idempotent-" + fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := modules.Package(b, "install", "curl"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	pkg := buildPlaybookArchive(t, b.Playbook())

	if err := suite.copyToContainer(ctx, pkg.ArchivePath, "/tmp/test-idempotent.tar.gz"); err != nil {
		t.Fatalf("copy archive: %v", err)
	}
	if err := suite.copyToContainer(ctx, pkg.SignaturePath, "/tmp/test-idempotent.sig"); err != nil {
		t.Fatalf("copy signature: %v", err)
	}

	// First run.
	code, out, err := suite.exec(ctx,
		"/usr/local/bin/nestor-agent", "-playbook", "/tmp/test-idempotent.tar.gz")
	if err != nil {
		t.Fatalf("exec agent (first run): %v", err)
	}
	if code != 0 {
		t.Fatalf("agent exited %d on first run:\n%s", code, out)
	}

	// Second run.
	code, out, err = suite.exec(ctx,
		"/usr/local/bin/nestor-agent", "-playbook", "/tmp/test-idempotent.tar.gz")
	if err != nil {
		t.Fatalf("exec agent (second run): %v", err)
	}
	if code != 0 {
		t.Fatalf("agent exited %d on second run:\n%s", code, out)
	}

	if !strings.Contains(out, "changed: 0") {
		t.Logf("second run output:\n%s", out)
		t.Error("expected 'changed: 0' in second run output (idempotency)")
	}
}
