package handlers

import (
	"fmt"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// serviceUserCmd builds the sudo-wrapped command for running systemctl --user
// as a specific user. The XDG_RUNTIME_DIR is resolved on the remote host via
// $(id -u) so that systemctl can reach the user's D-Bus socket.
func serviceUserCmd(runAs, op, name string) (string, []string) {
	shellCmd := fmt.Sprintf(
		"XDG_RUNTIME_DIR=/run/user/$(id -u) systemctl --user %s %s",
		op, name,
	)
	return "sudo", []string{"-u", runAs, "/bin/sh", "-c", shellCmd}
}

// serviceCommand returns the command name and argument list for the given
// init system, service name, and operation.
func serviceCommand(initSystem, name, op, runAs string) (string, []string, error) {
	if runAs != "" {
		// Already validated that initSystem == "systemd" before this call.
		cmd, args := serviceUserCmd(runAs, op, name)
		return cmd, args, nil
	}
	switch initSystem {
	case "systemd":
		return "systemctl", []string{op, name}, nil
	case "sysvinit":
		return "/etc/init.d/" + name, []string{op}, nil
	case "openrc":
		return "rc-service", []string{name, op}, nil
	default:
		return "", nil, fmt.Errorf("unsupported init system: %s", initSystem)
	}
}

// serviceIsActive reports whether the named service is currently running.
// It returns true when the status command exits with code 0.
func serviceIsActive(cmd executor.CommandRunner, initSystem, name, runAs string) (bool, error) {
	var cmdName string
	var args []string

	if runAs != "" {
		// Already validated that initSystem == "systemd" before this call.
		cmdName, args = serviceUserCmd(runAs, "is-active --quiet", name)
	} else {
		switch initSystem {
		case "systemd":
			cmdName = "systemctl"
			args = []string{"is-active", "--quiet", name}
		case "sysvinit":
			cmdName = "/etc/init.d/" + name
			args = []string{"status"}
		case "openrc":
			cmdName = "rc-service"
			args = []string{name, "status"}
		default:
			return false, fmt.Errorf("unsupported init system: %s", initSystem)
		}
	}

	_, exitCode, err := cmd.CombinedOutput(cmdName, nil, args...)
	if err != nil && exitCode == 0 {
		// Unexpected error (not just a non-zero exit code).
		return false, err
	}
	return exitCode == 0, nil
}

// --- ServiceStartHandler ---

// ServiceStartHandler starts a service if it is not already running.
type ServiceStartHandler struct{}

func NewServiceStartHandler() *ServiceStartHandler { return &ServiceStartHandler{} }

func (h *ServiceStartHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	name := getStringParam(action.Params, "name", "")
	if name == "" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Missing required parameter: name",
			Error:   "name parameter is empty",
		}
	}

	initSystem := context.SystemInfo.InitSystem
	if initSystem == "unknown" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Init system not detected",
			Error:   "unable to detect init system",
		}
	}

	runAs := getStringParam(action.Params, "run_as", "")
	if runAs != "" && initSystem != "systemd" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("RunAs requires systemd, got %s", initSystem),
			Error:   "run_as is only supported with the systemd init system",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would start service %s", name),
		}
	}

	active, err := serviceIsActive(context.Cmd, initSystem, name, runAs)
	if err != nil {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("Failed to check status of service %s", name),
			Error:   err.Error(),
		}
	}
	if active {
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: fmt.Sprintf("Service %s is already running", name),
		}
	}

	cmdName, args, err := serviceCommand(initSystem, name, "start", runAs)
	if err != nil {
		return executor.ActionResult{Status: "failed", Error: err.Error()}
	}

	output, _, err := context.Cmd.CombinedOutput(cmdName, nil, args...)
	if err != nil {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("Failed to start service %s", name),
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Started service %s", name),
	}
}

// --- ServiceStopHandler ---

// ServiceStopHandler stops a service if it is currently running.
type ServiceStopHandler struct{}

