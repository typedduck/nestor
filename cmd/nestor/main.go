package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/typedduck/nestor/controller/executor"
	"github.com/typedduck/nestor/playbook/yamlloader"
)

const (
	Version                = "dev"
	ControllerName         = "nestor"
	DefaultAgentPath       = "./build/nestor-agent-{os}-{arch}"
	DefaultRemoteAgentPath = "/usr/local/bin/nestor-agent"
)

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Execute     func(cfg *Config, args []string) error
}

var commands = []Command{
	{
		Name:        "apply",
		Description: "Apply a YAML playbook to a remote host",
		Execute:     cmdApply,
	},
	{
		Name:        "local",
		Description: "Execute a playbook directory on the local machine",
		Execute:     cmdLocal,
	},
	{
		Name:        "init",
		Description: "Initialize a remote system for nestor",
		Execute:     cmdInit,
	},
	{
		Name:        "attach",
		Description: "Attach to a running or completed agent",
		Execute:     cmdAttach,
	},
	{
		Name:        "status",
		Description: "Check the status of a remote agent",
		Execute:     cmdStatus,
	},
}

// varFlags implements flag.Value to accumulate repeated -var key=value flags.
type varFlags []string

func (v *varFlags) String() string { return strings.Join(*v, ", ") }
func (v *varFlags) Set(s string) error {
	*v = append(*v, s)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	cmdName := os.Args[1]

	// Find and execute command
	for _, cmd := range commands {
		if cmd.Name == cmdName {
			if err := cmd.Execute(cfg, os.Args[2:]); err != nil {
				log.Fatalf("Error: %v", err)
			}
			return
		}
	}

	// Command not found
	fmt.Printf("Unknown command: %s\n\n", cmdName)
	printUsage()
	os.Exit(1)
}

func printUsage() {
	fmt.Printf("%s version %s\n\n", ControllerName, Version)
	fmt.Println("Usage: nestor <command> [options]")
	fmt.Println("\nCommands:")
	for _, cmd := range commands {
		fmt.Printf("  %-10s %s\n", cmd.Name, cmd.Description)
	}
	fmt.Println("\nUse 'nestor <command> -h' for command-specific help")
}

// cmdApply loads a YAML playbook and deploys it to a remote host.
func cmdApply(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)

	var vars varFlags
	fs.Var(&vars, "var", "Set a playbook variable (key=value), may be repeated")
	sshKey := fs.String("ssh-key", cfg.resolveSSHKey("apply"), "SSH private key")
	signingKey := fs.String("signing-key", cfg.resolveSigningKey("apply"), "Signing key (defaults to SSH key)")
	knownHosts := fs.String("known-hosts", cfg.resolveKnownHosts("apply"), "known_hosts file")
	dryRun := fs.Bool("dry-run", resolveBool(cfg.Apply.DryRun), "Package and sign without deploying")

	fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Println("Usage: nestor apply [options] <playbook.yaml> user@host")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		return fmt.Errorf("missing playbook file and/or host argument")
	}

	playbookFile := fs.Arg(0)
	host := fs.Arg(1)

	// Parse -var flags into a map; CLI vars override YAML vars.
	varMap := make(map[string]string, len(vars))
	for _, kv := range vars {
		before, after, ok := strings.Cut(kv, "=")
		if !ok {
			return fmt.Errorf("-var %q must be in key=value form", kv)
		}
		varMap[before] = after
	}

	data, err := os.ReadFile(playbookFile)
	if err != nil {
		return fmt.Errorf("failed to read playbook file: %w", err)
	}

	result, err := yamlloader.Load(data, varMap)
	if err != nil {
		return fmt.Errorf("failed to load playbook: %w", err)
	}

	log.Printf("[INFO ] loaded %q: %d action(s)", result.Remote.Name, len(result.Remote.Actions))

	return executor.Deploy(&executor.Deployment{
		Pre:         result.Pre,
		Remote:      result.Remote,
		Post:        result.Post,
		PlaybookDir: filepath.Dir(playbookFile),
	}, host, &executor.Config{
		SSHKeyPath:     *sshKey,
		SigningKeyPath: *signingKey,
		KnownHostsPath: *knownHosts,
		DryRun:         *dryRun,
	})
}

