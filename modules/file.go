package modules

import (
	"fmt"

	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/playbook/builder"
)

// FileOption is a functional option for configuring file operations.
type FileOption func(*fileConfig)

// fileConfig holds the configuration for a file operation.
type fileConfig struct {
	// Source configuration
	sourceType   string            // "content", "template", "file"
	content      string            // inline content
	sourcePath   string            // path to source file or template
	templateVars map[string]string // variables for template rendering

	// Permissions
	owner string
	group string
	mode  uint32

	// Directory options
	recursive bool
}

// File creates or updates a file on the remote system.
//
// The File function uses functional options to configure the file operation.
// You must specify exactly one source option (Content, FromTemplate, or FromFile).
//
// Examples:
//
//	// Create file with inline content
//	modules.File(b, "/etc/motd",
//	    modules.Content("Welcome to the server\n"))
//
//	// Create file from template with variables
//	modules.File(b, "/etc/app/config.yml",
//	    modules.FromTemplate("config.yml.tmpl"),
//	    modules.TemplateVars(map[string]string{
//	        "DBHost": "db.example.com",
//	        "DBPort": "5432",
//	    }))
//
//	// Upload local file
//	modules.File(b, "/usr/local/bin/myapp",
//	    modules.FromFile("./build/myapp"),
//	    modules.Mode(0755))
//
//	// Set ownership and permissions
//	modules.File(b, "/etc/app/secret.conf",
//	    modules.Content("api_key=secret123"),
//	    modules.Owner("appuser", "appgroup"),
//	    modules.Mode(0600))
func File(b *builder.Builder, destination string, opts ...FileOption) error {
	cfg := &fileConfig{
		mode: 0644, // default mode
	}

	// Apply all options
	for _, opt := range opts {
		opt(cfg)
	}

	// Validate that a source was specified
	if cfg.sourceType == "" {
		return fmt.Errorf("file operation requires a source (Content, FromTemplate, or FromFile)")
	}

	// Build the appropriate action based on source type
	var action playbook.Action

	switch cfg.sourceType {
	case "content":
		action = playbook.Action{
			Type: "file.content",
			Params: map[string]any{
				"destination": destination,
				"content":     cfg.content,
			},
		}

	case "template":
		params := map[string]interface{}{
			"source":      cfg.sourcePath,
			"destination": destination,
		}
		if cfg.templateVars != nil {
			params["variables"] = cfg.templateVars
		}
		action = playbook.Action{
			Type:   "file.template",
			Params: params,
		}

	case "file":
		action = playbook.Action{
			Type: "file.upload",
			Params: map[string]any{
				"source":      cfg.sourcePath,
				"destination": destination,
			},
		}

	default:
		return fmt.Errorf("unknown source type: %s", cfg.sourceType)
	}

	// Add common parameters if specified
	if cfg.owner != "" {
		action.Params["owner"] = cfg.owner
	}
	if cfg.group != "" {
		action.Params["group"] = cfg.group
	}
	if cfg.mode != 0644 { // Only add if non-default
		action.Params["mode"] = fmt.Sprintf("0%o", cfg.mode)
	}

	b.AddAction(action)
	return nil
}

// Content specifies that the file should be created with inline content.
func Content(content string) FileOption {
	return func(cfg *fileConfig) {
		cfg.sourceType = "content"
		cfg.content = content
	}
}

// FromTemplate specifies that the file should be created from a template.
// The template path is relative to the playbook's upload directory.
func FromTemplate(templatePath string) FileOption {
	return func(cfg *fileConfig) {
		cfg.sourceType = "template"
		cfg.sourcePath = templatePath
	}
}

// FromFile specifies that the file should be uploaded from a local file.
// The file will be included in the playbook archive.
func FromFile(localPath string) FileOption {
	return func(cfg *fileConfig) {
		cfg.sourceType = "file"
		cfg.sourcePath = localPath
	}
}

// TemplateVars specifies variables to be used in template rendering.
// Only valid when used with FromTemplate.
func TemplateVars(vars map[string]string) FileOption {
	return func(cfg *fileConfig) {
		cfg.templateVars = vars
	}
}

// Owner specifies the owner and group for the file.
func Owner(owner, group string) FileOption {
	return func(cfg *fileConfig) {
		cfg.owner = owner
		cfg.group = group
	}
}

// Mode specifies the file permissions in octal format.
//
// Example:
//
//	modules.Mode(0755) // rwxr-xr-x
//	modules.Mode(0644) // rw-r--r--
//	modules.Mode(0600) // rw-------
func Mode(mode uint32) FileOption {
	return func(cfg *fileConfig) {
		cfg.mode = mode
	}
}

// Directory creates a directory on the remote system.
//
// Examples:
//
//	// Create simple directory
//	modules.Directory(b, "/var/app/data")
//
//	// Create directory with ownership and permissions
//	modules.Directory(b, "/var/app/data",
//	    modules.Owner("appuser", "appgroup"),
//	    modules.Mode(0750))
//
//	// Create directory recursively (like mkdir -p)
//	modules.Directory(b, "/var/app/data/logs",
//	    modules.Recursive(true))
func Directory(b *builder.Builder, path string, opts ...FileOption) error {
	cfg := &fileConfig{
		mode:      0755, // default directory mode
		recursive: false,
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	action := playbook.Action{
		Type: "directory.create",
		Params: map[string]interface{}{
			"path":      path,
			"mode":      fmt.Sprintf("0%o", cfg.mode),
			"recursive": cfg.recursive,
		},
	}

	if cfg.owner != "" {
		action.Params["owner"] = cfg.owner
	}
	if cfg.group != "" {
		action.Params["group"] = cfg.group
	}

	b.AddAction(action)
	return nil
}

// Recursive specifies that directory creation should be recursive (like mkdir -p).
// Only valid when used with Directory.
func Recursive(recursive bool) FileOption {
	return func(cfg *fileConfig) {
		cfg.recursive = recursive
	}
}

// Symlink creates a symbolic link on the remote system.
//
// Parameters:
//   - destination: the path where the symlink will be created
//   - target: the path the symlink will point to
//
// Example:
//
//	// Enable nginx site by creating symlink
//	modules.Symlink(b,
//	    "/etc/nginx/sites-enabled/myapp",
//	    "/etc/nginx/sites-available/myapp")
func Symlink(b *builder.Builder, destination, target string) error {
	action := playbook.Action{
		Type: "file.symlink",
		Params: map[string]interface{}{
			"destination": destination,
			"source":      target,
		},
	}

	b.AddAction(action)
	return nil
}

// Remove removes a file or directory from the remote system.
//
// Examples:
//
//	// Remove a file
//	modules.Remove(b, "/tmp/old-config.conf")
//
//	// Remove a directory recursively
//	modules.Remove(b, "/var/app/old-version",
//	    modules.Recursive(true))
func Remove(b *builder.Builder, path string, opts ...FileOption) error {
	cfg := &fileConfig{
		recursive: false,
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	action := playbook.Action{
		Type: "file.remove",
		Params: map[string]any{
			"path":      path,
			"recursive": cfg.recursive,
		},
	}

	b.AddAction(action)
	return nil
}
