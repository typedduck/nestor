// Package executortest provides mock implementations of executor.FileSystem and
// executor.CommandRunner for use in tests. It is never imported by production code.
package executortest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/typedduck/nestor/agent/executor"
)

// ============================================================
// MockFileSystem
// ============================================================

// MockFileSystem implements executor.FileSystem using an in-memory file tree.
// All paths are cleaned before use. Safe for concurrent access.
type MockFileSystem struct {
	mu    sync.RWMutex
	files map[string]*mockFile
}

type mockFile struct {
	data  []byte
	mode  os.FileMode
	uid   int
	gid   int
	isDir bool
}

// NewMockFileSystem creates an empty in-memory file system.
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{files: make(map[string]*mockFile)}
}

// AddFile pre-populates a file with content and mode.
func (m *MockFileSystem) AddFile(path string, data []byte, mode os.FileMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[filepath.Clean(path)] = &mockFile{data: data, mode: mode}
}

// AddDir pre-populates a directory entry.
func (m *MockFileSystem) AddDir(path string, mode os.FileMode) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[filepath.Clean(path)] = &mockFile{isDir: true, mode: mode | os.ModeDir}
}

func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[filepath.Clean(path)]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}
	if f.isDir {
		return nil, &os.PathError{Op: "read", Path: path, Err: fmt.Errorf("is a directory")}
	}
	cp := make([]byte, len(f.data))
	copy(cp, f.data)
	return cp, nil
}

func (m *MockFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clean := filepath.Clean(path)

	dir := filepath.Dir(clean)
	if dir != "." && dir != "/" {
		if p, ok := m.files[dir]; !ok || !p.isDir {
			return &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
		}
	}

	cp := make([]byte, len(data))
	copy(cp, data)
	if f, ok := m.files[clean]; ok {
		f.data = cp
		f.mode = perm
	} else {
		m.files[clean] = &mockFile{data: cp, mode: perm}
	}
	return nil
}

func (m *MockFileSystem) Stat(path string) (os.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[filepath.Clean(path)]
	if !ok {
		return nil, &os.PathError{Op: "stat", Path: path, Err: os.ErrNotExist}
	}
	return &MockFileInfo{name: filepath.Base(path), file: f}, nil
}

func (m *MockFileSystem) Mkdir(path string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clean := filepath.Clean(path)

	if _, ok := m.files[clean]; ok {
		return &os.PathError{Op: "mkdir", Path: path, Err: os.ErrExist}
	}

	dir := filepath.Dir(clean)
	if dir != "." && dir != "/" {
		if p, ok := m.files[dir]; !ok || !p.isDir {
			return &os.PathError{Op: "mkdir", Path: path, Err: os.ErrNotExist}
		}
	}

	m.files[clean] = &mockFile{isDir: true, mode: perm | os.ModeDir}
	return nil
}

func (m *MockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	clean := filepath.Clean(path)

	parts := strings.Split(clean, string(filepath.Separator))
	current := ""
	if filepath.IsAbs(clean) {
		current = "/"
	}
	for _, part := range parts {
		if part == "" {
			continue
		}
		if current == "" || current == "/" {
			current = current + part
		} else {
			current = current + string(filepath.Separator) + part
		}
		if existing, ok := m.files[current]; ok {
			if !existing.isDir {
				return &os.PathError{Op: "mkdir", Path: current, Err: fmt.Errorf("not a directory")}
			}
			continue
		}
		m.files[current] = &mockFile{isDir: true, mode: perm | os.ModeDir}
	}
	return nil
}

func (m *MockFileSystem) Chmod(path string, mode os.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[filepath.Clean(path)]
	if !ok {
		return &os.PathError{Op: "chmod", Path: path, Err: os.ErrNotExist}
	}
	if f.isDir {
		f.mode = mode | os.ModeDir
	} else {
		f.mode = mode
	}
	return nil
}

func (m *MockFileSystem) Chown(path string, uid, gid int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, ok := m.files[filepath.Clean(path)]
	if !ok {
		return &os.PathError{Op: "chown", Path: path, Err: os.ErrNotExist}
	}
	f.uid = uid
	f.gid = gid
	return nil
}

// Open is not supported by the mock (it returns *os.File which requires a real file
// descriptor). Use ReadFile instead, or a real temp dir for tests that need Open.
func (m *MockFileSystem) Open(path string) (*os.File, error) {
	return nil, &os.PathError{Op: "open", Path: path,
		Err: fmt.Errorf("MockFileSystem.Open not supported; use ReadFile or a real temp dir")}
}

// FileContent returns the raw bytes stored for a file, for test assertions.
func (m *MockFileSystem) FileContent(path string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[filepath.Clean(path)]
	if !ok || f.isDir {
		return nil, false
	}
	return f.data, true
}

// FileMode returns the mode stored for a path, for test assertions.
func (m *MockFileSystem) FileMode(path string) (os.FileMode, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, ok := m.files[filepath.Clean(path)]
	if !ok {
		return 0, false
	}
	return f.mode, true
}

