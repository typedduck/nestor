package system

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Info contains information about the system
type Info struct {
	OS             string
	Distribution   string
	PackageManager string
	InitSystem     string
	Architecture   string
}

// DetectSystem detects the system capabilities
func DetectSystem() *Info {
	info := &Info{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	if info.OS == "linux" {
		info.Distribution = detectDistribution()
		info.PackageManager = detectPackageManager()
		info.InitSystem = detectInitSystem()
	}

	return info
}

// IsRoot checks if the current process is running as root
func IsRoot() bool {
	return os.Geteuid() == 0
}

// detectDistribution detects the Linux distribution
func detectDistribution() string {
	// Try reading /etc/os-release
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "ID=") {
				return strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
		}
	}

	// Fallback checks
	if fileExists("/etc/debian_version") {
		return "debian"
	}
	if fileExists("/etc/redhat-release") {
		return "redhat"
	}

	return "unknown"
}

// detectPackageManager detects the available package manager
func detectPackageManager() string {
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
		if commandExists(mgr.command) {
			return mgr.name
		}
	}

	return "unknown"
}

// detectInitSystem detects the init system
func detectInitSystem() string {
	// Check for systemd
	if commandExists("systemctl") {
		return "systemd"
	}

	// Check for SysV init
	if fileExists("/etc/init.d") {
		return "sysvinit"
	}

	// Check for OpenRC
	if commandExists("rc-service") {
		return "openrc"
	}

	return "unknown"
}

// commandExists checks if a command is available in PATH
func commandExists(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
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
func GetFileInfo(path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
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
