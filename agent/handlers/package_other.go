package handlers

import (
	"fmt"
	"strings"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// --- PackageRemoveHandler ---

// PackageRemoveHandler removes packages that are currently installed.
type PackageRemoveHandler struct{}

func NewPackageRemoveHandler() *PackageRemoveHandler { return &PackageRemoveHandler{} }

func (h *PackageRemoveHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	packagesInterface, ok := action.Params["packages"]
	if !ok {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: packages",
			Error:   "packages parameter not found",
		}
	}

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
			Message: "No packages to remove",
		}
	}

	pm := context.SystemInfo.PackageManager
	if pm == "unknown" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Package manager not detected",
			Error:   "unable to detect package manager",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would remove %d packages: %s",
				len(packages), strings.Join(packages, ", ")),
		}
	}

	cmd := context.Cmd

	// Check which packages are actually installed
	toRemove := []string{}
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
		if installed {
			toRemove = append(toRemove, pkg)
		}
	}

	if len(toRemove) == 0 {
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: fmt.Sprintf("None of the %d packages are installed", len(packages)),
		}
	}

	if err := h.removePackages(cmd, pm, toRemove); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to remove packages: %s", strings.Join(toRemove, ", ")),
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Removed %d packages: %s",
			len(toRemove), strings.Join(toRemove, ", ")),
	}
}

func (h *PackageRemoveHandler) isPackageInstalled(cmd executor.CommandRunner,
	pm, pkg string) (bool, error) {
	switch pm {
	case "apt":
		output, _, err := cmd.CombinedOutput("dpkg-query", nil, "-W", "-f=${Status}", pkg)
		if err != nil {
			return false, nil
		}
		return strings.Contains(string(output), "install ok installed"), nil

	case "yum", "dnf":
		err := cmd.Run(pm, nil, "list", "installed", pkg)
		return err == nil, nil

	case "brew":
		err := cmd.Run("brew", nil, "list", "--formula", pkg)
		return err == nil, nil

	default:
		return false, fmt.Errorf("unsupported package manager: %s", pm)
	}
}

func (h *PackageRemoveHandler) removePackages(cmd executor.CommandRunner,
	pm string, packages []string) error {
	var name string
	var args []string
	var opts *executor.CommandOpts

	switch pm {
	case "apt":
		name = "apt-get"
		args = append([]string{"remove", "-y", "-qq"}, packages...)
		opts = &executor.CommandOpts{Env: []string{"DEBIAN_FRONTEND=noninteractive"}}
	case "yum":
		name = "yum"
		args = append([]string{"remove", "-y", "-q"}, packages...)
	case "dnf":
		name = "dnf"
		args = append([]string{"remove", "-y", "-q"}, packages...)
	case "brew":
		name = "brew"
		args = append([]string{"uninstall"}, packages...)
	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, _, err := cmd.CombinedOutput(name, opts, args...)
	if err != nil {
		return fmt.Errorf("remove failed: %s", string(output))
	}
	return nil
}

// --- PackageUpdateHandler ---

// PackageUpdateHandler updates the package manager cache.
type PackageUpdateHandler struct{}

func NewPackageUpdateHandler() *PackageUpdateHandler { return &PackageUpdateHandler{} }

func (h *PackageUpdateHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	pm := context.SystemInfo.PackageManager
	if pm == "unknown" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Package manager not detected",
			Error:   "unable to detect package manager",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would update %s package cache", pm),
		}
	}

	if err := h.updateCache(context.Cmd, pm); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Failed to update package cache",
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Updated %s package cache", pm),
	}
}

func (h *PackageUpdateHandler) updateCache(cmd executor.CommandRunner, pm string) error {
	switch pm {
	case "apt":
		output, _, err := cmd.CombinedOutput("apt-get", nil, "update", "-qq")
		if err != nil {
			return fmt.Errorf("update failed: %s", string(output))
		}
		return nil

	case "yum":
		output, exitCode, err := cmd.CombinedOutput("yum", nil, "check-update", "-q")
		if err != nil && exitCode != 100 {
			return fmt.Errorf("update failed: %s", string(output))
		}
		return nil

	case "dnf":
		output, exitCode, err := cmd.CombinedOutput("dnf", nil, "check-update", "-q")
		if err != nil && exitCode != 100 {
			return fmt.Errorf("update failed: %s", string(output))
		}
		return nil

	case "brew":
		output, _, err := cmd.CombinedOutput("brew", nil, "update")
		if err != nil {
			return fmt.Errorf("brew update failed: %s", string(output))
		}
		return nil

	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}
}

// --- PackageUpgradeHandler ---

// PackageUpgradeHandler upgrades all installed packages.
type PackageUpgradeHandler struct{}

func NewPackageUpgradeHandler() *PackageUpgradeHandler { return &PackageUpgradeHandler{} }

func (h *PackageUpgradeHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	pm := context.SystemInfo.PackageManager
	if pm == "unknown" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Package manager not detected",
			Error:   "unable to detect package manager",
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would upgrade all %s packages", pm),
		}
	}

	if err := h.upgradePackages(context.Cmd, pm); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Failed to upgrade packages",
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Upgraded all %s packages", pm),
	}
}

func (h *PackageUpgradeHandler) upgradePackages(cmd executor.CommandRunner, pm string) error {
	var name string
	var args []string
	var opts *executor.CommandOpts

	switch pm {
	case "apt":
		name = "apt-get"
		args = []string{"upgrade", "-y", "-qq"}
		opts = &executor.CommandOpts{Env: []string{"DEBIAN_FRONTEND=noninteractive"}}
	case "yum":
		name = "yum"
		args = []string{"update", "-y", "-q"}
	case "dnf":
		name = "dnf"
		args = []string{"update", "-y", "-q"}
	case "brew":
		name = "brew"
		args = []string{"upgrade"}
	default:
		return fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, _, err := cmd.CombinedOutput(name, opts, args...)
	if err != nil {
		return fmt.Errorf("upgrade failed: %s", string(output))
	}
	return nil
}