// cmdLocal loads a YAML playbook from a directory and executes it locally.
// The directory must contain playbook.yaml; uploads/ is the default upload root.
// Run with sudo on Linux or when writing to system paths on macOS.
func cmdLocal(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("local", flag.ExitOnError)

	var vars varFlags
	fs.Var(&vars, "var", "Set a playbook variable (key=value), may be repeated")
	dryRun := fs.Bool("dry-run", resolveBool(cfg.Local.DryRun), "Show what would be done without making changes")

	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: nestor local [options] <dir>")
		fmt.Println()
		fmt.Println("  <dir> must contain playbook.yaml.")
		fmt.Println("  Upload files are resolved relative to <dir>/.")
		fmt.Println()
		fmt.Println("Options:")
		fs.PrintDefaults()
		return fmt.Errorf("missing playbook directory argument")
	}

	dir := fs.Arg(0)

	varMap := make(map[string]string, len(vars))
	for _, kv := range vars {
		before, after, ok := strings.Cut(kv, "=")
		if !ok {
			return fmt.Errorf("-var %q must be in key=value form", kv)
		}
		varMap[before] = after
	}

	data, err := os.ReadFile(filepath.Join(dir, "playbook.yaml"))
	if err != nil {
		return fmt.Errorf("failed to read playbook.yaml: %w", err)
	}

	result, err := yamlloader.Load(data, varMap)
	if err != nil {
		return fmt.Errorf("failed to load playbook: %w", err)
	}

	if result.Pre != nil || result.Post != nil {
		return fmt.Errorf("pre: and post: sections are not supported with 'nestor local'")
	}

	log.Printf("[INFO ] loaded %q: %d action(s)", result.Remote.Name, len(result.Remote.Actions))

	return executor.Local(result.Remote, dir, *dryRun)
}

// cmdInit initializes a remote system for nestor
func cmdInit(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)

	initAgentPath := cfg.Init.Agent
	if initAgentPath == "" {
		initAgentPath = DefaultAgentPath
	}
	initRemoteAgentPath := cfg.Init.RemotePath
	if initRemoteAgentPath == "" {
		initRemoteAgentPath = DefaultRemoteAgentPath
	}

	agentPath := fs.String("agent", initAgentPath,
		"Path to local agent binary")
	sshKey := fs.String("ssh-key", cfg.resolveSSHKey("init"),
		"Path to SSH private key")
	signingKey := fs.String("signing-key", cfg.resolveSigningKey("init"),
		"Path to signing key (defaults to SSH key)")
	remoteAgentPath := fs.String("remote-path", initRemoteAgentPath,
		"Path to install agent on remote system")

	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: nestor init [options] user@host")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		return fmt.Errorf("missing host argument")
	}

	host := fs.Arg(0)

	log.Printf("[INFO ] initializing %s for nestor...\n", host)

	// Create executor
	config := &executor.Config{
		SSHKeyPath:     *sshKey,
		SigningKeyPath: *signingKey,
		AgentPath:      *remoteAgentPath,
	}

	exec, err := executor.New(config)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Initialize remote system
	if err := exec.InitRemote(host, *agentPath); err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	log.Printf("[INFO ] agent installed at: %s\n", *remoteAgentPath)
	log.Println("[INFO ] controller public key added to authorized_keys")
	log.Println("[INFO ] system ready to receive playbooks")
	log.Println("[INFO ] initialization complete")

	return nil
}

