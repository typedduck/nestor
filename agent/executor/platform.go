package executor

import (
	"os"
	"os/exec"
)

// OSFileReader is the default FileReader using the real OS.
type OSFileReader struct{}

func (OSFileReader) ReadFile(path string) ([]byte, error)  { return os.ReadFile(path) }
func (OSFileReader) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }

// OSCommandLooker is the default CommandLooker using os/exec.
type OSCommandLooker struct{}

func (OSCommandLooker) LookPath(name string) (string, error) { return exec.LookPath(name) }

// OSFileSystem implements FileSystem using the real OS.
type OSFileSystem struct{}

func (OSFileSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (OSFileSystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}
func (OSFileSystem) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (OSFileSystem) Mkdir(path string, perm os.FileMode) error    { return os.Mkdir(path, perm) }
func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (OSFileSystem) Chmod(path string, mode os.FileMode) error    { return os.Chmod(path, mode) }
func (OSFileSystem) Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}
func (OSFileSystem) Open(path string) (*os.File, error) { return os.Open(path) }

// OSCommandRunner implements CommandRunner using os/exec.
type OSCommandRunner struct{}

func (OSCommandRunner) Run(name string, opts *CommandOpts, args ...string) error {
	cmd := exec.Command(name, args...)
	applyOpts(cmd, opts)
	return cmd.Run()
}

func (OSCommandRunner) CombinedOutput(name string, opts *CommandOpts,
	args ...string) ([]byte, int, error) {
	cmd := exec.Command(name, args...)
	applyOpts(cmd, opts)
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return output, -1, err
		}
	}
	return output, exitCode, err
}

func (OSCommandRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func applyOpts(cmd *exec.Cmd, opts *CommandOpts) {
	if opts == nil {
		return
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(cmd.Environ(), opts.Env...)
	}
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
}
