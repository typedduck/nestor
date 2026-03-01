package executor_test

import (
	"errors"
	"os"
	"testing"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/agent/executor/executortest"
)

// ============================================================
// MockFileSystem — ReadFile
// ============================================================

func TestMockFS_ReadFile_ReturnsContent(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/etc/test.conf", []byte("hello"), 0644)

	data, err := fs.ReadFile("/etc/test.conf")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestMockFS_ReadFile_NotFound(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	_, err := fs.ReadFile("/nonexistent")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestMockFS_ReadFile_Directory(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	_, err := fs.ReadFile("/etc")
	if err == nil {
		t.Fatal("expected error when reading a directory")
	}
}

func TestMockFS_ReadFile_ReturnsACopy(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/test", []byte("original"), 0644)

	data, _ := fs.ReadFile("/test")
	data[0] = 'X' // mutate the returned slice

	data2, _ := fs.ReadFile("/test")
	if string(data2) != "original" {
		t.Error("ReadFile should return a copy; mutating the result changed stored data")
	}
}

// ============================================================
// MockFileSystem — WriteFile
// ============================================================

func TestMockFS_WriteFile_CreatesFile(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	if err := fs.WriteFile("/etc/new.conf", []byte("content"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	data, ok := fs.FileContent("/etc/new.conf")
	if !ok {
		t.Fatal("file not found after WriteFile")
	}
	if string(data) != "content" {
		t.Errorf("expected 'content', got %q", string(data))
	}
}

func TestMockFS_WriteFile_UpdatesExistingFile(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)
	fs.AddFile("/etc/test.conf", []byte("old"), 0644)

	fs.WriteFile("/etc/test.conf", []byte("new"), 0600)

	data, _ := fs.FileContent("/etc/test.conf")
	if string(data) != "new" {
		t.Errorf("expected 'new', got %q", string(data))
	}
}

func TestMockFS_WriteFile_UpdatesMode(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	fs.WriteFile("/etc/secret.conf", []byte("data"), 0600)

	mode, _ := fs.FileMode("/etc/secret.conf")
	if mode != 0600 {
		t.Errorf("expected mode 0600, got %04o", mode)
	}
}

func TestMockFS_WriteFile_ParentMustExist(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	err := fs.WriteFile("/nonexistent/dir/file.conf", []byte("data"), 0644)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist for missing parent, got %v", err)
	}
}

func TestMockFS_WriteFile_ParentMustBeDirectory(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/etc", []byte("i am a file"), 0644) // /etc is a file, not a dir

	err := fs.WriteFile("/etc/test.conf", []byte("data"), 0644)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist when parent is a file, got %v", err)
	}
}

func TestMockFS_WriteFile_StoresACopy(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/etc", 0755)

	input := []byte("original")
	fs.WriteFile("/etc/test.conf", input, 0644)
	input[0] = 'X' // mutate after write

	data, _ := fs.FileContent("/etc/test.conf")
	if string(data) != "original" {
		t.Error("WriteFile should copy input; mutating the slice after write changed stored data")
	}
}

func TestMockFS_WriteFile_RootLevelNeedsNoParentEntry(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	if err := fs.WriteFile("/topfile", []byte("data"), 0644); err != nil {
		t.Fatalf("WriteFile at root level: %v", err)
	}
	if !fs.Exists("/topfile") {
		t.Error("file should exist after WriteFile")
	}
}

// ============================================================
// MockFileSystem — Stat / MockFileInfo
// ============================================================

func TestMockFS_Stat_FileFields(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/etc/nginx.conf", []byte("hello"), 0644)

	info, err := fs.Stat("/etc/nginx.conf")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if info.Name() != "nginx.conf" {
		t.Errorf("Name: expected 'nginx.conf', got %q", info.Name())
	}
	if info.Size() != 5 {
		t.Errorf("Size: expected 5, got %d", info.Size())
	}
	if info.Mode() != 0644 {
		t.Errorf("Mode: expected 0644, got %04o", info.Mode())
	}
	if info.IsDir() {
		t.Error("IsDir: expected false for a file")
	}
	if info.Sys() != nil {
		t.Error("Sys: expected nil")
	}
	if !info.ModTime().IsZero() {
		t.Errorf("ModTime: expected zero time, got %v", info.ModTime())
	}
}

func TestMockFS_Stat_DirectoryIsDir(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt/app", 0755)

	info, err := fs.Stat("/opt/app")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("IsDir: expected true for a directory")
	}
	if info.Mode()&os.ModeDir == 0 {
		t.Errorf("Mode: expected ModeDir bit set, got %v", info.Mode())
	}
}

