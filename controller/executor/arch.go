package executor

import (
	"fmt"
	"strings"

	"github.com/typedduck/nestor/controller/ssh"
)

// unameOS maps lowercase uname -s output to GOOS values.
var unameOS = map[string]string{
	"linux":   "linux",
	"darwin":  "darwin",
	"freebsd": "freebsd",
	"openbsd": "openbsd",
	"netbsd":  "netbsd",
}

// unameMachine maps lowercase uname -m output to GOARCH values.
var unameMachine = map[string]string{
	"x86_64":  "amd64",
	"aarch64": "arm64",
	"arm64":   "arm64",
	"armv7l":  "arm",
	"armv6l":  "arm",
	"i386":    "386",
	"i686":    "386",
}

// mapOS maps a uname -s value to a GOOS name.
// The comparison is case-insensitive.
func mapOS(s string) (string, error) {
	if goos, ok := unameOS[strings.ToLower(s)]; ok {
		return goos, nil
	}
	return "", fmt.Errorf("unsupported remote OS %q", s)
}

// mapArch maps a uname -m value to a GOARCH name.
// The comparison is case-insensitive.
func mapArch(s string) (string, error) {
	if goarch, ok := unameMachine[strings.ToLower(s)]; ok {
		return goarch, nil
	}
	return "", fmt.Errorf("unsupported remote architecture %q", s)
}

// HasArchPlaceholders reports whether path contains {os} or {arch}.
func HasArchPlaceholders(path string) bool {
	return strings.Contains(path, "{os}") || strings.Contains(path, "{arch}")
}

// ResolveAgentPath substitutes {os} and {arch} placeholders in path
// with the provided values. If path contains no placeholders it is returned unchanged.
func ResolveAgentPath(path, goos, goarch string) string {
	path = strings.ReplaceAll(path, "{os}", goos)
	path = strings.ReplaceAll(path, "{arch}", goarch)
	return path
}

// detectRemoteSystem runs "uname -s -m" on the already-connected client and
// returns the remote GOOS and GOARCH using Go's naming conventions.
func detectRemoteSystem(client *ssh.Client) (goos, goarch string, err error) {
	stdout, _, err := client.RunCommandWithOutput("uname -s -m")
	if err != nil {
		return "", "", fmt.Errorf("uname failed: %w", err)
	}

	fields := strings.Fields(strings.TrimSpace(stdout))
	if len(fields) != 2 {
		return "", "", fmt.Errorf("unexpected uname output: %q", stdout)
	}

	goos, err = mapOS(fields[0])
	if err != nil {
		return "", "", err
	}
	goarch, err = mapArch(fields[1])
	if err != nil {
		return "", "", err
	}

	return goos, goarch, nil
}