func NewServiceStopHandler() *ServiceStopHandler { return &ServiceStopHandler{} }

func (h *ServiceStopHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	name := getStringParam(action.Params, "name", "")
	if name == "" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Missing required parameter: name",
			Error:   "name parameter is empty",
		}
	}

	initSystem := context.SystemInfo.InitSystem
	if initSystem == "unknown" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Init system not detected",
			Error:   "unable to detect init system",
		}
	}

	runAs := getStringParam(action.Params, "run_as", "")
	if runAs != "" && initSystem != "systemd" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("RunAs requires systemd, got %s", initSystem),
			Error:   "run_as is only supported with the systemd init system",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would stop service %s", name),
		}
	}

	active, err := serviceIsActive(context.Cmd, initSystem, name, runAs)
	if err != nil {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("Failed to check status of service %s", name),
			Error:   err.Error(),
		}
	}
	if !active {
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: fmt.Sprintf("Service %s is already stopped", name),
		}
	}

	cmdName, args, err := serviceCommand(initSystem, name, "stop", runAs)
	if err != nil {
		return executor.ActionResult{Status: "failed", Error: err.Error()}
	}

	output, _, err := context.Cmd.CombinedOutput(cmdName, nil, args...)
	if err != nil {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("Failed to stop service %s", name),
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Stopped service %s", name),
	}
}

// --- ServiceRestartHandler ---

// ServiceRestartHandler always restarts the named service.
type ServiceRestartHandler struct{}

func NewServiceRestartHandler() *ServiceRestartHandler { return &ServiceRestartHandler{} }

func (h *ServiceRestartHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	name := getStringParam(action.Params, "name", "")
	if name == "" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Missing required parameter: name",
			Error:   "name parameter is empty",
		}
	}

	initSystem := context.SystemInfo.InitSystem
	if initSystem == "unknown" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Init system not detected",
			Error:   "unable to detect init system",
		}
	}

	runAs := getStringParam(action.Params, "run_as", "")
	if runAs != "" && initSystem != "systemd" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("RunAs requires systemd, got %s", initSystem),
			Error:   "run_as is only supported with the systemd init system",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would restart service %s", name),
		}
	}

	cmdName, args, err := serviceCommand(initSystem, name, "restart", runAs)
	if err != nil {
		return executor.ActionResult{Status: "failed", Error: err.Error()}
	}

	output, _, err := context.Cmd.CombinedOutput(cmdName, nil, args...)
	if err != nil {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("Failed to restart service %s", name),
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Restarted service %s", name),
	}
}

// --- ServiceReloadHandler ---

// ServiceReloadHandler always reloads the named service's configuration.
type ServiceReloadHandler struct{}

func NewServiceReloadHandler() *ServiceReloadHandler { return &ServiceReloadHandler{} }

func (h *ServiceReloadHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	name := getStringParam(action.Params, "name", "")
	if name == "" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Missing required parameter: name",
			Error:   "name parameter is empty",
		}
	}

	initSystem := context.SystemInfo.InitSystem
	if initSystem == "unknown" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: "Init system not detected",
			Error:   "unable to detect init system",
		}
	}

	runAs := getStringParam(action.Params, "run_as", "")
	if runAs != "" && initSystem != "systemd" {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("RunAs requires systemd, got %s", initSystem),
			Error:   "run_as is only supported with the systemd init system",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would reload service %s", name),
		}
	}

	cmdName, args, err := serviceCommand(initSystem, name, "reload", runAs)
	if err != nil {
		return executor.ActionResult{Status: "failed", Error: err.Error()}
	}

	output, _, err := context.Cmd.CombinedOutput(cmdName, nil, args...)
	if err != nil {
		return executor.ActionResult{
			Status: "failed", Changed: false,
			Message: fmt.Sprintf("Failed to reload service %s", name),
			Error:   fmt.Sprintf("%s: %s", err.Error(), string(output)),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Reloaded service %s", name),
	}
}
