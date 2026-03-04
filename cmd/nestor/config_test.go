package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTempConfig writes content to a file at path, creating parent dirs as needed.
func writeTempConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

// TestLoadConfig_NoFiles returns an empty Config when no config files exist.
func TestLoadConfig_NoFiles(t *testing.T) {
	// Override config search paths to point at a non-existent directory.
	dir := t.TempDir()

	origPaths := configSearchPaths
	configSearchPaths = func() []string {
		return []string{
			filepath.Join(dir, "does-not-exist.yaml"),
		}
	}
	t.Cleanup(func() { configSearchPaths = origPaths })

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SSHKey != "" || cfg.KnownHosts != "" || cfg.Apply.DryRun != nil {
		t.Errorf("expected zero config, got %+v", cfg)
	}
}

// TestLoadConfig_SingleFile parses global and per-command values correctly.
func TestLoadConfig_SingleFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	writeTempConfig(t, cfgFile, `
ssh-key: /tmp/id_ed25519
known-hosts: /tmp/known_hosts
signing-key: /tmp/signing

apply:
  dry-run: true

init:
  agent: /tmp/nestor-agent
  remote-path: /usr/local/bin/nestor-agent

attach:
  state-file: /tmp/agent.state
  follow: true

status:
  state-file: /tmp/agent.state

local:
  dry-run: true
`)

	origPaths := configSearchPaths
	configSearchPaths = func() []string { return []string{cfgFile} }
	t.Cleanup(func() { configSearchPaths = origPaths })

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.SSHKey != "/tmp/id_ed25519" {
		t.Errorf("SSHKey = %q, want /tmp/id_ed25519", cfg.SSHKey)
	}
	if cfg.KnownHosts != "/tmp/known_hosts" {
		t.Errorf("KnownHosts = %q, want /tmp/known_hosts", cfg.KnownHosts)
	}
	if cfg.SigningKey != "/tmp/signing" {
		t.Errorf("SigningKey = %q, want /tmp/signing", cfg.SigningKey)
	}
	if cfg.Apply.DryRun == nil || !*cfg.Apply.DryRun {
		t.Errorf("Apply.DryRun = %v, want true", cfg.Apply.DryRun)
	}
	if cfg.Init.Agent != "/tmp/nestor-agent" {
		t.Errorf("Init.Agent = %q, want /tmp/nestor-agent", cfg.Init.Agent)
	}
	if cfg.Init.RemotePath != "/usr/local/bin/nestor-agent" {
		t.Errorf("Init.RemotePath = %q, want /usr/local/bin/nestor-agent", cfg.Init.RemotePath)
	}
	if cfg.Attach.StateFile != "/tmp/agent.state" {
		t.Errorf("Attach.StateFile = %q, want /tmp/agent.state", cfg.Attach.StateFile)
	}
	if cfg.Attach.Follow == nil || !*cfg.Attach.Follow {
		t.Errorf("Attach.Follow = %v, want true", cfg.Attach.Follow)
	}
	if cfg.Local.DryRun == nil || !*cfg.Local.DryRun {
		t.Errorf("Local.DryRun = %v, want true", cfg.Local.DryRun)
	}
}

// TestMergeConfig_PwdOverridesXDG verifies that higher-priority files win for string fields.
func TestMergeConfig_PwdOverridesXDG(t *testing.T) {
	dir := t.TempDir()
	xdgFile := filepath.Join(dir, "xdg.yaml")
	pwdFile := filepath.Join(dir, "nestor.yaml")

	writeTempConfig(t, xdgFile, `
ssh-key: /xdg/id_ed25519
known-hosts: /xdg/known_hosts
`)
	writeTempConfig(t, pwdFile, `
ssh-key: /pwd/id_ed25519
`)

	origPaths := configSearchPaths
	// xdgFile has lower precedence (listed first), pwdFile has higher.
	configSearchPaths = func() []string { return []string{xdgFile, pwdFile} }
	t.Cleanup(func() { configSearchPaths = origPaths })

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// pwd overrides xdg for ssh-key
	if cfg.SSHKey != "/pwd/id_ed25519" {
		t.Errorf("SSHKey = %q, want /pwd/id_ed25519", cfg.SSHKey)
	}
	// xdg value preserved when pwd doesn't set it
	if cfg.KnownHosts != "/xdg/known_hosts" {
		t.Errorf("KnownHosts = %q, want /xdg/known_hosts", cfg.KnownHosts)
	}
}

// TestMergeConfig_BooleanNilVsExplicitFalse verifies that *bool nil and false are handled distinctly.
func TestMergeConfig_BooleanNilVsExplicitFalse(t *testing.T) {
	dir := t.TempDir()
	baseFile := filepath.Join(dir, "base.yaml")
	overrideFile := filepath.Join(dir, "override.yaml")

	writeTempConfig(t, baseFile, `
apply:
  dry-run: true
`)
	writeTempConfig(t, overrideFile, `
apply:
  dry-run: false
`)

	origPaths := configSearchPaths
	configSearchPaths = func() []string { return []string{baseFile, overrideFile} }
	t.Cleanup(func() { configSearchPaths = origPaths })

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Higher-priority file explicitly sets false, which should override true.
	if cfg.Apply.DryRun == nil || *cfg.Apply.DryRun != false {
		t.Errorf("Apply.DryRun = %v, want *false", cfg.Apply.DryRun)
	}
}

