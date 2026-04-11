//go:build integration

package agent_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/typedduck/nestor/controller/packager"
	"github.com/typedduck/nestor/controller/signer"
	"github.com/typedduck/nestor/playbook"
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

// runAgent packages pb, copies it into the container, executes the agent, and
// returns the exit code and combined output. A transport error is fatal.
func runAgent(t *testing.T, ctx context.Context, pb *playbook.Playbook) (int, string) {
	t.Helper()

	pkg := buildPlaybookArchive(t, pb)
	archiveDst := "/tmp/" + filepath.Base(pkg.ArchivePath)
	sigDst := "/tmp/" + filepath.Base(pkg.SignaturePath)

	if err := suite.copyToContainer(ctx, pkg.ArchivePath, archiveDst); err != nil {
		t.Fatalf("copy archive: %v", err)
	}
	if err := suite.copyToContainer(ctx, pkg.SignaturePath, sigDst); err != nil {
		t.Fatalf("copy signature: %v", err)
	}

	code, out, err := suite.exec(ctx, "/usr/local/bin/nestor-agent", "-playbook", archiveDst)
	if err != nil {
		t.Fatalf("exec agent: %v", err)
	}
	return code, out
}
