package modules

import (
	"testing"

	"github.com/typedduck/nestor/playbook"
)

// TestPackageInstall verifies that Package install operation adds correct actions
func TestPackageInstall(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Package(pb, "install", "vim", "git", "htop")
	if err != nil {
		t.Fatalf("Package install failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "package.install" {
		t.Errorf("Expected action type 'package.install', got '%s'", action.Type)
	}

	packages, ok := action.Params["packages"].([]string)
	if !ok {
		t.Fatal("packages param is not a string slice")
	}

	expectedPackages := []string{"vim", "git", "htop"}
	if len(packages) != len(expectedPackages) {
		t.Fatalf("Expected %d packages, got %d", len(expectedPackages), len(packages))
	}

	for i, pkg := range expectedPackages {
		if packages[i] != pkg {
			t.Errorf("Expected package[%d] = '%s', got '%s'", i, pkg, packages[i])
		}
	}

	updateCache, ok := action.Params["update_cache"].(bool)
	if !ok || !updateCache {
		t.Error("Expected update_cache to be true")
	}
}

// TestPackageInstallNoPackages verifies error handling when no packages specified
func TestPackageInstallNoPackages(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Package(pb, "install")
	if err == nil {
		t.Fatal("Expected error when installing with no packages")
	}

	if len(pb.Actions) != 0 {
		t.Errorf("Expected 0 actions after error, got %d", len(pb.Actions))
	}
}

// TestPackageRemove verifies that Package remove operation works correctly
func TestPackageRemove(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Package(pb, "remove", "apache2")
	if err != nil {
		t.Fatalf("Package remove failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "package.remove" {
		t.Errorf("Expected action type 'package.remove', got '%s'", action.Type)
	}

	packages, ok := action.Params["packages"].([]string)
	if !ok {
		t.Fatal("packages param is not a string slice")
	}

	if len(packages) != 1 || packages[0] != "apache2" {
		t.Errorf("Expected ['apache2'], got %v", packages)
	}
}

// TestPackageUpdate verifies that Package update operation works correctly
func TestPackageUpdate(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Package(pb, "update")
	if err != nil {
		t.Fatalf("Package update failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "package.update" {
		t.Errorf("Expected action type 'package.update', got '%s'", action.Type)
	}
}

// TestPackageUpgrade verifies that Package upgrade operation works correctly
func TestPackageUpgrade(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Package(pb, "upgrade")
	if err != nil {
		t.Fatalf("Package upgrade failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "package.upgrade" {
		t.Errorf("Expected action type 'package.upgrade', got '%s'", action.Type)
	}
}

// TestPackageInvalidOperation verifies error handling for invalid operations
func TestPackageInvalidOperation(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Package(pb, "invalidop", "somepackage")
	if err == nil {
		t.Fatal("Expected error for invalid operation")
	}

	if len(pb.Actions) != 0 {
		t.Errorf("Expected 0 actions after error, got %d", len(pb.Actions))
	}
}

// TestPackageMultipleOperations verifies multiple package operations in sequence
func TestPackageMultipleOperations(t *testing.T) {
	pb := playbook.New("test-playbook")

	// Update cache
	if err := Package(pb, "update"); err != nil {
		t.Fatalf("Package update failed: %v", err)
	}

	// Install packages
	if err := Package(pb, "install", "nginx", "vim"); err != nil {
		t.Fatalf("Package install failed: %v", err)
	}

	// Remove package
	if err := Package(pb, "remove", "apache2"); err != nil {
		t.Fatalf("Package remove failed: %v", err)
	}

	if len(pb.Actions) != 3 {
		t.Fatalf("Expected 3 actions, got %d", len(pb.Actions))
	}

	// Verify action sequence
	expectedTypes := []string{"package.update", "package.install", "package.remove"}
	for i, expectedType := range expectedTypes {
		if pb.Actions[i].Type != expectedType {
			t.Errorf("Action[%d]: expected type '%s', got '%s'",
				i, expectedType, pb.Actions[i].Type)
		}
	}
}

// TestPackageWithOptions verifies the extended options interface
func TestPackageWithOptions(t *testing.T) {
	pb := playbook.New("test-playbook")

	opts := &PackageOptions{
		UpdateCache:    false,
		AllowDowngrade: true,
		Force:          true,
	}

	err := PackageWithOptions(pb, "install", []string{"nginx"}, opts)
	if err != nil {
		t.Fatalf("PackageWithOptions failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]

	if updateCache, ok := action.Params["update_cache"].(bool); !ok || updateCache {
		t.Error("Expected update_cache to be false")
	}

	if allowDowngrade, ok := action.Params["allow_downgrade"].(bool); !ok || !allowDowngrade {
		t.Error("Expected allow_downgrade to be true")
	}

	if force, ok := action.Params["force"].(bool); !ok || !force {
		t.Error("Expected force to be true")
	}
}

// TestPackageActionIDs verifies that actions get sequential IDs
func TestPackageActionIDs(t *testing.T) {
	pb := playbook.New("test-playbook")

	Package(pb, "install", "vim")
	Package(pb, "install", "git")
	Package(pb, "install", "htop")

	expectedIDs := []string{"action-001", "action-002", "action-003"}
	for i, expectedID := range expectedIDs {
		if pb.Actions[i].ID != expectedID {
			t.Errorf("Action[%d]: expected ID '%s', got '%s'",
				i, expectedID, pb.Actions[i].ID)
		}
	}
}