func TestMockFS_Stat_DirectoryName(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt/app", 0755)

	info, _ := fs.Stat("/opt/app")
	if info.Name() != "app" {
		t.Errorf("Name: expected 'app', got %q", info.Name())
	}
}

func TestMockFS_Stat_NotFound(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	_, err := fs.Stat("/nonexistent")
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

// ============================================================
// MockFileSystem — Mkdir
// ============================================================

func TestMockFS_Mkdir_CreatesDirectory(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	if err := fs.Mkdir("/opt/app", 0750); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}

	info, err := fs.Stat("/opt/app")
	if err != nil {
		t.Fatalf("Stat after Mkdir: %v", err)
	}
	if !info.IsDir() {
		t.Error("created entry should be a directory")
	}
}

func TestMockFS_Mkdir_SetsModeDirBit(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)
	fs.Mkdir("/opt/app", 0750)

	mode, _ := fs.FileMode("/opt/app")
	if mode&os.ModeDir == 0 {
		t.Errorf("ModeDir bit missing after Mkdir; mode=%v", mode)
	}
}

func TestMockFS_Mkdir_AlreadyExists(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	err := fs.Mkdir("/opt", 0755)
	if !errors.Is(err, os.ErrExist) {
		t.Errorf("expected os.ErrExist, got %v", err)
	}
}

func TestMockFS_Mkdir_ParentMustExist(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	err := fs.Mkdir("/nonexistent/child", 0755)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist for missing parent, got %v", err)
	}
}

func TestMockFS_Mkdir_RootLevelNeedsNoParentEntry(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	if err := fs.Mkdir("/topleveldir", 0755); err != nil {
		t.Fatalf("Mkdir at root level: %v", err)
	}
	if !fs.Exists("/topleveldir") {
		t.Error("directory should exist")
	}
}

// ============================================================
// MockFileSystem — MkdirAll
// ============================================================

