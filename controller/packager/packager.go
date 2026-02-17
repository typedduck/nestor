package packager

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/typedduck/nestor/playbook"
)

// Package represents a packaged playbook ready for transfer
type Package struct {
	ArchivePath   string // Path to the tar.gz file
	SignaturePath string // Path to archive signature file
	ManifestPath  string // Path to the manifest file
	PlaybookPath  string // Path to playbook.json
	UploadDir     string // Directory containing files to upload
}

// Packager creates playbook archives
type Packager struct {
	workDir string
}

// New creates a new packager
func New(workDir string) *Packager {
	return &Packager{
		workDir: workDir,
	}
}

// Package creates a playbook archive from a playbook
//
// The archive contains:
//   - playbook.json: The action definitions
//   - manifest: SHA256 checksums of all files
//   - upload/: Directory with files referenced by actions
func (p *Packager) Package(pb *playbook.Playbook) (*Package, error) {
	// Create working directory
	pkgDir := filepath.Join(p.workDir, "playbook-"+pb.Name)
	if err := os.RemoveAll(pkgDir); err != nil {
		return nil, fmt.Errorf("failed to remove old package directory: %w", err)
	}
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create package directory: %w", err)
	}
	log.Printf("[INFO ] working directory: %s", pkgDir)

	pkg := &Package{
		PlaybookPath:  filepath.Join(pkgDir, "playbook.json"),
		ManifestPath:  filepath.Join(pkgDir, "manifest"),
		UploadDir:     filepath.Join(pkgDir, "upload"),
		ArchivePath:   filepath.Join(p.workDir, pb.Name+".tar.gz"),
		SignaturePath: filepath.Join(p.workDir, pb.Name+".sig"),
	}

	// Create upload directory
	if err := os.MkdirAll(pkg.UploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Write playbook.json
	if err := p.writePlaybookJSON(pb, pkg.PlaybookPath); err != nil {
		return nil, fmt.Errorf("failed to write playbook.json: %w", err)
	}

	// Collect files to include in the archive
	files := []string{"playbook.json"}

	// Copy files referenced by actions to upload directory
	uploadedFiles, err := p.collectUploadFiles(pb, pkg.UploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to collect upload files: %w", err)
	}
	files = append(files, uploadedFiles...)

	// Generate manifest with checksums
	if err := p.generateManifest(pkgDir, files, pkg.ManifestPath); err != nil {
		return nil, fmt.Errorf("failed to generate manifest: %w", err)
	}
	files = append(files, "manifest")

	// Create tar.gz archive
	if err := p.createArchive(pkgDir, files, pkg.ArchivePath); err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}

	return pkg, nil
}

// writePlaybookJSON writes the playbook to JSON format
func (p *Packager) writePlaybookJSON(pb *playbook.Playbook, path string) error {
	data, err := json.MarshalIndent(pb, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// collectUploadFiles collects files referenced by actions
func (p *Packager) collectUploadFiles(pb *playbook.Playbook, uploadDir string) ([]string, error) {
	files := []string{}
	seen := make(map[string]bool)

	for _, action := range pb.Actions {
		// Check for file upload actions
		if action.Type == "file.upload" {
			source, ok := action.Params["source"].(string)
			if !ok {
				continue
			}

			// Skip if already copied
			if seen[source] {
				continue
			}
			seen[source] = true

			// Copy file to upload directory
			destName := filepath.Base(source)
			destPath := filepath.Join(uploadDir, destName)

			if err := copyFile(source, destPath); err != nil {
				return nil, fmt.Errorf("failed to copy %s: %w", source, err)
			}

			files = append(files, "upload/"+destName)

			// Update action to reference the file in the upload directory
			action.Params["source"] = "upload/" + destName
		}

		// Check for template actions
		if action.Type == "file.template" {
			source, ok := action.Params["source"].(string)
			if !ok {
				continue
			}

			if seen[source] {
				continue
			}
			seen[source] = true

			// Copy template to upload directory
			destName := filepath.Base(source)
			destPath := filepath.Join(uploadDir, destName)

			if err := copyFile(source, destPath); err != nil {
				return nil, fmt.Errorf("failed to copy %s: %w", source, err)
			}

			files = append(files, "upload/"+destName)

			// Update action to reference the file in the upload directory
			action.Params["source"] = "upload/" + destName
		}
	}

	return files, nil
}

// generateManifest generates a manifest file with SHA256 checksums
func (p *Packager) generateManifest(baseDir string, files []string, manifestPath string) error {
	f, err := os.Create(manifestPath)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, file := range files {
		filePath := filepath.Join(baseDir, file)
		checksum, err := computeChecksum(filePath)
		if err != nil {
			return fmt.Errorf("failed to compute checksum for %s: %w", file, err)
		}

		// Write in format: "checksum  filepath"
		if _, err := fmt.Fprintf(f, "%s  %s\n", checksum, file); err != nil {
			return err
		}
	}

	return nil
}

// createArchive creates a tar.gz archive from the specified files
func (p *Packager) createArchive(baseDir string, files []string, archivePath string) error {
	// Create archive file
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	defer archiveFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add each file to the archive
	for _, file := range files {
		filePath := filepath.Join(baseDir, file)

		if err := addFileToArchive(tarWriter, filePath, file); err != nil {
			return fmt.Errorf("failed to add %s to archive: %w", file, err)
		}
	}

	return nil
}

// addFileToArchive adds a file to a tar archive
func addFileToArchive(tw *tar.Writer, filePath, nameInArchive string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	// Create tar header
	header := &tar.Header{
		Name:    nameInArchive,
		Mode:    int64(stat.Mode()),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}

	// Write header
	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// Write file content
	_, err = io.Copy(tw, file)
	return err
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// computeChecksum computes the SHA256 checksum of a file
func computeChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
