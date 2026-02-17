package system

import (
	"os"
	"os/exec"
)

// osFileReader is the default FileReader using the real OS.
type osFileReader struct{}

func (osFileReader) ReadFile(path string) ([]byte, error)      { return os.ReadFile(path) }
func (osFileReader) Stat(path string) (os.FileInfo, error)     { return os.Stat(path) }

// osCommandLooker is the default CommandLooker using os/exec.
type osCommandLooker struct{}

func (osCommandLooker) LookPath(name string) (string, error) { return exec.LookPath(name) }
