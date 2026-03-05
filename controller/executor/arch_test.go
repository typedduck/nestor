package executor

import "testing"

func TestMapGOOS(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"Linux", "linux", false},
		{"linux", "linux", false},
		{"Darwin", "darwin", false},
		{"FreeBSD", "freebsd", false},
		{"OpenBSD", "openbsd", false},
		{"NetBSD", "netbsd", false},
		{"Windows", "", true},
		{"SunOS", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := mapOS(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("mapGOOS(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("mapGOOS(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMapGOARCH(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"x86_64", "amd64", false},
		{"X86_64", "amd64", false},
		{"aarch64", "arm64", false},
		{"arm64", "arm64", false},
		{"armv7l", "arm", false},
		{"armv6l", "arm", false},
		{"i386", "386", false},
		{"i686", "386", false},
		{"mips64", "", true},
		{"", "", true},
	}
	for _, tc := range tests {
		got, err := mapArch(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("mapGOARCH(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("mapGOARCH(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestHasArchPlaceholders(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"./build/nestor-agent-{os}-{arch}", true},
		{"./build/nestor-agent-{os}", true},
		{"./build/nestor-agent-{arch}", true},
		{"./build/nestor-agent", false},
		{"./build/nestor-agent-linux-amd64", false},
		{"/usr/local/bin/nestor-agent", false},
	}
	for _, tc := range tests {
		if got := HasArchPlaceholders(tc.path); got != tc.want {
			t.Errorf("HasArchPlaceholders(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestResolveAgentPath(t *testing.T) {
	tests := []struct {
		path   string
		goos   string
		goarch string
		want   string
	}{
		{
			"./build/nestor-agent-{os}-{arch}",
			"linux", "amd64",
			"./build/nestor-agent-linux-amd64",
		},
		{
			"./build/nestor-agent-{os}-{arch}",
			"darwin", "arm64",
			"./build/nestor-agent-darwin-arm64",
		},
		{
			// Only {arch} placeholder.
			"./build/nestor-agent-{arch}",
			"linux", "amd64",
			"./build/nestor-agent-amd64",
		},
		{
			// No placeholders — path returned unchanged.
			"./build/nestor-agent-linux-amd64",
			"linux", "amd64",
			"./build/nestor-agent-linux-amd64",
		},
		{
			// Absolute path, no placeholders.
			"/tmp/nestor-agent",
			"linux", "amd64",
			"/tmp/nestor-agent",
		},
	}
	for _, tc := range tests {
		got := ResolveAgentPath(tc.path, tc.goos, tc.goarch)
		if got != tc.want {
			t.Errorf("ResolveAgentPath(%q, %q, %q) = %q, want %q",
				tc.path, tc.goos, tc.goarch, got, tc.want)
		}
	}
}
