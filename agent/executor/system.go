package executor

import (
	"os"
	"runtime"
	"strings"
)

// FileReader abstracts file system reads for testability.
// This is a subset of executor.FileSystem to avoid circular imports.
type FileReader interface {
	ReadFile(path string) ([]byte, error)
	Stat(path string) (os.FileInfo, error)
}

// FileSystem abstracts file system operations for testability
type FileSystem interface {
	FileReader
	WriteFile(path string, data []byte, perm os.FileMode) error
	Mkdir(path string, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Chmod(path string, mode os.FileMode) error
	Chown(path string, uid, gid int) error
	Open(path string) (*os.File, error)
}

// CommandLooker abstracts command lookup for testability.
// This is a subset of executor.CommandRunner to avoid circular imports.
type CommandLooker interface {
	LookPath(name string) (string, error)
}

// CommandRunner abstracts command execution for testability
type CommandRunner interface {
	CommandLooker
	Run(name string, opts *CommandOpts, args ...string) error
	CombinedOutput(name string, opts *CommandOpts, args ...string) ([]byte, int, error)
}

// FileInfo provides information about a file
type FileInfo struct {
	Exists bool
	Mode   os.FileMode
	Owner  int
	Group  int
	Size   int64
}

// Info contains information about the system
type Info struct {
	OS             string
	Distribution   string
	PackageManager string
	InitSystem     string
	Architecture   string
}

// CommandOpts provides optional configuration for command execution
type CommandOpts struct {
	Env []string // additional environment variables
	Dir string   // working directory
}

// DetectSystem detects the system capabilities using the provided abstractions.
// If fr or cl are nil, real OS/exec implementations are used.
func DetectSystem(fr FileReader, cl CommandLooker) *Info {
	if fr == nil {
		fr = OSFileReader{}
	}
	if cl == nil {
		cl = OSCommandLooker{}
	}

	info := &Info{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	if info.OS == "linux" {
		info.Distribution = DetectDistribution(fr)
		info.PackageManager = DetectPackageManager(cl)
		info.InitSystem = DetectInitSystem(fr, cl)
	}

	return info
}

// IsRoot checks if the current process is running as root
func IsRoot() bool {
	return os.Geteuid() == 0
}

// DetectDistribution detects the Linux distribution
func DetectDistribution(fr FileReader) string {
	// Try reading /etc/os-release
	data, err := fr.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.SplitSeq(string(data), "\n")
		for line := range lines {
			if after, ok := strings.CutPrefix(line, "ID="); ok {
				return strings.Trim(after, "\"")
			}
		}
	}

	// Fallback checks
	if fileExistsVia(fr, "/etc/debian_version") {
		return "debian"
	}
	if fileExistsVia(fr, "/etc/redhat-release") {
		return "redhat"
	}

	return "unknown"
}

// DetectPackageManager detects the available package manager
func DetectPackageManager(cl CommandLooker) string {
	managers := []struct {
		command string
		name    string
	}{
		{"apt-get", "apt"},
		{"yum", "yum"},
		{"dnf", "dnf"},
		{"pacman", "pacman"},
		{"zypper", "zypper"},
		{"apk", "apk"},
	}

	for _, mgr := range managers {
		if commandExistsVia(cl, mgr.command) {
			return mgr.name
		}
	}

	return "unknown"
}

// DetectInitSystem detects the init system
func DetectInitSystem(fr FileReader, cl CommandLooker) string {
	// Check for systemd
	if commandExistsVia(cl, "systemctl") {
		return "systemd"
	}

	// Check for SysV init
	if fileExistsVia(fr, "/etc/init.d") {
		return "sysvinit"
	}

	// Check for OpenRC
	if commandExistsVia(cl, "rc-service") {
		return "openrc"
	}

	return "unknown"
}

// commandExistsVia checks if a command is available in PATH
func commandExistsVia(cl CommandLooker, command string) bool {
	_, err := cl.LookPath(command)
	return err == nil
}

// fileExistsVia checks if a file exists using the FileReader
func fileExistsVia(fr FileReader, path string) bool {
	_, err := fr.Stat(path)
	return err == nil
}

// GetFileInfo gets information about a file
func GetFileInfo(fr FileReader, path string) (*FileInfo, error) {
	stat, err := fr.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileInfo{Exists: false}, nil
		}
		return nil, err
	}

	// TODO: Get owner and group from stat.Sys()
	// This requires platform-specific code

	return &FileInfo{
		Exists: true,
		Mode:   stat.Mode(),
		Size:   stat.Size(),
	}, nil
}
