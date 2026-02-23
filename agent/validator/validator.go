package validator

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/typedduck/nestor/agent/executor"
)

// FileOpener abstracts file system operations for testability.
type FileOpener interface {
	Stat(path string) (os.FileInfo, error)
	Open(path string) (*os.File, error)
}

// Validator validates playbook integrity
type Validator struct {
	playbookPath string
	extractPath  string
	fs           FileOpener
}

// New creates a new validator. If fs is nil, real OS operations are used.
func New(playbookPath, extractPath string, fs FileOpener) *Validator {
	if fs == nil {
		fs = executor.OSFileSystem{}
	}

	return &Validator{
		playbookPath: playbookPath,
		extractPath:  extractPath,
		fs:           fs,
	}
}

// ValidateSignature validates the playbook signature.
// It expects a .sig file beside the archive (sibling file, same directory).
func (v *Validator) ValidateSignature() error {
	base := filepath.Base(v.playbookPath)
	name := strings.TrimSuffix(base, filepath.Ext(base)) // strip .gz
	name = strings.TrimSuffix(name, filepath.Ext(name))  // strip .tar
	sigPath := filepath.Join(filepath.Dir(v.playbookPath), name+".sig")

	if _, err := v.fs.Stat(sigPath); err != nil {
		return fmt.Errorf("signature file not found: %w", err)
	}

	// TODO: verify cryptographic signature
	return nil
}

// ValidateManifest validates the manifest checksums
func (v *Validator) ValidateManifest() error {
	manifestPath := filepath.Join(v.extractPath, "manifest")

	// Read manifest file
	f, err := v.fs.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to open manifest: %w", err)
	}
	defer f.Close()

	// Parse manifest and verify each file
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Parse line: "checksum  filepath"
		parts := strings.Fields(line)
		if len(parts) != 2 {
			return fmt.Errorf("invalid manifest format at line %d", lineNum)
		}

		expectedChecksum := parts[0]
		filepath := parts[1]

		// Compute actual checksum
		actualChecksum, err := v.computeChecksum(filepath)
		if err != nil {
			return fmt.Errorf("failed to compute checksum for %s: %w", filepath, err)
		}

		// Verify checksum
		if actualChecksum != expectedChecksum {
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
				filepath, expectedChecksum, actualChecksum)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading manifest: %w", err)
	}

	return nil
}

// computeChecksum computes the SHA256 checksum of a file
func (v *Validator) computeChecksum(relPath string) (string, error) {
	fullPath := filepath.Join(v.extractPath, relPath)

	f, err := v.fs.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
