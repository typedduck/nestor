package modules

import (
	"fmt"

	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/playbook/builder"
)

// CommandOption is a functional option for configuring command execution.
type CommandOption func(*commandConfig)

type commandConfig struct {
	creates string
	env     []string
	chdir   string
}

// Creates sets the idempotency guard: the command is skipped when the given
// path already exists on the remote system.
func Creates(path string) CommandOption {
	return func(cfg *commandConfig) { cfg.creates = path }
}

// CommandEnv adds extra environment variables (in KEY=VALUE form) for the
// command execution.
func CommandEnv(env ...string) CommandOption {
	return func(cfg *commandConfig) { cfg.env = append(cfg.env, env...) }
}

// Chdir sets the working directory for the command execution.
func Chdir(dir string) CommandOption {
	return func(cfg *commandConfig) { cfg.chdir = dir }
}

// Command adds an action that runs a shell command on the remote system via
// /bin/sh -c. The command string is passed verbatim to the shell, so it may
// contain pipes, redirections, and variable expansions.
//
// The command is always considered to have made a change (Changed: true) when
// it runs. Use Creates to make the action idempotent: the command is skipped
// when the given path already exists.
//
// Examples:
//
//	modules.Command(b, "echo hello")
//	modules.Command(b, "useradd -m deploy", modules.Creates("/home/deploy"))
//	modules.Command(b, "make install", modules.Chdir("/opt/src/app"))
func Command(b *builder.Builder, command string, opts ...CommandOption) error {
	if command == "" {
		return fmt.Errorf("command must not be empty")
	}

	cfg := &commandConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	params := map[string]any{"command": command}
	if cfg.creates != "" {
		params["creates"] = cfg.creates
	}
	if len(cfg.env) > 0 {
		params["env"] = cfg.env
	}
	if cfg.chdir != "" {
		params["chdir"] = cfg.chdir
	}

	b.AddAction(playbook.Action{
		Type:   "command.execute",
		Params: params,
	})
	return nil
}

// ScriptOption is a functional option for configuring script execution.
type ScriptOption func(*scriptConfig)

type scriptConfig struct {
	args    []string
	creates string
	env     []string
	chdir   string
}

// ScriptArgs appends positional arguments passed to the script.
func ScriptArgs(args ...string) ScriptOption {
	return func(cfg *scriptConfig) { cfg.args = append(cfg.args, args...) }
}

// ScriptCreates sets the idempotency guard for script execution.
func ScriptCreates(path string) ScriptOption {
	return func(cfg *scriptConfig) { cfg.creates = path }
}

// ScriptEnv adds extra environment variables (in KEY=VALUE form).
func ScriptEnv(env ...string) ScriptOption {
	return func(cfg *scriptConfig) { cfg.env = append(cfg.env, env...) }
}

// ScriptChdir sets the working directory for script execution.
func ScriptChdir(dir string) ScriptOption {
	return func(cfg *scriptConfig) { cfg.chdir = dir }
}

// Script adds an action that uploads and executes a local script on the remote
// system. The script is run via /bin/sh so it does not need to be executable.
//
// The source path is relative to the local project root; the packager will
// include it in the playbook bundle under upload/.
//
// Examples:
//
//	modules.Script(b, "scripts/setup.sh")
//	modules.Script(b, "scripts/setup.sh",
//	    modules.ScriptArgs("--verbose"),
//	    modules.ScriptCreates("/etc/app/setup.done"))
func Script(b *builder.Builder, source string, opts ...ScriptOption) error {
	if source == "" {
		return fmt.Errorf("script source must not be empty")
	}

	cfg := &scriptConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	params := map[string]any{"source": source}
	if len(cfg.args) > 0 {
		params["args"] = cfg.args
	}
	if cfg.creates != "" {
		params["creates"] = cfg.creates
	}
	if len(cfg.env) > 0 {
		params["env"] = cfg.env
	}
	if cfg.chdir != "" {
		params["chdir"] = cfg.chdir
	}

	b.AddAction(playbook.Action{
		Type:   "script.execute",
		Params: params,
	})
	return nil
}
