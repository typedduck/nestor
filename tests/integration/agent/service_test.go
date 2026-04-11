//go:build integration

package agent_test

import (
	"context"
	"testing"
	"time"

	"github.com/typedduck/nestor/modules"
	"github.com/typedduck/nestor/playbook/builder"
)

// TestService_Start_WhenStopped verifies that service.start on a stopped cron
// service brings it to the active state (FR-20).
// Depends on: TestMain having stopped cron before m.Run().
func TestService_Start_WhenStopped(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("svc-start-when-stopped")
	if err := modules.Service(b, "cron", "start"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "systemctl", "is-active", "cron")
	if err != nil {
		t.Fatalf("exec is-active: %v", err)
	}
	if code != 0 {
		t.Fatal("cron is not active after service.start")
	}
}

// TestService_Start_Idempotent verifies that service.start on an already-running
// cron service exits with code 0 and produces no error (FR-21).
// Depends on: TestService_Start_WhenStopped having started cron.
func TestService_Start_Idempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("svc-start-idempotent")
	if err := modules.Service(b, "cron", "start"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d (expected 0 for idempotent start):\n%s", code, out)
	}
}

// TestService_Stop_WhenRunning verifies that service.stop on a running cron
// service brings it to an inactive state (FR-22).
// Depends on: TestService_Start_Idempotent leaving cron running.
func TestService_Stop_WhenRunning(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("svc-stop-when-running")
	if err := modules.Service(b, "cron", "stop"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "systemctl", "is-active", "cron")
	if err != nil {
		t.Fatalf("exec is-active: %v", err)
	}
	if code == 0 {
		t.Fatal("cron is still active after service.stop")
	}
}

// TestService_Stop_Idempotent verifies that service.stop on an already-stopped
// cron service exits with code 0 and produces no error (FR-23).
// Depends on: TestService_Stop_WhenRunning having stopped cron.
func TestService_Stop_Idempotent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("svc-stop-idempotent")
	if err := modules.Service(b, "cron", "stop"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d (expected 0 for idempotent stop):\n%s", code, out)
	}
}

// TestService_Restart verifies that service.restart on a running cron service
// leaves it active (FR-24).
func TestService_Restart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Establish known state: cron must be running before restart.
	if code, _, err := suite.exec(ctx, "systemctl", "start", "cron"); err != nil || code != 0 {
		t.Fatalf("pre-condition: start cron: code=%d err=%v", code, err)
	}

	b := builder.New("svc-restart")
	if err := modules.Service(b, "cron", "restart"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "systemctl", "is-active", "cron")
	if err != nil {
		t.Fatalf("exec is-active: %v", err)
	}
	if code != 0 {
		t.Fatal("cron is not active after service.restart")
	}
}

// TestService_Reload verifies that service.reload on a running cron service
// exits with code 0 and leaves the service active (FR-25).
// Depends on: TestService_Restart leaving cron running.
func TestService_Reload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("svc-reload")
	if err := modules.Service(b, "cron", "reload"); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}

	code, _, err := suite.exec(ctx, "systemctl", "is-active", "cron")
	if err != nil {
		t.Fatalf("exec is-active: %v", err)
	}
	if code != 0 {
		t.Fatal("cron is not active after service.reload")
	}
}

// TestService_DaemonReload verifies that modules.DaemonReload exits with code 0
// and produces no error (FR-26).
func TestService_DaemonReload(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	b := builder.New("svc-daemon-reload")
	if err := modules.DaemonReload(b); err != nil {
		t.Fatalf("build playbook: %v", err)
	}

	code, out := runAgent(t, ctx, b.Playbook())
	if code != 0 {
		t.Fatalf("agent exited %d:\n%s", code, out)
	}
}