func TestMockFS_MkdirAll_CreatesAllComponents(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	if err := fs.MkdirAll("/opt/app/data/logs", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	for _, path := range []string{"/opt", "/opt/app", "/opt/app/data", "/opt/app/data/logs"} {
		if !fs.Exists(path) {
			t.Errorf("expected %s to exist after MkdirAll", path)
		}
	}
}

func TestMockFS_MkdirAll_Idempotent(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	if err := fs.MkdirAll("/opt/app", 0755); err != nil {
		t.Fatalf("first MkdirAll: %v", err)
	}
	if err := fs.MkdirAll("/opt/app", 0755); err != nil {
		t.Fatalf("second MkdirAll (idempotent): %v", err)
	}
}

func TestMockFS_MkdirAll_FailsWhenComponentIsFile(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/opt", []byte("i am a file"), 0644)

	err := fs.MkdirAll("/opt/app", 0755)
	if err == nil {
		t.Fatal("expected error when a path component is a file")
	}
}

// ============================================================
// MockFileSystem — Chmod
// ============================================================

func TestMockFS_Chmod_UpdatesFileMode(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/test.conf", []byte("data"), 0644)

	if err := fs.Chmod("/test.conf", 0600); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	mode, _ := fs.FileMode("/test.conf")
	if mode != 0600 {
		t.Errorf("expected mode 0600 after Chmod, got %04o", mode)
	}
}

func TestMockFS_Chmod_PreservesModeDirBit(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	fs.Chmod("/opt", 0700)

	mode, _ := fs.FileMode("/opt")
	if mode&os.ModeDir == 0 {
		t.Error("Chmod should preserve ModeDir bit on a directory")
	}
	if mode&0777 != 0700 {
		t.Errorf("expected permission bits 0700, got %04o", mode&0777)
	}
}

func TestMockFS_Chmod_NotFound(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	err := fs.Chmod("/nonexistent", 0644)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

// ============================================================
// MockFileSystem — Chown
// ============================================================

func TestMockFS_Chown_UpdatesOwnership(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/test.conf", []byte("data"), 0644)

	if err := fs.Chown("/test.conf", 1000, 2000); err != nil {
		t.Fatalf("Chown: %v", err)
	}

	uid, gid, ok := fs.FileOwner("/test.conf")
	if !ok {
		t.Fatal("FileOwner: path not found")
	}
	if uid != 1000 || gid != 2000 {
		t.Errorf("expected uid=1000 gid=2000, got uid=%d gid=%d", uid, gid)
	}
}

func TestMockFS_Chown_DefaultOwnershipIsZero(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/test.conf", []byte("data"), 0644)

	uid, gid, ok := fs.FileOwner("/test.conf")
	if !ok {
		t.Fatal("FileOwner: not found")
	}
	if uid != 0 || gid != 0 {
		t.Errorf("expected default uid=0 gid=0, got uid=%d gid=%d", uid, gid)
	}
}

func TestMockFS_Chown_NotFound(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	err := fs.Chown("/nonexistent", 0, 0)
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

// ============================================================
// MockFileSystem — Open
// ============================================================

func TestMockFS_Open_AlwaysReturnsError(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/test", []byte("data"), 0644)

	_, err := fs.Open("/test")
	if err == nil {
		t.Fatal("Open should always return an error (not supported by mock)")
	}
}

// ============================================================
// MockFileSystem — path cleaning
// ============================================================

func TestMockFS_PathCleaning_DotDot(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/etc/nginx/nginx.conf", []byte("data"), 0644)

	if !fs.Exists("/etc/nginx/../nginx/nginx.conf") {
		t.Error("path with '..' should resolve to the same entry")
	}
}

func TestMockFS_PathCleaning_TrailingSlash(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt/app", 0755)

	if !fs.Exists("/opt/app/") {
		t.Error("trailing slash should clean to the same entry")
	}
}

// ============================================================
// MockFileSystem — helpers: FileContent, FileMode, FileOwner, Exists
// ============================================================

func TestMockFS_FileContent_NotFoundReturnsFalse(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	_, ok := fs.FileContent("/nonexistent")
	if ok {
		t.Error("FileContent should return false for a missing path")
	}
}

func TestMockFS_FileContent_DirectoryReturnsFalse(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	_, ok := fs.FileContent("/opt")
	if ok {
		t.Error("FileContent should return false for a directory")
	}
}

func TestMockFS_FileMode_NotFoundReturnsFalse(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	_, ok := fs.FileMode("/nonexistent")
	if ok {
		t.Error("FileMode should return false for a missing path")
	}
}

func TestMockFS_FileOwner_NotFoundReturnsFalse(t *testing.T) {
	fs := executortest.NewMockFileSystem()

	_, _, ok := fs.FileOwner("/nonexistent")
	if ok {
		t.Error("FileOwner should return false for a missing path")
	}
}

func TestMockFS_Exists_File(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddFile("/etc/test.conf", []byte("x"), 0644)

	if !fs.Exists("/etc/test.conf") {
		t.Error("Exists should return true for a known file")
	}
	if fs.Exists("/etc/other.conf") {
		t.Error("Exists should return false for an unknown file")
	}
}

func TestMockFS_Exists_Directory(t *testing.T) {
	fs := executortest.NewMockFileSystem()
	fs.AddDir("/opt", 0755)

	if !fs.Exists("/opt") {
		t.Error("Exists should return true for a known directory")
	}
}

// ============================================================
// MockCommandRunner — Run
// ============================================================

func TestMockCmd_Run_Unconfigured(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()

	err := cmd.Run("unknown-cmd", nil)
	if err == nil {
		t.Fatal("expected error for unconfigured command")
	}
}

func TestMockCmd_Run_Success(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{ExitCode: 0})

	if err := cmd.Run("apt-get", nil, "install", "-y", "nginx"); err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestMockCmd_Run_ConfiguredError(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	sentinel := errors.New("disk full")
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{Err: sentinel})

	err := cmd.Run("apt-get", nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
}

func TestMockCmd_Run_NonZeroExitCode(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{ExitCode: 1})

	if err := cmd.Run("apt-get", nil); err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}

// ============================================================
// MockCommandRunner — CombinedOutput
// ============================================================

func TestMockCmd_CombinedOutput_Unconfigured(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()

	_, exitCode, err := cmd.CombinedOutput("unknown-cmd", nil)
	if err == nil {
		t.Fatal("expected error for unconfigured command")
	}
	if exitCode != -1 {
		t.Errorf("expected exit code -1 for unconfigured command, got %d", exitCode)
	}
}

func TestMockCmd_CombinedOutput_Success(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{
		Output: []byte("install ok installed"), ExitCode: 0,
	})

	output, exitCode, err := cmd.CombinedOutput("dpkg-query", nil, "-W", "nginx")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if string(output) != "install ok installed" {
		t.Errorf("unexpected output: %q", string(output))
	}
}

func TestMockCmd_CombinedOutput_NonZeroExit(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("cmd", executortest.MockCommandResponse{
		Output: []byte("error text"), ExitCode: 2,
	})

	output, exitCode, err := cmd.CombinedOutput("cmd", nil)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
	if exitCode != 2 {
		t.Errorf("expected exit code 2, got %d", exitCode)
	}
	if string(output) != "error text" {
		t.Errorf("expected 'error text', got %q", string(output))
	}
}

func TestMockCmd_CombinedOutput_ConfiguredError(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	sentinel := errors.New("connection refused")
	cmd.SetResponse("cmd", executortest.MockCommandResponse{
		Output: []byte("partial"), ExitCode: 1, Err: sentinel,
	})

	output, exitCode, err := cmd.CombinedOutput("cmd", nil)
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got: %v", err)
	}
	if string(output) != "partial" {
		t.Errorf("expected 'partial' output, got %q", string(output))
	}
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}
}