// TestApplyEnvVars_String verifies NESTOR_SSH_KEY sets Config.SSHKey.
func TestApplyEnvVars_String(t *testing.T) {
	t.Setenv("NESTOR_SSH_KEY", "/env/id_ed25519")
	t.Setenv("NESTOR_KNOWN_HOSTS", "/env/known_hosts")
	t.Setenv("NESTOR_INIT_AGENT", "/env/agent")

	origPaths := configSearchPaths
	configSearchPaths = func() []string { return nil }
	t.Cleanup(func() { configSearchPaths = origPaths })

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SSHKey != "/env/id_ed25519" {
		t.Errorf("SSHKey = %q, want /env/id_ed25519", cfg.SSHKey)
	}
	if cfg.KnownHosts != "/env/known_hosts" {
		t.Errorf("KnownHosts = %q, want /env/known_hosts", cfg.KnownHosts)
	}
	if cfg.Init.Agent != "/env/agent" {
		t.Errorf("Init.Agent = %q, want /env/agent", cfg.Init.Agent)
	}
}

// TestApplyEnvVars_Bool verifies NESTOR_APPLY_DRY_RUN sets the *bool field.
func TestApplyEnvVars_Bool(t *testing.T) {
	for _, tc := range []struct {
		val  string
		want bool
	}{
		{"true", true},
		{"1", true},
		{"false", false},
		{"0", false},
	} {
		t.Run(tc.val, func(t *testing.T) {
			t.Setenv("NESTOR_APPLY_DRY_RUN", tc.val)

			origPaths := configSearchPaths
			configSearchPaths = func() []string { return nil }
			t.Cleanup(func() { configSearchPaths = origPaths })

			cfg, err := loadConfig()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Apply.DryRun == nil {
				t.Fatalf("Apply.DryRun is nil, want *%v", tc.want)
			}
			if *cfg.Apply.DryRun != tc.want {
				t.Errorf("Apply.DryRun = %v, want %v", *cfg.Apply.DryRun, tc.want)
			}
		})
	}
}

// TestResolveSSHKey_CommandOverridesGlobal verifies command-specific key beats global.
func TestResolveSSHKey_CommandOverridesGlobal(t *testing.T) {
	cfg := &Config{
		SSHKey: "/global/id_ed25519",
		Apply:  ApplyConfig{SSHKey: "/apply/id_ed25519"},
	}
	got := cfg.resolveSSHKey("apply")
	if got != "/apply/id_ed25519" {
		t.Errorf("resolveSSHKey(apply) = %q, want /apply/id_ed25519", got)
	}
}

// TestResolveSSHKey_GlobalOverridesHardcoded verifies global key beats hardcoded default.
func TestResolveSSHKey_GlobalOverridesHardcoded(t *testing.T) {
	cfg := &Config{SSHKey: "/global/id_ed25519"}
	got := cfg.resolveSSHKey("apply")
	if got != "/global/id_ed25519" {
		t.Errorf("resolveSSHKey(apply) = %q, want /global/id_ed25519", got)
	}
}

// TestResolveSSHKey_HardcodedFallback verifies the hardcoded default is used when nothing is set.
func TestResolveSSHKey_HardcodedFallback(t *testing.T) {
	cfg := &Config{}
	got := cfg.resolveSSHKey("apply")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".ssh", "id_ed25519")
	if got != want {
		t.Errorf("resolveSSHKey(apply) = %q, want %q", got, want)
	}
}

// TestResolveStateFile verifies per-command state file with hardcoded fallback.
func TestResolveStateFile(t *testing.T) {
	cfg := &Config{
		Attach: AttachConfig{StateFile: "/custom/agent.state"},
	}
	if got := cfg.resolveStateFile("attach"); got != "/custom/agent.state" {
		t.Errorf("resolveStateFile(attach) = %q, want /custom/agent.state", got)
	}
	// fallback for status when not set
	if got := cfg.resolveStateFile("status"); got != "/tmp/nestor-agent.state" {
		t.Errorf("resolveStateFile(status) = %q, want /tmp/nestor-agent.state", got)
	}
}

// TestResolveBool verifies nil returns false and non-nil returns the pointed-to value.
func TestResolveBool(t *testing.T) {
	if resolveBool(nil) != false {
		t.Error("resolveBool(nil) should be false")
	}
	if resolveBool(boolPtr(true)) != true {
		t.Error("resolveBool(true) should be true")
	}
	if resolveBool(boolPtr(false)) != false {
		t.Error("resolveBool(false) should be false")
	}
}

// TestExpandHome verifies ~ is replaced with the home directory.
func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()

	got := expandHome("~/.ssh/id_ed25519")
	want := filepath.Join(home, ".ssh", "id_ed25519")
	if got != want {
		t.Errorf("expandHome = %q, want %q", got, want)
	}

	// Non-~ paths unchanged
	if got := expandHome("/absolute/path"); got != "/absolute/path" {
		t.Errorf("expandHome(/absolute/path) = %q, want unchanged", got)
	}
}
