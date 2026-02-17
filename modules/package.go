package modules

import (
	"fmt"

	"github.com/typedduck/nestor/playbook"
)

// Package adds package management actions to the playbook.
//
// Supported operations:
//   - "install": Install one or more packages
//   - "remove": Remove one or more packages
//   - "update": Update the package cache
//   - "upgrade": Upgrade all installed packages
//
// The Package module generates actions that are idempotent - they can be
// run multiple times safely. The agent will detect the package manager
// (apt, yum, dnf) and execute the appropriate commands.
//
// Examples:
//
//	modules.Package(pb, "install", "vim", "git", "htop")
//	modules.Package(pb, "remove", "apache2")
//	modules.Package(pb, "update")
//	modules.Package(pb, "upgrade")
func Package(pb *playbook.Playbook, operation string, packages ...string) error {
	switch operation {
	case "install":
		return packageInstall(pb, packages)
	case "remove":
		return packageRemove(pb, packages)
	case "update":
		return packageUpdate(pb)
	case "upgrade":
		return packageUpgrade(pb)
	default:
		return fmt.Errorf("unknown package operation: %s (valid: install, remove, update, upgrade)", operation)
	}
}

// packageInstall adds an action to install one or more packages.
func packageInstall(pb *playbook.Playbook, packages []string) error {
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for install operation")
	}

	action := playbook.Action{
		Type: "package.install",
		Params: map[string]interface{}{
			"packages":     packages,
			"update_cache": true,
		},
	}

	pb.AddAction(action)
	return nil
}

// packageRemove adds an action to remove one or more packages.
func packageRemove(pb *playbook.Playbook, packages []string) error {
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified for remove operation")
	}

	action := playbook.Action{
		Type: "package.remove",
		Params: map[string]interface{}{
			"packages": packages,
		},
	}

	pb.AddAction(action)
	return nil
}

// packageUpdate adds an action to update the package cache.
// This is equivalent to 'apt update' or 'yum check-update'.
func packageUpdate(pb *playbook.Playbook) error {
	action := playbook.Action{
		Type:   "package.update",
		Params: map[string]interface{}{},
	}

	pb.AddAction(action)
	return nil
}

// packageUpgrade adds an action to upgrade all installed packages.
// This is equivalent to 'apt upgrade' or 'yum update'.
func packageUpgrade(pb *playbook.Playbook) error {
	action := playbook.Action{
		Type:   "package.upgrade",
		Params: map[string]interface{}{},
	}

	pb.AddAction(action)
	return nil
}

// PackageOptions provides additional configuration for package operations.
type PackageOptions struct {
	UpdateCache    bool
	AllowDowngrade bool
	Force          bool
}

// PackageWithOptions adds a package installation action with custom options.
// This provides more control over the package installation process.
//
// Example:
//
//	opts := &modules.PackageOptions{
//	    UpdateCache: true,
//	    AllowDowngrade: false,
//	    Force: false,
//	}
//	modules.PackageWithOptions(pb, "install", []string{"nginx"}, opts)
func PackageWithOptions(pb *playbook.Playbook, operation string, packages []string, opts *PackageOptions) error {
	if opts == nil {
		opts = &PackageOptions{
			UpdateCache:    true,
			AllowDowngrade: false,
			Force:          false,
		}
	}

	switch operation {
	case "install":
		if len(packages) == 0 {
			return fmt.Errorf("no packages specified for install operation")
		}

		action := playbook.Action{
			Type: "package.install",
			Params: map[string]interface{}{
				"packages":        packages,
				"update_cache":    opts.UpdateCache,
				"allow_downgrade": opts.AllowDowngrade,
				"force":           opts.Force,
			},
		}

		pb.AddAction(action)
		return nil

	case "remove":
		if len(packages) == 0 {
			return fmt.Errorf("no packages specified for remove operation")
		}

		action := playbook.Action{
			Type: "package.remove",
			Params: map[string]interface{}{
				"packages": packages,
				"force":    opts.Force,
			},
		}

		pb.AddAction(action)
		return nil

	default:
		return fmt.Errorf("unknown package operation: %s", operation)
	}
}
