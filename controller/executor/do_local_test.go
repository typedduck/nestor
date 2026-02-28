package executor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/typedduck/nestor/controller/executor"
	"github.com/typedduck/nestor/playbook"
)

// commandAction returns a command.execute action for use in local tests.
func commandAction(cmd string) playbook.Action {
	return playbook.Action{
		ID:   "action-001",
		Type: "command.execute",
		Params: map[string]any{
			"command": cmd,
		},
	}
}

// TestLocal_DryRun verifies that Local() completes successfully in dry-run
// mode without executing any real commands.
func TestLocal_DryRun(t *testing.T) {
	dir := t.TempDir()
	pb := &playbook.Playbook{
		Name:    "test-dry-run",
		Actions: []playbook.Action{commandAction(`echo "hello"`)},
	}

	if err := executor.Local(pb, dir, true); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

// TestLocal_UnknownAction verifies that Local() returns an error when the
// playbook contains an action type for which no handler is registered.
func TestLocal_UnknownAction(t *testing.T) {
	dir := t.TempDir()
	pb := &playbook.Playbook{
		Name: "test-unknown-action",
		Actions: []playbook.Action{
			{
				ID:     "action-001",
				Type:   "banana.split",
				Params: map[string]any{},
			},
		},
	}

	if err := executor.Local(pb, dir, false); err == nil {
		t.Fatal("expected error for unregistered action type, got nil")
	}
}

// TestLocal_FileContent verifies end-to-end local execution by writing a
// file via the file.content handler and checking its contents on disk.
func TestLocal_FileContent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "hello.txt")

	pb := &playbook.Playbook{
		Name: "test-file-content",
		Actions: []playbook.Action{
			{
				ID:   "action-001",
				Type: "file.content",
				Params: map[string]any{
					"destination": dest,
					"content":     "hello from nestor\n",
				},
			},
		},
	}

	if err := executor.Local(pb, dir, false); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("expected file to exist at %s: %v", dest, err)
	}
	if string(data) != "hello from nestor\n" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}
