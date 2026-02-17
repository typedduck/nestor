package handlers

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/typedduck/nestor/agent/executor"
)

// PackageInstallHandler handles package installation
type PackageInstallHandler struct{}

// NewPackageInstallHandler creates a new package install handler
func NewPackageInstallHandler() *PackageInstallHandler {
	return &PackageInstallHandler{}
}

// Execute installs packages using the system package manager
func (h *PackageInstallHandler) Execute(action executor.Action, context *executor.ExecutionContext) executor.ActionResult {
	// Extract parameters
	packagesInterface, ok := action.Params["packages"]
	if !ok {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: packages",
			Error:   "packages parameter not found",
		}
	}

	// Convert packages to string slice
	var packages []string
	switch v := packagesInterface.(type) {
	case []interface{}:
		for _, pkg := range v {
			if pkgStr, ok := pkg.(string); ok {
				packages = append(packages, pkgStr)
			}
		}
	case []string:
		packages = v
	default:
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Invalid packages parameter type",
			Error:   fmt.Sprintf("expected array, got %T", packagesInterface),
		}
	}

	if len(packages) == 0 {
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: "No packages to install",
		}
	}

	updateCache := getBoolParam(action.Params, "update_cache", true)

	// Get package manager
	pm := context.SystemInfo.PackageManager
	if pm == "unknown" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Package manager not detected",
			Error:   "unable to detect package manager",
		}
	}

	// Dry run mode
	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would install %d packages: %s",
				len(packages), strings.Join(packages, ", ")),
		}
	}

	// Check which packages are already installed
	toInstall := []string{}
	for _, pkg := range packages {
		installed, err := h.isPackageInstalled(pm, pkg)
		if err != nil {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: fmt.Sprintf("Failed to check if %s is installed", pkg),
				Error:   err.Error(),
			}
		}
		if !installed {
			toInstall = append(toInstall, pkg)
		}
	}

	// If all packages are installed, we're done
	if len(toInstall) == 0 {
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: fmt.Sprintf("All %d packages already installed", len(packages)),
		}
	}

	// Update package cache if requested
	if updateCache {
		if err := h.updateCache(pm); err != nil {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: "Failed to update package cache",
				Error:   err.Error(),
			}
		}
	}

	// Install missing packages
	if err := h.installPackages(pm, toInstall); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to install packages: %s", strings.Join(toInstall, ", ")),
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Installed %d packages: %s",
			len(toInstall), strings.Join(toInstall, ", ")),
	}
}

// isPackageInstalled checks if a package is installed
func (h *PackageInstallHandler) isPackageInstalled(pm, pkg string) (bool, error) {
	var cmd *exec.Cmd

	switch pm {
	case "apt":
		cmd = exec.Command("dpkg-query", "-W", "-f=${Status}", pkg)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Package not found
			return false, nil
		}
		return strings.Contains(string(output), "install ok installed"), nil

	case "yum", "dnf":
		cmd = exec.Command(pm, "list", "installed", pkg)
		err := cmd.Run()
		return err == nil, nil

	default:
		return false, fmt.Errorf("unsupported package manager: %s", pm)
	}
}

// updateCache updates the package manager cache
func (h *PackageInstallHandler) updateCache(pm string) error {
	var cmd *exec.Cmd

	switch pm {
	case "apt":
		cmd = exec.Command("apt-get", "update", "-qq")
	case "yum":
		cmd = exec.Command("yum", "check-update", "-q")
	case "dnf":
		cmd = exec.Command("dnf", "check-update", "-q")
	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// yum/dnf return exit code 100 when updates are available
		if pm == "yum" || pm == "dnf" {
			if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 100 {
				return nil
			}
		}
		return fmt.Errorf("update cache failed: %s", string(output))
	}

	return nil
}

// installPackages installs the specified packages
func (h *PackageInstallHandler) installPackages(pm string, packages []string) error {
	var cmd *exec.Cmd

	switch pm {
	case "apt":
		args := append([]string{"install", "-y", "-qq"}, packages...)
		cmd = exec.Command("apt-get", args...)
	case "yum":
		args := append([]string{"install", "-y", "-q"}, packages...)
		cmd = exec.Command("yum", args...)
	case "dnf":
		args := append([]string{"install", "-y", "-q"}, packages...)
		cmd = exec.Command("dnf", args...)
	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	// Set environment to non-interactive
	cmd.Env = append(cmd.Env, "DEBIAN_FRONTEND=noninteractive")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %s", string(output))
	}

	return nil
}

// getBoolParam gets a boolean parameter with a default value
func getBoolParam(params map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := params[key]; ok {
		if boolVal, ok := val.(bool); ok {
			return boolVal
		}
	}
	return defaultValue
}
