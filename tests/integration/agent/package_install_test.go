//go:build integration

package agent_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/typedduck/nestor/controller/packager"
	"github.com/typedduck/nestor/controller/signer"
	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/playbook/builder"
)

// buildPlaybookArchive creates a signed playbook archive in a temp directory.
// It generates a fresh ed25519 key pair, packages the playbook, and signs it.
func buildPlaybookArchive(t *testing.T, pb *playbook.Playbook) *packager.Package {
	t.Helper()

	workDir := t.TempDir()

	// Generate an ed25519 key pair and write the private key as PKCS8 PEM.
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}

	pkcs8Bytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		t.Fatalf("marshal pkcs8: %v", err)
	}

	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8Bytes})
	keyFile, err := os.CreateTemp(workDir, "signing-key-*.pem")
	if err != nil {
		t.Fatalf("create key file: %v", err)
	}
	if _, err := keyFile.Write(privPEM); err != nil {
		keyFile.Close()
		t.Fatalf("write key file: %v", err)
	}
	keyFile.Close()

	// Package the playbook.
	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("package playbook: %v", err)
	}

	// Sign the archive.
	s, err := signer.New(keyFile.Name())
	if err != nil {
		t.Fatalf("create signer: %v", err)
	}
	if err := s.Sign(pkg); err != nil {
		t.Fatalf("sign package: %v", err)
	}

	return pkg
}

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
