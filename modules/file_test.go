package modules

import (
	"testing"

	"github.com/typedduck/nestor/playbook"
)

// TestFileWithContent verifies creating a file with inline content
func TestFileWithContent(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := File(pb, "/etc/motd", Content("Welcome to the server\n"))
	if err != nil {
		t.Fatalf("File with content failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "file.content" {
		t.Errorf("Expected action type 'file.content', got '%s'", action.Type)
	}

	dest, ok := action.Params["destination"].(string)
	if !ok || dest != "/etc/motd" {
		t.Errorf("Expected destination '/etc/motd', got '%v'", dest)
	}

	content, ok := action.Params["content"].(string)
	if !ok || content != "Welcome to the server\n" {
		t.Errorf("Expected content 'Welcome to the server\\n', got '%v'", content)
	}
}

// TestFileWithTemplate verifies creating a file from a template
func TestFileWithTemplate(t *testing.T) {
	pb := playbook.New("test-playbook")

	vars := map[string]string{
		"DBHost": "db.example.com",
		"DBPort": "5432",
	}

	err := File(pb, "/etc/app/config.yml",
		FromTemplate("config.yml.tmpl"),
		TemplateVars(vars))
	if err != nil {
		t.Fatalf("File with template failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "file.template" {
		t.Errorf("Expected action type 'file.template', got '%s'", action.Type)
	}

	source, ok := action.Params["source"].(string)
	if !ok || source != "config.yml.tmpl" {
		t.Errorf("Expected source 'config.yml.tmpl', got '%v'", source)
	}

	variables, ok := action.Params["variables"].(map[string]string)
	if !ok {
		t.Fatal("variables param is not a map[string]string")
	}

	if variables["DBHost"] != "db.example.com" {
		t.Errorf("Expected DBHost 'db.example.com', got '%s'", variables["DBHost"])
	}
}

// TestFileUpload verifies uploading a local file
func TestFileUpload(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := File(pb, "/usr/local/bin/myapp",
		FromFile("./build/myapp"),
		Mode(0755))
	if err != nil {
		t.Fatalf("File upload failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "file.upload" {
		t.Errorf("Expected action type 'file.upload', got '%s'", action.Type)
	}

	source, ok := action.Params["source"].(string)
	if !ok || source != "./build/myapp" {
		t.Errorf("Expected source './build/myapp', got '%v'", source)
	}

	mode, ok := action.Params["mode"].(string)
	if !ok || mode != "0755" {
		t.Errorf("Expected mode '0755', got '%v'", mode)
	}
}

// TestFileWithOwnership verifies setting file owner and group
func TestFileWithOwnership(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := File(pb, "/etc/app/secret.conf",
		Content("api_key=secret123"),
		Owner("appuser", "appgroup"),
		Mode(0600))
	if err != nil {
		t.Fatalf("File with ownership failed: %v", err)
	}

	action := pb.Actions[0]

	owner, ok := action.Params["owner"].(string)
	if !ok || owner != "appuser" {
		t.Errorf("Expected owner 'appuser', got '%v'", owner)
	}

	group, ok := action.Params["group"].(string)
	if !ok || group != "appgroup" {
		t.Errorf("Expected group 'appgroup', got '%v'", group)
	}

	mode, ok := action.Params["mode"].(string)
	if !ok || mode != "0600" {
		t.Errorf("Expected mode '0600', got '%v'", mode)
	}
}

// TestFileNoSource verifies error when no source is specified
func TestFileNoSource(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := File(pb, "/etc/test.conf")
	if err == nil {
		t.Fatal("Expected error when no source specified")
	}

	if len(pb.Actions) != 0 {
		t.Errorf("Expected 0 actions after error, got %d", len(pb.Actions))
	}
}

// TestDirectory verifies directory creation
func TestDirectory(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Directory(pb, "/var/app/data",
		Owner("appuser", "appgroup"),
		Mode(0750))
	if err != nil {
		t.Fatalf("Directory creation failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "directory.create" {
		t.Errorf("Expected action type 'directory.create', got '%s'", action.Type)
	}

	path, ok := action.Params["path"].(string)
	if !ok || path != "/var/app/data" {
		t.Errorf("Expected path '/var/app/data', got '%v'", path)
	}

	mode, ok := action.Params["mode"].(string)
	if !ok || mode != "0750" {
		t.Errorf("Expected mode '0750', got '%v'", mode)
	}

	recursive, ok := action.Params["recursive"].(bool)
	if !ok || recursive {
		t.Error("Expected recursive to be false by default")
	}
}

// TestDirectoryRecursive verifies recursive directory creation
func TestDirectoryRecursive(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Directory(pb, "/var/app/data/logs",
		Recursive(true))
	if err != nil {
		t.Fatalf("Recursive directory creation failed: %v", err)
	}

	action := pb.Actions[0]

	recursive, ok := action.Params["recursive"].(bool)
	if !ok || !recursive {
		t.Error("Expected recursive to be true")
	}
}

// TestSymlink verifies symlink creation
func TestSymlink(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Symlink(pb,
		"/etc/nginx/sites-enabled/myapp",
		"/etc/nginx/sites-available/myapp")
	if err != nil {
		t.Fatalf("Symlink creation failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "file.symlink" {
		t.Errorf("Expected action type 'file.symlink', got '%s'", action.Type)
	}

	dest, ok := action.Params["destination"].(string)
	if !ok || dest != "/etc/nginx/sites-enabled/myapp" {
		t.Errorf("Expected destination '/etc/nginx/sites-enabled/myapp', got '%v'", dest)
	}

	source, ok := action.Params["source"].(string)
	if !ok || source != "/etc/nginx/sites-available/myapp" {
		t.Errorf("Expected source '/etc/nginx/sites-available/myapp', got '%v'", source)
	}
}

// TestRemove verifies file removal
func TestRemove(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Remove(pb, "/tmp/old-config.conf")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if len(pb.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(pb.Actions))
	}

	action := pb.Actions[0]
	if action.Type != "file.remove" {
		t.Errorf("Expected action type 'file.remove', got '%s'", action.Type)
	}

	path, ok := action.Params["path"].(string)
	if !ok || path != "/tmp/old-config.conf" {
		t.Errorf("Expected path '/tmp/old-config.conf', got '%v'", path)
	}

	recursive, ok := action.Params["recursive"].(bool)
	if !ok || recursive {
		t.Error("Expected recursive to be false by default")
	}
}

// TestRemoveRecursive verifies recursive directory removal
func TestRemoveRecursive(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := Remove(pb, "/var/app/old-version", Recursive(true))
	if err != nil {
		t.Fatalf("Recursive remove failed: %v", err)
	}

	action := pb.Actions[0]

	recursive, ok := action.Params["recursive"].(bool)
	if !ok || !recursive {
		t.Error("Expected recursive to be true")
	}
}

// TestMultipleFileOperations verifies multiple file operations in sequence
func TestMultipleFileOperations(t *testing.T) {
	pb := playbook.New("test-playbook")

	// Create directory
	Directory(pb, "/var/app", Owner("appuser", "appgroup"))

	// Upload file
	File(pb, "/var/app/myapp",
		FromFile("./build/myapp"),
		Mode(0755))

	// Create config from template
	File(pb, "/var/app/config.yml",
		FromTemplate("config.yml.tmpl"),
		TemplateVars(map[string]string{"Port": "8080"}))

	// Create symlink
	Symlink(pb, "/usr/local/bin/myapp", "/var/app/myapp")

	if len(pb.Actions) != 4 {
		t.Fatalf("Expected 4 actions, got %d", len(pb.Actions))
	}

	expectedTypes := []string{
		"directory.create",
		"file.upload",
		"file.template",
		"file.symlink",
	}

	for i, expectedType := range expectedTypes {
		if pb.Actions[i].Type != expectedType {
			t.Errorf("Action[%d]: expected type '%s', got '%s'",
				i, expectedType, pb.Actions[i].Type)
		}
	}
}

// TestFileDefaultPermissions verifies default permissions are not added unnecessarily
func TestFileDefaultPermissions(t *testing.T) {
	pb := playbook.New("test-playbook")

	// Create file without specifying mode (should use default 0644)
	err := File(pb, "/etc/test.conf", Content("test"))
	if err != nil {
		t.Fatalf("File creation failed: %v", err)
	}

	action := pb.Actions[0]

	// Default mode should not be added to params to keep JSON clean
	if _, exists := action.Params["mode"]; exists {
		t.Error("Expected default mode (0644) not to be added to params")
	}
}

// TestFileActionIDs verifies that actions get sequential IDs
func TestFileActionIDs(t *testing.T) {
	pb := playbook.New("test-playbook")

	File(pb, "/etc/file1", Content("content1"))
	Directory(pb, "/var/data")
	Symlink(pb, "/link", "/target")

	expectedIDs := []string{"action-001", "action-002", "action-003"}
	for i, expectedID := range expectedIDs {
		if pb.Actions[i].ID != expectedID {
			t.Errorf("Action[%d]: expected ID '%s', got '%s'",
				i, expectedID, pb.Actions[i].ID)
		}
	}
}

// TestTemplateWithoutVars verifies template can be used without variables
func TestTemplateWithoutVars(t *testing.T) {
	pb := playbook.New("test-playbook")

	err := File(pb, "/etc/config",
		FromTemplate("config.tmpl"))
	if err != nil {
		t.Fatalf("Template without vars failed: %v", err)
	}

	action := pb.Actions[0]

	// Should have nil/empty variables map
	vars := action.Params["variables"]
	if vars != nil {
		t.Errorf("Expected nil variables, got %v", vars)
	}
}
