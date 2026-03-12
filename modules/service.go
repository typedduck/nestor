package modules

import (
	"fmt"

	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/playbook/builder"
)

// ServiceOption is a functional option for configuring service actions.
type ServiceOption func(*serviceConfig)

type serviceConfig struct {
	runAs string
}

// RunAs runs the service command as the given user via sudo, using a systemd
// user session (systemctl --user). Only valid when the remote init system is
// systemd; the action fails at runtime on sysvinit or openrc.
//
// The XDG_RUNTIME_DIR is derived automatically from the user's UID on the
// remote host so that systemctl can reach the user's D-Bus socket.
//
// Example:
//
//	modules.Service(b, "myapp", "daemon-reload", modules.RunAs("alice"))
func RunAs(user string) ServiceOption {
	return func(cfg *serviceConfig) { cfg.runAs = user }
}

// Service adds a service management action to the playbook.
//
// Supported operations:
//   - "start":   Start the named service (idempotent: no-op if already running)
//   - "stop":    Stop the named service (idempotent: no-op if already stopped)
//   - "restart": Restart the named service (always acts)
//   - "reload":  Reload the named service configuration (always acts)
//
// The agent will use the detected init system (systemd, sysvinit, openrc).
// When RunAs is provided the action uses systemctl --user and is restricted
// to systemd.
//
// Examples:
//
//	modules.Service(b, "nginx", "start")
//	modules.Service(b, "nginx", "reload")
//	modules.Service(b, "myapp", "restart", modules.RunAs("alice"))
func Service(b *builder.Builder, name, operation string, opts ...ServiceOption) error {
	switch operation {
	case "start", "stop", "restart", "reload":
	default:
		return fmt.Errorf("unknown service operation: %s (valid: start, stop, restart, reload)", operation)
	}

	cfg := &serviceConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	params := map[string]any{"name": name}
	if cfg.runAs != "" {
		params["run_as"] = cfg.runAs
	}

	b.AddAction(playbook.Action{
		Type:   "service." + operation,
		Params: params,
	})
	return nil
}
