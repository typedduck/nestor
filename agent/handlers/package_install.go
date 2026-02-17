package handlers

import (
	"fmt"
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
func (h *PackageInstallHandler) Execute(action executor.Action,
	context *executor.ExecutionContext) executor.ActionResult {
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
	case []any:
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

	cmd := context.Cmd

	// Check which packages are already installed
	toInstall := []string{}
	for _, pkg := range packages {
		installed, err := h.isPackageInstalled(cmd, pm, pkg)
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
		if err := h.updateCache(cmd, pm); err != nil {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: "Failed to update package cache",
				Error:   err.Error(),
			}
		}
	}

	// Install missing packages
	if err := h.installPackages(cmd, pm, toInstall); err != nil {
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
func (h *PackageInstallHandler) isPackageInstalled(cmd executor.CommandRunner,
	pm, pkg string) (bool, error) {
	switch pm {
	case "apt":
		output, _, err := cmd.CombinedOutput("dpkg-query", nil, "-W", "-f=${Status}", pkg)
		if err != nil {
			// Package not found
			return false, nil
		}
		return strings.Contains(string(output), "install ok installed"), nil

	case "yum", "dnf":
		err := cmd.Run(pm, nil, "list", "installed", pkg)
		return err == nil, nil

	default:
		return false, fmt.Errorf("unsupported package manager: %s", pm)
	}
}

// updateCache updates the package manager cache
func (h *PackageInstallHandler) updateCache(cmd executor.CommandRunner, pm string) error {
	switch pm {
	case "apt":
		output, _, err := cmd.CombinedOutput("apt-get", nil, "update", "-qq")
		if err != nil {
			return fmt.Errorf("update cache failed: %s", string(output))
		}
		return nil

	case "yum":
		output, exitCode, err := cmd.CombinedOutput("yum", nil, "check-update", "-q")
		if err != nil {
			// yum returns exit code 100 when updates are available
			if exitCode == 100 {
				return nil
			}
			return fmt.Errorf("update cache failed: %s", string(output))
		}
		return nil

	case "dnf":
		output, exitCode, err := cmd.CombinedOutput("dnf", nil, "check-update", "-q")
		if err != nil {
			// dnf returns exit code 100 when updates are available
			if exitCode == 100 {
				return nil
			}
			return fmt.Errorf("update cache failed: %s", string(output))
		}
		return nil

	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}
}

// installPackages installs the specified packages
func (h *PackageInstallHandler) installPackages(cmd executor.CommandRunner, pm string,
	packages []string) error {
	var name string
	var args []string
	opts := &executor.CommandOpts{
		Env: []string{"DEBIAN_FRONTEND=noninteractive"},
	}

	switch pm {
	case "apt":
		name = "apt-get"
		args = append([]string{"install", "-y", "-qq"}, packages...)
	case "yum":
		name = "yum"
		args = append([]string{"install", "-y", "-q"}, packages...)
	case "dnf":
		name = "dnf"
		args = append([]string{"install", "-y", "-q"}, packages...)
	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, _, err := cmd.CombinedOutput(name, opts, args...)
	if err != nil {
		return fmt.Errorf("install failed: %s", string(output))
	}

	return nil
}
