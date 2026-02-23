package validator

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/typedduck/nestor/agent/executor"
)

func setupExtractDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func writeFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:])
}

// newTestValidator creates a Validator pointing at a temp dir for both the
// (fake) archive path and the extract path, using real OS file operations.
// The same dir is used for both so that sig files and extracted files can
// coexist without separate setup.
func newTestValidator(t *testing.T) (*Validator, string) {
	t.Helper()
	dir := setupExtractDir(t)
	v := &Validator{
		playbookPath: filepath.Join(dir, "test-playbook.tar.gz"),
		extractPath:  dir,
		fs:           executor.OSFileSystem{},
	}
	return v, dir
}

func TestValidateSignature_Success(t *testing.T) {
	v, dir := newTestValidator(t)
	// Sig file must be beside the archive: <dir>/test-playbook.sig
	writeFile(t, filepath.Join(dir, "test-playbook.sig"), []byte("fake-sig"))

	if err := v.ValidateSignature(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateSignature_Missing(t *testing.T) {
	v, _ := newTestValidator(t)

	err := v.ValidateSignature()
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}

func TestValidateManifest_Success(t *testing.T) {
	v, dir := newTestValidator(t)

	// Create a test file
	content := []byte("server { listen 80; }")
	writeFile(t, filepath.Join(dir, "nginx.conf"), content)

	// Create manifest with correct checksum
	checksum := sha256hex(content)
	manifest := fmt.Sprintf("%s  nginx.conf\n", checksum)
	writeFile(t, filepath.Join(dir, "manifest"), []byte(manifest))

	if err := v.ValidateManifest(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateManifest_ChecksumMismatch(t *testing.T) {
	v, dir := newTestValidator(t)

	writeFile(t, filepath.Join(dir, "nginx.conf"), []byte("actual content"))

	manifest := "0000000000000000000000000000000000000000000000000000000000000000  nginx.conf\n"
	writeFile(t, filepath.Join(dir, "manifest"), []byte(manifest))

	err := v.ValidateManifest()
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestValidateManifest_MissingFile(t *testing.T) {
	v, dir := newTestValidator(t)

	manifest := "abc123  nonexistent.conf\n"
	writeFile(t, filepath.Join(dir, "manifest"), []byte(manifest))

	err := v.ValidateManifest()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestValidateManifest_MissingManifest(t *testing.T) {
	v, _ := newTestValidator(t)

	err := v.ValidateManifest()
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestValidateManifest_InvalidFormat(t *testing.T) {
	v, dir := newTestValidator(t)

	writeFile(t, filepath.Join(dir, "manifest"), []byte("bad line format here too many fields\n"))

	err := v.ValidateManifest()
	if err == nil {
		t.Fatal("expected error for invalid manifest format")
	}
}

func TestValidateManifest_MultipleFiles(t *testing.T) {
	v, dir := newTestValidator(t)

	file1 := []byte("file one content")
	file2 := []byte("file two content")
	writeFile(t, filepath.Join(dir, "file1.txt"), file1)
	writeFile(t, filepath.Join(dir, "file2.txt"), file2)

	manifest := fmt.Sprintf("%s  file1.txt\n%s  file2.txt\n",
		sha256hex(file1), sha256hex(file2))
	writeFile(t, filepath.Join(dir, "manifest"), []byte(manifest))

	if err := v.ValidateManifest(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateManifest_SkipsEmptyLines(t *testing.T) {
	v, dir := newTestValidator(t)

	content := []byte("test")
	writeFile(t, filepath.Join(dir, "test.txt"), content)

	manifest := fmt.Sprintf("\n%s  test.txt\n\n", sha256hex(content))
	writeFile(t, filepath.Join(dir, "manifest"), []byte(manifest))

	if err := v.ValidateManifest(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestNew_NilFSUsesDefault(t *testing.T) {
	v := New("/tmp/test.tar.gz", "/tmp/extract", nil)
	if v.fs == nil {
		t.Fatal("expected non-nil fs with nil input")
	}
}