// FileOwner returns the uid and gid stored for a path, for test assertions.
func (m *MockFileSystem) FileOwner(path string) (uid, gid int, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	f, exists := m.files[filepath.Clean(path)]
	if !exists {
		return 0, 0, false
	}
	return f.uid, f.gid, true
}

// Exists reports whether a path exists in the mock filesystem.
func (m *MockFileSystem) Exists(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.files[filepath.Clean(path)]
	return ok
}

// ============================================================
// MockFileInfo
// ============================================================

// MockFileInfo implements os.FileInfo for mock filesystem entries.
type MockFileInfo struct {
	name string
	file *mockFile
}

func (fi *MockFileInfo) Name() string       { return fi.name }
func (fi *MockFileInfo) Size() int64        { return int64(len(fi.file.data)) }
func (fi *MockFileInfo) Mode() os.FileMode  { return fi.file.mode }
func (fi *MockFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *MockFileInfo) IsDir() bool        { return fi.file.isDir }
func (fi *MockFileInfo) Sys() any           { return nil }

// ============================================================
// MockCommandRunner
// ============================================================

// MockCommandCall records a single command invocation.
type MockCommandCall struct {
	Name string
	Args []string
	Opts *executor.CommandOpts
}

// MockCommandResponse defines what a mock command invocation should return.
type MockCommandResponse struct {
	Output   []byte
	ExitCode int
	Err      error
}

// MockCommandRunner implements executor.CommandRunner with canned responses.
// Commands are matched by name; use SetResponse / SetLookPath to configure.
type MockCommandRunner struct {
	mu        sync.Mutex
	calls     []MockCommandCall
	responses map[string]MockCommandResponse
	sequences map[string][]MockCommandResponse
	seqIdx    map[string]int
	lookPaths map[string]string
}

// NewMockCommandRunner creates a CommandRunner with no configured responses.
// By default all commands return a "command not found" error.
func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{
		responses: make(map[string]MockCommandResponse),
		sequences: make(map[string][]MockCommandResponse),
		seqIdx:    make(map[string]int),
		lookPaths: make(map[string]string),
	}
}

// SetResponse configures the response for a command name.
func (m *MockCommandRunner) SetResponse(name string, resp MockCommandResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[name] = resp
}

// SetResponses configures an ordered sequence of responses for a command name.
// Each invocation consumes the next response; once exhausted the last response
// repeats. Sequences take precedence over a single SetResponse for the same name.
func (m *MockCommandRunner) SetResponses(name string, resps ...MockCommandResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sequences[name] = resps
	m.seqIdx[name] = 0
}

// pickResponse returns the response for the next call to name (must be called with lock held).
func (m *MockCommandRunner) pickResponse(name string) (MockCommandResponse, bool) {
	if seq, ok := m.sequences[name]; ok && len(seq) > 0 {
		idx := m.seqIdx[name]
		if idx >= len(seq) {
			idx = len(seq) - 1
		}
		resp := seq[idx]
		if m.seqIdx[name] < len(seq)-1 {
			m.seqIdx[name]++
		}
		return resp, true
	}
	resp, ok := m.responses[name]
	return resp, ok
}

// SetLookPath makes LookPath succeed for the given command name.
func (m *MockCommandRunner) SetLookPath(name, path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lookPaths[name] = path
}

// Calls returns a copy of all recorded command invocations.
func (m *MockCommandRunner) Calls() []MockCommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]MockCommandCall, len(m.calls))
	copy(cp, m.calls)
	return cp
}

func (m *MockCommandRunner) record(name string, opts *executor.CommandOpts, args []string) {
	m.calls = append(m.calls, MockCommandCall{Name: name, Args: args, Opts: opts})
}

func (m *MockCommandRunner) Run(name string, opts *executor.CommandOpts, args ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record(name, opts, args)
	resp, ok := m.pickResponse(name)
	if !ok {
		return fmt.Errorf("command not found: %s", name)
	}
	if resp.Err != nil {
		return resp.Err
	}
	if resp.ExitCode != 0 {
		return fmt.Errorf("exit status %d", resp.ExitCode)
	}
	return nil
}

func (m *MockCommandRunner) CombinedOutput(name string, opts *executor.CommandOpts,
	args ...string) ([]byte, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record(name, opts, args)
	resp, ok := m.pickResponse(name)
	if !ok {
		return nil, -1, fmt.Errorf("command not found: %s", name)
	}
	if resp.Err != nil {
		return resp.Output, resp.ExitCode, resp.Err
	}
	if resp.ExitCode != 0 {
		return resp.Output, resp.ExitCode, fmt.Errorf("exit status %d", resp.ExitCode)
	}
	return resp.Output, 0, nil
}

func (m *MockCommandRunner) LookPath(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok := m.lookPaths[name]; ok {
		return p, nil
	}
	return "", fmt.Errorf("executable file not found in $PATH: %s", name)
}