// ============================================================
// MockCommandRunner — LookPath
// ============================================================

func TestMockCmd_LookPath_Found(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetLookPath("apt-get", "/usr/bin/apt-get")

	path, err := cmd.LookPath("apt-get")
	if err != nil {
		t.Fatalf("LookPath: %v", err)
	}
	if path != "/usr/bin/apt-get" {
		t.Errorf("expected /usr/bin/apt-get, got %q", path)
	}
}

func TestMockCmd_LookPath_NotFound(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()

	_, err := cmd.LookPath("nonexistent-tool")
	if err == nil {
		t.Fatal("expected error for unconfigured LookPath")
	}
}

func TestMockCmd_LookPath_NotRecordedInCalls(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetLookPath("apt-get", "/usr/bin/apt-get")

	cmd.LookPath("apt-get")

	if len(cmd.Calls()) != 0 {
		t.Errorf("LookPath should not be recorded in Calls(); got %d call(s)", len(cmd.Calls()))
	}
}

// ============================================================
// MockCommandRunner — Calls recording
// ============================================================

func TestMockCmd_Calls_RecordsRunAndCombinedOutput(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{})
	cmd.SetResponse("dpkg-query", executortest.MockCommandResponse{})

	cmd.Run("apt-get", nil, "update")
	cmd.CombinedOutput("dpkg-query", nil, "-W", "nginx")

	calls := cmd.Calls()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Name != "apt-get" {
		t.Errorf("call[0]: expected apt-get, got %q", calls[0].Name)
	}
	if calls[1].Name != "dpkg-query" {
		t.Errorf("call[1]: expected dpkg-query, got %q", calls[1].Name)
	}
}

func TestMockCmd_Calls_RecordsArgs(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{})

	cmd.Run("apt-get", nil, "install", "-y", "nginx")

	calls := cmd.Calls()
	if len(calls[0].Args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(calls[0].Args), calls[0].Args)
	}
	if calls[0].Args[0] != "install" || calls[0].Args[1] != "-y" || calls[0].Args[2] != "nginx" {
		t.Errorf("unexpected args: %v", calls[0].Args)
	}
}

func TestMockCmd_Calls_RecordsOpts(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{})

	opts := &executor.CommandOpts{Env: []string{"DEBIAN_FRONTEND=noninteractive"}, Dir: "/tmp"}
	cmd.Run("apt-get", opts, "install", "nginx")

	calls := cmd.Calls()
	if calls[0].Opts == nil {
		t.Fatal("expected Opts to be recorded")
	}
	if len(calls[0].Opts.Env) != 1 || calls[0].Opts.Env[0] != "DEBIAN_FRONTEND=noninteractive" {
		t.Errorf("unexpected Opts.Env: %v", calls[0].Opts.Env)
	}
	if calls[0].Opts.Dir != "/tmp" {
		t.Errorf("expected Opts.Dir=/tmp, got %q", calls[0].Opts.Dir)
	}
}

func TestMockCmd_Calls_NilOptsRecordedAsNil(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("cmd", executortest.MockCommandResponse{})

	cmd.Run("cmd", nil)

	calls := cmd.Calls()
	if calls[0].Opts != nil {
		t.Errorf("expected nil Opts, got %v", calls[0].Opts)
	}
}

func TestMockCmd_Calls_ReturnsACopy(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()
	cmd.SetResponse("apt-get", executortest.MockCommandResponse{})
	cmd.Run("apt-get", nil)

	calls := cmd.Calls()
	calls[0].Name = "mutated"

	calls2 := cmd.Calls()
	if calls2[0].Name == "mutated" {
		t.Error("Calls() should return a copy; mutating it changed internal state")
	}
}

func TestMockCmd_Calls_EmptyInitially(t *testing.T) {
	cmd := executortest.NewMockCommandRunner()

	if len(cmd.Calls()) != 0 {
		t.Errorf("expected 0 calls initially, got %d", len(cmd.Calls()))
	}
}
