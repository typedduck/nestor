package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds user-configurable defaults for all nestor commands.
// Fields at the top level are global defaults shared across commands.
// Per-command sub-structs allow overriding specific flags per command.
//
// Precedence (lowest to highest):
//
//	macOS Library config < XDG config < pwd config < env vars < CLI flags
type Config struct {
	SSHKey     string `yaml:"ssh-key"`
	SigningKey  string `yaml:"signing-key"`
	KnownHosts string `yaml:"known-hosts"`

	Apply  ApplyConfig  `yaml:"apply"`
	Init   InitConfig   `yaml:"init"`
	Attach AttachConfig `yaml:"attach"`
	Status StatusConfig `yaml:"status"`
	Local  LocalConfig  `yaml:"local"`
}

// ApplyConfig holds per-command defaults for the apply command.
type ApplyConfig struct {
	SSHKey     string `yaml:"ssh-key"`
	SigningKey  string `yaml:"signing-key"`
	KnownHosts string `yaml:"known-hosts"`
	DryRun     *bool  `yaml:"dry-run"`
}

// InitConfig holds per-command defaults for the init command.
type InitConfig struct {
	SSHKey     string `yaml:"ssh-key"`
	SigningKey  string `yaml:"signing-key"`
	Agent      string `yaml:"agent"`
	RemotePath string `yaml:"remote-path"`
}

// AttachConfig holds per-command defaults for the attach command.
type AttachConfig struct {
	SSHKey    string `yaml:"ssh-key"`
	StateFile string `yaml:"state-file"`
	Follow    *bool  `yaml:"follow"`
}

// StatusConfig holds per-command defaults for the status command.
type StatusConfig struct {
	SSHKey    string `yaml:"ssh-key"`
	StateFile string `yaml:"state-file"`
}

// LocalConfig holds per-command defaults for the local command.
type LocalConfig struct {
	DryRun *bool `yaml:"dry-run"`
}

// configSearchPaths returns config file paths in order from lowest to highest
// precedence. Missing files are silently skipped by loadConfig.
// It is a variable so tests can substitute it.
var configSearchPaths = func() []string {
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()
	return []string{
		filepath.Join(home, "Library", "Application Support", "nestor", "config.yaml"),
		filepath.Join(home, ".config", "nestor", "config.yaml"),
		filepath.Join(cwd, ".nestor.yaml"),
		filepath.Join(cwd, "nestor.yaml"),
	}
}

// loadConfig reads and merges all config files found in configSearchPaths,
// then applies NESTOR_* environment variable overrides.
func loadConfig() (*Config, error) {
	result := &Config{}
	for _, path := range configSearchPaths() {
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading config %s: %w", path, err)
		}
		var c Config
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", path, err)
		}
		mergeConfig(result, &c)
	}
	applyEnvVars(result)
	return result, nil
}

// mergeConfig copies non-zero fields from override into base.
// For *bool fields, a non-nil pointer in override overwrites base.
func mergeConfig(base, override *Config) {
	if override.SSHKey != "" {
		base.SSHKey = override.SSHKey
	}
	if override.SigningKey != "" {
		base.SigningKey = override.SigningKey
	}
	if override.KnownHosts != "" {
		base.KnownHosts = override.KnownHosts
	}
	// Apply
	if override.Apply.SSHKey != "" {
		base.Apply.SSHKey = override.Apply.SSHKey
	}
	if override.Apply.SigningKey != "" {
		base.Apply.SigningKey = override.Apply.SigningKey
	}
	if override.Apply.KnownHosts != "" {
		base.Apply.KnownHosts = override.Apply.KnownHosts
	}
	if override.Apply.DryRun != nil {
		base.Apply.DryRun = override.Apply.DryRun
	}
	// Init
	if override.Init.SSHKey != "" {
		base.Init.SSHKey = override.Init.SSHKey
	}
	if override.Init.SigningKey != "" {
		base.Init.SigningKey = override.Init.SigningKey
	}
	if override.Init.Agent != "" {
		base.Init.Agent = override.Init.Agent
	}
	if override.Init.RemotePath != "" {
		base.Init.RemotePath = override.Init.RemotePath
	}
	// Attach
	if override.Attach.SSHKey != "" {
		base.Attach.SSHKey = override.Attach.SSHKey
	}
	if override.Attach.StateFile != "" {
		base.Attach.StateFile = override.Attach.StateFile
	}
	if override.Attach.Follow != nil {
		base.Attach.Follow = override.Attach.Follow
	}
	// Status
	if override.Status.SSHKey != "" {
		base.Status.SSHKey = override.Status.SSHKey
	}
	if override.Status.StateFile != "" {
		base.Status.StateFile = override.Status.StateFile
	}
	// Local
	if override.Local.DryRun != nil {
		base.Local.DryRun = override.Local.DryRun
	}
}

