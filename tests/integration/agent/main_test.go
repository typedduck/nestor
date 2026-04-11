//go:build integration

package agent_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

var suite *agentSuite

func TestMain(m *testing.M) {
	os.Exit(runSuite(m))
}

func runSuite(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	agentBin, err := buildAgentBinary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build agent binary: %v\n", err)
		return 1
	}
	defer os.Remove(agentBin)

	provider := providerFromEnv()

	s, err := newAgentSuite(ctx, provider, agentBin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start container: %v\n", err)
		return 1
	}
	suite = s
	defer s.teardown(context.Background())

	// Stop cron before service tests begin so they start from a known stopped state.
	if code, _, err := suite.exec(ctx, "systemctl", "stop", "cron"); err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "suite setup: stop cron: code=%d err=%v\n", code, err)
		return 1
	}

	return m.Run()
}

// providerFromEnv reads TC_PROVIDER_TYPE from the environment.
// "podman" maps to ProviderPodman; anything else maps to ProviderDocker.
func providerFromEnv() testcontainers.ProviderType {
	if os.Getenv("TC_PROVIDER_TYPE") == "podman" {
		return testcontainers.ProviderPodman
	}
	return testcontainers.ProviderDocker
}

// buildAgentBinary cross-compiles the agent for linux/<runtime.GOARCH> and
// returns the path to the temporary binary.
func buildAgentBinary() (string, error) {
	// Resolve the repository root relative to this file's location.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("runtime.Caller failed")
	}
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")

	tmpFile, err := os.CreateTemp("", "nestor-agent-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpFile.Close()
	binPath := tmpFile.Name()

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/nestor-agent")
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(),
		"GOOS=linux",
		"GOARCH="+runtime.GOARCH,
		"CGO_ENABLED=0",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		os.Remove(binPath)
		return "", fmt.Errorf("go build: %w\n%s", err, out)
	}

	return binPath, nil
}
