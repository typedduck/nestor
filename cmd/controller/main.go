package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/typedduck/nestor/controller/executor"
	"github.com/typedduck/nestor/playbook/yamlloader"
)

const (
	Version        = "dev"
	ControllerName = "nestor"
)

// Command represents a CLI command
type Command struct {
	Name        string
	Description string
	Execute     func(args []string) error
}

var commands = []Command{
	{
		Name:        "apply",
		Description: "Apply a YAML playbook to a remote host",
		Execute:     cmdApply,
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

	cmdName := os.Args[1]

	// Find and execute command
	for _, cmd := range commands {
		if cmd.Name == cmdName {
			if err := cmd.Execute(os.Args[2:]); err != nil {
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
func cmdApply(args []string) error {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)

	var vars varFlags
	fs.Var(&vars, "var", "Set a playbook variable (key=value), may be repeated")
	sshKey := fs.String("ssh-key", os.Getenv("HOME")+"/.ssh/id_ed25519", "SSH private key")
	signingKey := fs.String("signing-key", "", "Signing key (defaults to SSH key)")
	knownHosts := fs.String("known-hosts", os.Getenv("HOME")+"/.ssh/known_hosts", "known_hosts file")
	dryRun := fs.Bool("dry-run", false, "Package and sign without deploying")

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
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			return fmt.Errorf("-var %q must be in key=value form", kv)
		}
		varMap[kv[:idx]] = kv[idx+1:]
	}

	data, err := os.ReadFile(playbookFile)
	if err != nil {
		return fmt.Errorf("failed to read playbook file: %w", err)
	}

	b, err := yamlloader.Load(data, varMap)
	if err != nil {
		return fmt.Errorf("failed to load playbook: %w", err)
	}

	pb := b.Playbook()
	log.Printf("[INFO ] loaded %q: %d action(s)", pb.Name, len(pb.Actions))

	return executor.Deploy(pb, host, &executor.Config{
		SSHKeyPath:     *sshKey,
		SigningKeyPath: *signingKey,
		KnownHostsPath: *knownHosts,
		DryRun:         *dryRun,
	})
}

// cmdInit initializes a remote system for nestor
func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)

	agentPath := fs.String("agent", "./build/nestor-agent",
		"Path to local agent binary")
	sshKey := fs.String("ssh-key", os.Getenv("HOME")+"/.ssh/id_rsa",
		"Path to SSH private key")
	signingKey := fs.String("signing-key", "",
		"Path to signing key (defaults to SSH key)")
	remoteAgentPath := fs.String("remote-path", "/usr/local/bin/nestor-agent",
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
func cmdAttach(args []string) error {
	fs := flag.NewFlagSet("attach", flag.ExitOnError)

	sshKey := fs.String("ssh-key", os.Getenv("HOME")+"/.ssh/id_rsa",
		"Path to SSH private key")
	stateFile := fs.String("state-file", "/tmp/nestor-agent.state",
		"Path to agent state file on remote system")
	follow := fs.Bool("follow", false,
		"Follow execution in real-time (tail -f style)")

	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Println("Usage: nestor attach [options] user@host")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		return fmt.Errorf("missing host argument")
	}

	host := fs.Arg(0)

	log.Printf("[INFO ] attaching to agent on %s...\n", host)

	// Create executor
	config := &executor.Config{
		SSHKeyPath: *sshKey,
	}

	exec, err := executor.New(config)
	if err != nil {
		return fmt.Errorf("failed to create executor: %w", err)
	}

	// Attach to remote agent
	if *follow {
		return exec.AttachAndFollow(host, *stateFile)
	}

	result, err := exec.Attach(host, *stateFile)
	if err != nil {
		return fmt.Errorf("failed to attach: %w", err)
	}

	// Display results
	displayExecutionResult(result)

	return nil
}

// cmdStatus checks the status of a remote agent
func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)

	sshKey := fs.String("ssh-key", os.Getenv("HOME")+"/.ssh/id_rsa",
		"Path to SSH private key")
	stateFile := fs.String("state-file", "/tmp/nestor-agent.state",
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