// applyEnvVars applies NESTOR_* environment variables onto cfg.
// Env vars override config-file values but are overridden by CLI flags.
// Boolean env vars accept any value accepted by strconv.ParseBool.
func applyEnvVars(c *Config) {
	setStr := func(dest *string, key string) {
		if v := os.Getenv(key); v != "" {
			*dest = v
		}
	}
	setBool := func(dest **bool, key string) {
		if v := os.Getenv(key); v != "" {
			b, err := strconv.ParseBool(v)
			if err == nil {
				*dest = &b
			}
		}
	}

	// Global
	setStr(&c.SSHKey, "NESTOR_SSH_KEY")
	setStr(&c.SigningKey, "NESTOR_SIGNING_KEY")
	setStr(&c.KnownHosts, "NESTOR_KNOWN_HOSTS")
	// Apply
	setStr(&c.Apply.SSHKey, "NESTOR_APPLY_SSH_KEY")
	setStr(&c.Apply.SigningKey, "NESTOR_APPLY_SIGNING_KEY")
	setStr(&c.Apply.KnownHosts, "NESTOR_APPLY_KNOWN_HOSTS")
	setBool(&c.Apply.DryRun, "NESTOR_APPLY_DRY_RUN")
	// Init
	setStr(&c.Init.SSHKey, "NESTOR_INIT_SSH_KEY")
	setStr(&c.Init.SigningKey, "NESTOR_INIT_SIGNING_KEY")
	setStr(&c.Init.Agent, "NESTOR_INIT_AGENT")
	setStr(&c.Init.RemotePath, "NESTOR_INIT_REMOTE_PATH")
	// Attach
	setStr(&c.Attach.SSHKey, "NESTOR_ATTACH_SSH_KEY")
	setStr(&c.Attach.StateFile, "NESTOR_ATTACH_STATE_FILE")
	setBool(&c.Attach.Follow, "NESTOR_ATTACH_FOLLOW")
	// Status
	setStr(&c.Status.SSHKey, "NESTOR_STATUS_SSH_KEY")
	setStr(&c.Status.StateFile, "NESTOR_STATUS_STATE_FILE")
	// Local
	setBool(&c.Local.DryRun, "NESTOR_LOCAL_DRY_RUN")
}

// resolveSSHKey returns the effective ssh-key default for the named command.
// Resolution order: command-specific > global > hardcoded default.
func (c *Config) resolveSSHKey(cmd string) string {
	var cmdKey string
	switch cmd {
	case "apply":
		cmdKey = c.Apply.SSHKey
	case "init":
		cmdKey = c.Init.SSHKey
	case "attach":
		cmdKey = c.Attach.SSHKey
	case "status":
		cmdKey = c.Status.SSHKey
	}
	if cmdKey != "" {
		return expandHome(cmdKey)
	}
	if c.SSHKey != "" {
		return expandHome(c.SSHKey)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "id_ed25519")
}

// resolveSigningKey returns the effective signing-key default for the named command.
// Falls back to the global signing-key, then empty string (callers default to SSH key).
func (c *Config) resolveSigningKey(cmd string) string {
	var cmdKey string
	switch cmd {
	case "apply":
		cmdKey = c.Apply.SigningKey
	case "init":
		cmdKey = c.Init.SigningKey
	}
	if cmdKey != "" {
		return expandHome(cmdKey)
	}
	return expandHome(c.SigningKey)
}

// resolveKnownHosts returns the effective known-hosts default for the named command.
func (c *Config) resolveKnownHosts(cmd string) string {
	var cmdKey string
	if cmd == "apply" {
		cmdKey = c.Apply.KnownHosts
	}
	if cmdKey != "" {
		return expandHome(cmdKey)
	}
	if c.KnownHosts != "" {
		return expandHome(c.KnownHosts)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ssh", "known_hosts")
}

// resolveStateFile returns the effective state-file default for the named command.
func (c *Config) resolveStateFile(cmd string) string {
	var cmdKey string
	switch cmd {
	case "attach":
		cmdKey = c.Attach.StateFile
	case "status":
		cmdKey = c.Status.StateFile
	}
	if cmdKey != "" {
		return cmdKey
	}
	return "/tmp/nestor-agent.state"
}

// resolveBool returns the value of a *bool config field, defaulting to false.
func resolveBool(v *bool) bool {
	if v != nil {
		return *v
	}
	return false
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
