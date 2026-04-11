//go:build integration

package agent_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook/builder"
)

// TestFile_Content_Creates verifies that modules.File with Content creates a
// file on the remote system with the exact inline content specified (FR-40).
func TestFile_Content_Creates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-content-creates")
	if err := modules.File(b, "/tmp/file-content.txt",
		modules.Content("hello from nestor\n")); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "grep", "hello from nestor", "/tmp/file-content.txt")
	if err != nil {
		t.Fatalf("exec grep: %v", err)
	}
	if code != 0 {
		t.Fatal("expected inline content not found in /tmp/file-content.txt")
	}
}

// TestFile_Content_Idempotent verifies that running the same file.content
// playbook twice produces changed: 0 on the second run (FR-41).
func TestFile_Content_Idempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-content-idempotent")
	if err := modules.File(b, "/tmp/file-idempotent.txt",
		modules.Content("idempotent content\n")); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	// First run.
	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d on first run:\n%s", code, out)
	}

	// Second run — must report no changes.
	code, out = runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d on second run:\n%s", code, out)
	}
	if !strings.Contains(out, "changed: 0") {
		t.Logf("second run output:\n%s", out)
		t.Error("expected 'changed: 0' in second run output (idempotency)")
	}
}

// TestFile_Template verifies that modules.File with FromTemplate renders a
// Go text/template and writes the substituted result to the destination (FR-42).
func TestFile_Template(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-template")
	if err := modules.File(b, "/tmp/rendered.conf",
		modules.FromTemplate("testdata/config.tmpl"),
		modules.TemplateVars(map[string]string{"Host": "db.example.com"})); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "grep", "server=db.example.com", "/tmp/rendered.conf")
	if err != nil {
		t.Fatalf("exec grep: %v", err)
	}
	if code != 0 {
		t.Fatal("rendered template variable not found in /tmp/rendered.conf")
	}
}

// TestFile_Upload verifies that modules.File with FromFile uploads a local file
// to the correct destination path with matching content (FR-43).
func TestFile_Upload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-upload")
	if err := modules.File(b, "/tmp/uploaded.txt",
		modules.FromFile("testdata/upload-me.txt")); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "grep", "uploaded by nestor", "/tmp/uploaded.txt")
	if err != nil {
		t.Fatalf("exec grep: %v", err)
	}
	if code != 0 {
		t.Fatal("expected content not found in /tmp/uploaded.txt")
	}
}

// TestFile_Symlink verifies that modules.Symlink creates a symbolic link
// pointing to the correct target, as confirmed by readlink (FR-44).
func TestFile_Symlink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const target = "/tmp/file-content.txt"
	const link = "/tmp/test-link"

	b := builder.New("file-symlink")
	if err := modules.Symlink(b, link, target); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, out, err := suite.exec(ctx, "readlink", link)
	if err != nil {
		t.Fatalf("exec readlink: %v", err)
	}
	if code != 0 {
		t.Fatalf("readlink exited %d: symlink %s does not exist", code, link)
	}
	if !strings.Contains(out, target) {
		t.Errorf("readlink output %q does not contain target %q", out, target)
	}
}

// TestFile_Remove verifies that modules.Remove deletes a file and, as part of
// the same test, recursively removes a non-empty directory (FR-45).
func TestFile_Remove(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Set up: create a file and a non-empty directory tree.
	for _, cmd := range [][]string{
		{"touch", "/tmp/remove-me.txt"},
		{"mkdir", "-p", "/tmp/remove-dir/subdir"},
		{"touch", "/tmp/remove-dir/subdir/file.txt"},
	} {
		if code, _, err := suite.exec(ctx, cmd...); err != nil || code != 0 {
			t.Fatalf("pre-condition %v: code=%d err=%v", cmd, code, err)
		}
	}

	b := builder.New("file-remove")
	if err := modules.Remove(b, "/tmp/remove-me.txt"); err != nil {
		t.Fatalf("build playbook (file remove): %v", err)
	}
	if err := modules.Remove(b, "/tmp/remove-dir", modules.Recursive(true)); err != nil {
		t.Fatalf("build playbook (dir remove): %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	for _, path := range []string{"/tmp/remove-me.txt", "/tmp/remove-dir"} {
		code, _, err := suite.exec(ctx, "stat", path)
		if err != nil {
			t.Fatalf("exec stat %s: %v", path, err)
		}
		if code == 0 {
			t.Errorf("%s still exists after modules.Remove", path)
		}
	}
}

// TestFile_Directory verifies that modules.Directory creates a directory with
// the specified permissions, as confirmed by stat (FR-46).
func TestFile_Directory(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-directory")
	if err := modules.Directory(b, "/tmp/test-dir", modules.Mode(0750)); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, out, err := suite.exec(ctx, "stat", "-c", "%a", "/tmp/test-dir")
	if err != nil {
		t.Fatalf("exec stat: %v", err)
	}
	if code != 0 {
		t.Fatal("directory /tmp/test-dir does not exist after modules.Directory")
	}
	if !strings.Contains(out, "750") {
		t.Errorf("expected mode 750, stat output: %q", out)
	}
}

// TestFile_Mode verifies that modules.File with Mode sets the correct file
// permissions on the remote system, as confirmed by stat (FR-47).
func TestFile_Mode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-mode")
	if err := modules.File(b, "/tmp/mode-file.txt",
		modules.Content("mode test\n"),
		modules.Mode(0600)); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, out, err := suite.exec(ctx, "stat", "-c", "%a", "/tmp/mode-file.txt")
	if err != nil {
		t.Fatalf("exec stat: %v", err)
	}
	if code != 0 {
		t.Fatal("file /tmp/mode-file.txt does not exist after modules.File")
	}
	if !strings.Contains(out, "600") {
		t.Errorf("expected mode 600, stat output: %q", out)
	}
}

// TestFile_Owner verifies that modules.File with Owner sets the correct owner
// and group using built-in system accounts, as confirmed by stat (FR-48).
func TestFile_Owner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("file-owner")
	if err := modules.File(b, "/tmp/owned-file.txt",
		modules.Content("owner test\n"),
		modules.Owner("root", "daemon")); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, out, err := suite.exec(ctx, "stat", "-c", "%U:%G", "/tmp/owned-file.txt")
	if err != nil {
		t.Fatalf("exec stat: %v", err)
	}
	if code != 0 {
		t.Fatal("file /tmp/owned-file.txt does not exist after modules.File")
	}
	if !strings.Contains(out, "root:daemon") {
		t.Errorf("expected owner root:daemon, stat output: %q", out)
	}
}
