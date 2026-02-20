package packager_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/typedduck/nestor/controller/packager"
	"github.com/typedduck/nestor/playbook"
)

// --- helpers ---

func newPlaybook(name string) *playbook.Playbook {
	pb := playbook.New(name)
	pb.Controller = "test-controller"
	return pb
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:])
}

// archiveEntries opens a tar.gz and returns a sorted list of entry names.
func archiveEntries(t *testing.T, archivePath string) []string {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var names []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		names = append(names, hdr.Name)
	}
	sort.Strings(names)
	return names
}

// archiveFileContent returns the content of a named entry from a tar.gz archive.
func archiveFileContent(t *testing.T, archivePath, name string) []byte {
	t.Helper()
	f, err := os.Open(archivePath)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		if hdr.Name == name {
			data, err := io.ReadAll(tr)
			if err != nil {
				t.Fatalf("read archive entry %q: %v", name, err)
			}
			return data
		}
	}
	t.Fatalf("entry %q not found in archive %s", name, archivePath)
	return nil
}

// parseManifest reads a manifest file and returns a filename→checksum map.
func parseManifest(t *testing.T, manifestPath string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	result := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			t.Fatalf("invalid manifest line: %q", line)
		}
		result[fields[1]] = fields[0]
	}
	return result
}

// writeSourceFile creates a file with the given content and returns its path.
func writeSourceFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write source file: %v", err)
	}
	return path
}

// --- tests ---

func TestPackage_BasicPlaybook(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("myplaybook")
	pb.AddAction(playbook.Action{
		Type:   "package.install",
		Params: map[string]any{"packages": []string{"nginx"}},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	entries := archiveEntries(t, pkg.ArchivePath)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (manifest, playbook.json), got %d: %v", len(entries), entries)
	}
	if entries[0] != "manifest" || entries[1] != "playbook.json" {
		t.Fatalf("unexpected archive entries: %v", entries)
	}
}

func TestPackage_ReturnedPathsExist(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	for _, path := range []string{pkg.ArchivePath, pkg.ManifestPath, pkg.PlaybookPath} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected path to exist: %s: %v", path, err)
		}
	}
}

func TestPackage_ArchiveNameMatchesPlaybook(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("my-server")

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	expected := filepath.Join(workDir, "my-server.tar.gz")
	if pkg.ArchivePath != expected {
		t.Errorf("expected archive at %s, got %s", expected, pkg.ArchivePath)
	}
}

