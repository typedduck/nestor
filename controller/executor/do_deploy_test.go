package executor_test

import (
	"strings"
	"testing"

	"github.com/typedduck/nestor/controller/executor"
	"github.com/typedduck/nestor/playbook"
)

// minimalPlaybook returns a minimal valid playbook for use in deploy tests.
func minimalPlaybook() *playbook.Playbook {
	return &playbook.Playbook{
		Name:    "test",
		Actions: []playbook.Action{},
	}
}

// TestDeploy_RejectsDryRunWithPre verifies that Deploy returns an error
// immediately when DryRun=true and a pre: phase is present.
func TestDeploy_RejectsDryRunWithPre(t *testing.T) {
	dir := t.TempDir()
	cfg := &executor.Config{DryRun: true, WorkDir: dir}
	d := &executor.Deployment{
		Pre:         minimalPlaybook(),
		Remote:      minimalPlaybook(),
		PlaybookDir: dir,
	}
	err := executor.Deploy(d, "user@localhost", cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--dry-run is not supported") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDeploy_RejectsDryRunWithPost verifies that Deploy returns an error
// immediately when DryRun=true and a post: phase is present.
func TestDeploy_RejectsDryRunWithPost(t *testing.T) {
	dir := t.TempDir()
	cfg := &executor.Config{DryRun: true, WorkDir: dir}
	d := &executor.Deployment{
		Remote:      minimalPlaybook(),
		Post:        minimalPlaybook(),
		PlaybookDir: dir,
	}
	err := executor.Deploy(d, "user@localhost", cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "--dry-run is not supported") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestDeploy_DryRunWithoutPhases verifies that the dry-run guard does not
// fire when Pre and Post are nil. The deployment may fail at a later stage
// (packaging, signing) but must not fail with the guard error.
func TestDeploy_DryRunWithoutPhases(t *testing.T) {
	dir := t.TempDir()
	cfg := &executor.Config{DryRun: true, WorkDir: dir}
	d := &executor.Deployment{
		Remote:      minimalPlaybook(),
		PlaybookDir: dir,
	}
	err := executor.Deploy(d, "user@localhost", cfg)
	if err != nil && strings.Contains(err.Error(), "--dry-run is not supported") {
		t.Fatalf("guard should not fire with nil Pre/Post: %v", err)
	}
}