// cmdAttach attaches to a running or completed agent
func cmdAttach(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("attach", flag.ExitOnError)

	sshKey := fs.String("ssh-key", cfg.resolveSSHKey("attach"),
		"Path to SSH private key")
	stateFile := fs.String("state-file", cfg.resolveStateFile("attach"),
		"Path to agent state file on remote system")
	follow := fs.Bool("follow", resolveBool(cfg.Attach.Follow),
		"Follow execution in real-time (tail -f style)")
	playbookFile := fs.String("playbook", "",
		"Playbook YAML file; if set and remote succeeded, the post: phase is executed")

	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: nestor attach [options] user@host")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		return fmt.Errorf("missing host argument")
	}

	host := fs.Arg(0)

	log.Printf("[INFO ] attaching to agent on %s...\n", host)

	exec, err := executor.New(&executor.Config{SSHKeyPath: *sshKey})
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	var result *executor.ExecutionResult

	if *follow {
		result, err = exec.AttachAndFollow(host, *stateFile)
	} else {
		result, err = exec.Attach(host, *stateFile)
	}
	if err != nil {
		return fmt.Errorf("failed to attach: %w", err)
	}

	if !*follow {
		displayExecutionResult(result)
	}

	// Run post: phase if a playbook was specified and the remote succeeded.
	if *playbookFile != "" && result.Status == "completed" && result.Summary.Failed == 0 {
		data, err := os.ReadFile(*playbookFile)
		if err != nil {
			return fmt.Errorf("failed to read playbook file: %w", err)
		}
		loaded, err := yamlloader.Load(data, nil)
		if err != nil {
			return fmt.Errorf("failed to load playbook: %w", err)
		}
		if loaded.Post != nil {
			log.Println("[INFO ] executing post: phase on controller...")
			if err := executor.Local(loaded.Post, filepath.Dir(*playbookFile), false); err != nil {
				return fmt.Errorf("post: phase failed: %w", err)
			}
		}
	}

	return nil
}

// cmdStatus checks the status of a remote agent
func cmdStatus(cfg *Config, args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	sshKey := fs.String("ssh-key", cfg.resolveSSHKey("status"),
		"Path to SSH private key")
	stateFile := fs.String("state-file", cfg.resolveStateFile("status"),
		"Path to agent state file on remote system")

	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: nestor status [options] user@host")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		return fmt.Errorf("missing host argument")
	}

	host := fs.Arg(0)

	// Create executor
	config := &executor.Config{
		SSHKeyPath: *sshKey,
	}

	exec, err := executor.New(config)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Check status
	status, err := exec.CheckStatus(host, *stateFile)
	if err != nil {
		return fmt.Errorf("failed to check status: %w", err)
	}

	// Display status
	fmt.Printf("Agent Status on %s:\n", host)
	fmt.Printf("  Status: %s\n", status.Status)
	fmt.Printf("  Playbook: %s\n", status.PlaybookID)
	fmt.Printf("  Progress: %d/%d actions\n", status.CompletedActions, status.TotalActions)

	switch status.Status {
	case "running":
		fmt.Printf("  Running since: %s\n", status.StartTime.Format(time.RFC3339))
		fmt.Printf("  Duration: %s\n", time.Since(status.StartTime).Round(time.Second))
	case "completed":
		fmt.Printf("  Completed at: %s\n", status.CompletedTime.Format(time.RFC3339))
		fmt.Printf("  Duration: %.2f seconds\n", status.Duration)
		fmt.Printf("  Success: %d, Failed: %d, Changed: %d\n",
			status.SuccessCount, status.FailedCount, status.ChangedCount)
	}

	return nil
}

// displayExecutionResult displays a detailed execution result
func displayExecutionResult(result *executor.ExecutionResult) {
	fmt.Println("\n━━━ Execution Result ━━━")
	fmt.Printf("Playbook: %s\n", result.PlaybookID)
	fmt.Printf("Status: %s\n", result.Status)

	if !result.Started.IsZero() {
		fmt.Printf("Started: %s\n", result.Started.Format(time.RFC3339))
	}

	if !result.Completed.IsZero() {
		fmt.Printf("Completed: %s\n", result.Completed.Format(time.RFC3339))
		fmt.Printf("Duration: %.2f seconds\n", result.DurationSeconds)
	}

	fmt.Println("\n━━━ Actions ━━━")
	for _, action := range result.Actions {
		status := "✓"
		switch action.Status {
		case "failed":
			status = "✗"
		case "skipped":
			status = "-"
		}

		changed := ""
		if action.Changed {
			changed = " [changed]"
		}

		fmt.Printf("%s [%s] %s: %s%s\n",
			status, action.ID, action.Type, action.Message, changed)

		if action.Error != "" {
			fmt.Printf("    Error: %s\n", action.Error)
		}
	}

	fmt.Println("\n━━━ Summary ━━━")
	fmt.Printf("Total: %d\n", result.Summary.Total)
	fmt.Printf("  Success: %d\n", result.Summary.Success)
	fmt.Printf("  Failed: %d\n", result.Summary.Failed)
	fmt.Printf("  Skipped: %d\n", result.Summary.Skipped)
	fmt.Printf("  Changed: %d\n", result.Summary.Changed)
}
