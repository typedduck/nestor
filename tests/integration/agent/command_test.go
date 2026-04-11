//go:build integration

package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook/builder"
)

// TestCommand_Execute verifies that modules.Command runs a shell command and
// produces the expected side effect on the remote system (FR-30).
func TestCommand_Execute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("cmd-execute")
	if err := modules.Command(b, "echo hello > /tmp/cmd-out.txt"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	// Verify the file exists and contains the expected content.
	code, _, err := suite.exec(ctx, "grep", "hello", "/tmp/cmd-out.txt")
	if err != nil {
		t.Fatalf("exec grep: %v", err)
	}
	if code != 0 {
		t.Fatal("expected 'hello' in /tmp/cmd-out.txt after command execution")
	}
}

// TestCommand_Creates_WhenAbsent verifies that modules.Command with a Creates
// guard runs the command when the guarded path does not exist (FR-31).
func TestCommand_Creates_WhenAbsent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const guard = "/tmp/cmd-creates-guard"

	b := builder.New("cmd-creates-absent")
	if err := modules.Command(b, "touch "+guard, modules.Creates(guard)); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	// Guard path must exist — the command ran and created it.
	code, _, err := suite.exec(ctx, "stat", guard)
	if err != nil {
		t.Fatalf("exec stat: %v", err)
	}
	if code != 0 {
		t.Fatalf("guard path %s does not exist after command should have run", guard)
	}
}

// TestCommand_Creates_WhenPresent verifies that modules.Command with a Creates
// guard skips execution when the guarded path already exists (FR-32).
func TestCommand_Creates_WhenPresent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	const guard = "/tmp/cmd-skip-guard"
	const proof = "/tmp/cmd-skip-proof"

	// Pre-create the guard so the agent skips the command.
	if code, _, err := suite.exec(ctx, "touch", guard); err != nil || code != 0 {
		t.Fatalf("pre-condition: create guard: code=%d err=%v", code, err)
	}

	b := builder.New("cmd-creates-present")
	if err := modules.Command(b, "touch "+proof, modules.Creates(guard)); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	// Proof path must not exist — the command was skipped.
	code, _, err := suite.exec(ctx, "stat", proof)
	if err != nil {
		t.Fatalf("exec stat: %v", err)
	}
	if code == 0 {
		t.Fatalf("proof path %s exists, but command should have been skipped", proof)
	}
}

// TestScript_Execute verifies that modules.Script uploads and executes a real
// script file, confirmed by a concrete side effect (FR-33).
func TestScript_Execute(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("cmd-script-execute")
	if err := modules.Script(b, "testdata/test-script.sh"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	// Sentinel file must exist — the script was uploaded and executed.
	code, _, err := suite.exec(ctx, "stat", "/tmp/script-ran")
	if err != nil {
		t.Fatalf("exec stat: %v", err)
	}
	if code != 0 {
		t.Fatal("sentinel /tmp/script-ran does not exist after script execution")
	}
}
