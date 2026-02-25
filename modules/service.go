package modules

import (
	"fmt"

	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/playbook/builder"
)

// Service adds a service management action to the playbook.
//
// Supported operations:
//   - "start":   Start the named service (idempotent: no-op if already running)
//   - "stop":    Stop the named service (idempotent: no-op if already stopped)
//   - "restart": Restart the named service (always acts)
//   - "reload":  Reload the named service configuration (always acts)
//
// The agent will use the detected init system (systemd, sysvinit, openrc).
//
// Examples:
//
//	modules.Service(b, "nginx", "start")
//	modules.Service(b, "nginx", "reload")
func Service(b *builder.Builder, name, operation string) error {
	switch operation {
	case "start", "stop", "restart", "reload":
		b.AddAction(playbook.Action{
			Type:   "service." + operation,
			Params: map[string]any{"name": name},
		})
		return nil
	default:
		return fmt.Errorf("unknown service operation: %s (valid: start, stop, restart, reload)", operation)
	}
}
