//go:build integration

package agent_test

import (
	"context"
	"io"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// agentSuite holds the running container for the integration test session.
type agentSuite struct {
	container testcontainers.Container
}

// newAgentSuite starts the systemd container, copies the agent binary into it,
// and returns a ready-to-use agentSuite.
func newAgentSuite(ctx context.Context, provider testcontainers.ProviderType,
	agentBinPath string) (*agentSuite, error) {
	c, err := startContainer(ctx, provider)
	if err != nil {
		return nil, err
	}

	if err := copyAgentBinary(ctx, c, agentBinPath); err != nil {
		_ = c.Terminate(ctx)
		return nil, err
	}

	return &agentSuite{container: c}, nil
}

// startContainer builds and starts the Debian 12 + systemd container.
func startContainer(ctx context.Context,
	provider testcontainers.ProviderType) (testcontainers.Container, error) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    ".",
				Dockerfile: "Containerfile",
				KeepImage:  true,
			},
			Privileged: true, //nolint:staticcheck
			Tmpfs: map[string]string{
				"/run":      "rw",
				"/run/lock": "rw",
			},
			WaitingFor: wait.ForExec([]string{"systemctl", "is-active", "basic.target"}).
				WithStartupTimeout(90 * time.Second),
		},
		Started:      true,
		ProviderType: provider,
	}

	return testcontainers.GenericContainer(ctx, req)
}

// copyAgentBinary copies the compiled agent binary into the container.
func copyAgentBinary(ctx context.Context, c testcontainers.Container, hostPath string) error {
	return c.CopyFileToContainer(ctx, hostPath, "/usr/local/bin/nestor-agent", 0755)
}

// exec runs a command inside the container and returns the exit code, combined
// stdout+stderr output, and any transport error.
func (s *agentSuite) exec(ctx context.Context, cmd ...string) (int, string, error) {
	exitCode, reader, err := s.container.Exec(ctx, cmd)
	if err != nil {
		return exitCode, "", err
	}
	out, _ := io.ReadAll(reader)
	return exitCode, string(out), nil
}

// teardown stops and removes the container.
func (s *agentSuite) teardown(ctx context.Context) {
	_ = s.container.Terminate(ctx)
}
