package system

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

// CommandLooker abstracts command lookup for testability.
// This is a subset of executor.CommandRunner to avoid circular imports.
type CommandLooker interface {
	LookPath(name string) (string, error)
}

// Info contains information about the system
type Info struct {
	OS             string
	Distribution   string
	PackageManager string
	InitSystem     string
	Architecture   string
}

// DetectSystem detects the system capabilities using the provided abstractions.
// If fr or cl are nil, real OS/exec implementations are used.
func DetectSystem(fr FileReader, cl CommandLooker) *Info {
	if fr == nil {
		fr = osFileReader{}
	}
	if cl == nil {
		cl = osCommandLooker{}
	}

	info := &Info{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	if info.OS == "linux" {
		info.Distribution = detectDistribution(fr)
		info.PackageManager = detectPackageManager(cl)
		info.InitSystem = detectInitSystem(fr, cl)
	}

	return info
}

// IsRoot checks if the current process is running as root
func IsRoot() bool {
	return os.Geteuid() == 0
}

// detectDistribution detects the Linux distribution
func detectDistribution(fr FileReader) string {
	// Try reading /etc/os-release
	data, err := fr.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ID=") {
				return strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
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

// detectPackageManager detects the available package manager
func detectPackageManager(cl CommandLooker) string {
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

// detectInitSystem detects the init system
func detectInitSystem(fr FileReader, cl CommandLooker) string {
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

// FileInfo provides information about a file
type FileInfo struct {
	Exists bool
	Mode   os.FileMode
	Owner  int
	Group  int
	Size   int64
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