func TestPackage_PlaybookJSONIsValidJSON(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")
	pb.SetEnv("KEY", "VALUE")
	pb.AddAction(playbook.Action{
		Type:   "package.install",
		Params: map[string]any{"packages": []string{"curl"}},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	data, err := os.ReadFile(pkg.PlaybookPath)
	if err != nil {
		t.Fatalf("read playbook.json: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("invalid JSON in playbook.json: %v", err)
	}
	if decoded["name"] != "mypb" {
		t.Errorf("expected name=mypb, got %v", decoded["name"])
	}
	if decoded["version"] != "1.0" {
		t.Errorf("expected version=1.0, got %v", decoded["version"])
	}
}

func TestPackage_PlaybookJSONInArchiveMatchesDisk(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	fromDisk, err := os.ReadFile(pkg.PlaybookPath)
	if err != nil {
		t.Fatalf("read playbook.json: %v", err)
	}
	fromArchive := archiveFileContent(t, pkg.ArchivePath, "playbook.json")

	if string(fromDisk) != string(fromArchive) {
		t.Error("playbook.json in archive differs from the file on disk")
	}
}

func TestPackage_ManifestChecksumForPlaybookJSON(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	data, err := os.ReadFile(pkg.PlaybookPath)
	if err != nil {
		t.Fatalf("read playbook.json: %v", err)
	}

	manifest := parseManifest(t, pkg.ManifestPath)
	want := sha256hex(data)
	if got := manifest["playbook.json"]; got != want {
		t.Errorf("playbook.json checksum mismatch: got %s, want %s", got, want)
	}
}

func TestPackage_ManifestInArchive(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	entries := archiveEntries(t, pkg.ArchivePath)
	if slices.Contains(entries, "manifest") {
		return
	}
	t.Fatalf("manifest not found in archive; entries: %v", entries)
}

func TestPackage_FileUploadCopiedToArchive(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()
	content := []byte("server { listen 80; }")
	src := writeSourceFile(t, srcDir, "nginx.conf", content)

	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.upload",
		Params: map[string]any{"source": src, "destination": "/etc/nginx/nginx.conf"},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	entries := archiveEntries(t, pkg.ArchivePath)
	if slices.Contains(entries, "upload/nginx.conf") {
		// Verify content
		got := archiveFileContent(t, pkg.ArchivePath, "upload/nginx.conf")
		if string(got) != string(content) {
			t.Errorf("content mismatch: got %q, want %q", got, content)
		}
		return
	}
	t.Fatalf("upload/nginx.conf not found in archive; entries: %v", entries)
}

func TestPackage_FileTemplateCopiedToArchive(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()
	src := writeSourceFile(t, srcDir, "greeting.tmpl", []byte("Hello {{ .Name }}"))

	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.template",
		Params: map[string]any{"source": src, "destination": "/etc/greeting"},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	entries := archiveEntries(t, pkg.ArchivePath)
	if slices.Contains(entries, "upload/greeting.tmpl") {
		return
	}
	t.Fatalf("upload/greeting.tmpl not found in archive; entries: %v", entries)
}

func TestPackage_UploadSourceRewrittenInPlaybookJSON(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()
	src := writeSourceFile(t, srcDir, "app.conf", []byte("config"))

	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.upload",
		Params: map[string]any{"source": src, "destination": "/etc/app.conf"},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	data, err := os.ReadFile(pkg.PlaybookPath)
	if err != nil {
		t.Fatalf("read playbook.json: %v", err)
	}

	var decoded struct {
		Actions []struct {
			Params map[string]any `json:"params"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal playbook.json: %v", err)
	}
	if len(decoded.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(decoded.Actions))
	}

	source := decoded.Actions[0].Params["source"]
	if source != "upload/app.conf" {
		t.Errorf("source param in playbook.json: got %q, want %q", source, "upload/app.conf")
	}
}

func TestPackage_TemplateSourceRewrittenInPlaybookJSON(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()
	src := writeSourceFile(t, srcDir, "page.tmpl", []byte("{{ .Title }}"))

	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.template",
		Params: map[string]any{"source": src, "destination": "/etc/page"},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	data, err := os.ReadFile(pkg.PlaybookPath)
	if err != nil {
		t.Fatalf("read playbook.json: %v", err)
	}

	var decoded struct {
		Actions []struct {
			Params map[string]any `json:"params"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal playbook.json: %v", err)
	}

	source := decoded.Actions[0].Params["source"]
	if source != "upload/page.tmpl" {
		t.Errorf("source param in playbook.json: got %q, want %q", source, "upload/page.tmpl")
	}
}

func TestPackage_DuplicateSourcesDeduped(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()
	src := writeSourceFile(t, srcDir, "shared.conf", []byte("shared"))

	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.upload",
		Params: map[string]any{"source": src, "destination": "/etc/a.conf"},
	})
	pb.AddAction(playbook.Action{
		Type:   "file.upload",
		Params: map[string]any{"source": src, "destination": "/etc/b.conf"},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	var uploadCount int
	for _, e := range archiveEntries(t, pkg.ArchivePath) {
		if strings.HasPrefix(e, "upload/") {
			uploadCount++
		}
	}
	if uploadCount != 1 {
		t.Errorf("expected 1 upload entry (deduped), got %d", uploadCount)
	}
}

func TestPackage_ManifestIncludesUploadFiles(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()
	content := []byte("important config")
	src := writeSourceFile(t, srcDir, "service.conf", content)

	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.upload",
		Params: map[string]any{"source": src, "destination": "/etc/service.conf"},
	})

	pkg, err := packager.New(workDir).Package(pb)
	if err != nil {
		t.Fatalf("Package: %v", err)
	}

	manifest := parseManifest(t, pkg.ManifestPath)
	want := sha256hex(content)
	got, ok := manifest["upload/service.conf"]
	if !ok {
		t.Fatalf("upload/service.conf not in manifest; got: %v", manifest)
	}
	if got != want {
		t.Errorf("upload/service.conf checksum mismatch: got %s, want %s", got, want)
	}
}

func TestPackage_MissingSourceReturnsError(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")
	pb.AddAction(playbook.Action{
		Type:   "file.upload",
		Params: map[string]any{"source": "/nonexistent/file.conf", "destination": "/etc/file.conf"},
	})

	_, err := packager.New(workDir).Package(pb)
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestPackage_Idempotent(t *testing.T) {
	workDir := t.TempDir()
	pb := newPlaybook("mypb")
	p := packager.New(workDir)

	if _, err := p.Package(pb); err != nil {
		t.Fatalf("first Package: %v", err)
	}
	if _, err := p.Package(pb); err != nil {
		t.Fatalf("second Package: %v", err)
	}
}
