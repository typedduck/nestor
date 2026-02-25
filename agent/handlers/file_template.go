package handlers

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/typedduck/nestor/agent/executor"
	"github.com/typedduck/nestor/playbook"
)

// FileTemplateHandler renders a Go text/template from the playbook bundle.
type FileTemplateHandler struct{}

func NewFileTemplateHandler() *FileTemplateHandler { return &FileTemplateHandler{} }

func (h *FileTemplateHandler) Execute(action playbook.Action,
	context *executor.ExecutionContext) executor.ActionResult {

	source := getStringParam(action.Params, "source", "")
	if source == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: source",
			Error:   "source parameter not found or empty",
		}
	}

	destination := getStringParam(action.Params, "destination", "")
	if destination == "" {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Missing required parameter: destination",
			Error:   "destination parameter not found or empty",
		}
	}

	owner := getStringParam(action.Params, "owner", "")
	group := getStringParam(action.Params, "group", "")
	mode := getStringParam(action.Params, "mode", "0644")

	// Extract optional variables map
	var vars map[string]any
	if v, ok := action.Params["variables"]; ok {
		if m, ok := v.(map[string]any); ok {
			vars = m
		}
	}

	fs := context.FS

	// Read template source from playbook bundle
	sourcePath := filepath.Join(context.PlaybookPath, source)
	tmplData, err := fs.ReadFile(sourcePath)
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to read template source %s", source),
			Error:   err.Error(),
		}
	}

	if context.DryRun {
		return executor.ActionResult{
			Status:  "success",
			Changed: true,
			Message: fmt.Sprintf("Would render template %s to %s", source, destination),
		}
	}

	// Parse template with missingkey=error so undefined variables surface clearly
	tmpl, err := template.New(filepath.Base(source)).
		Option("missingkey=error").
		Parse(string(tmplData))
	if err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to parse template %s", source),
			Error:   err.Error(),
		}
	}

	// Render template
	var buf strings.Builder
	if err := tmpl.Execute(&buf, vars); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to render template %s", source),
			Error:   err.Error(),
		}
	}
	rendered := buf.String()

	// Idempotency check: compare rendered content with existing destination
	existing, err := fs.ReadFile(destination)
	if err == nil && bytes.Equal(existing, []byte(rendered)) {
		if err := setPermissions(fs, destination, owner, group, mode); err != nil {
			return executor.ActionResult{
				Status:  "failed",
				Changed: false,
				Message: "Failed to set permissions",
				Error:   err.Error(),
			}
		}
		return executor.ActionResult{
			Status:  "success",
			Changed: false,
			Message: fmt.Sprintf("File %s already matches rendered template", destination),
		}
	} else if err != nil && !os.IsNotExist(err) {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to read destination file %s", destination),
			Error:   err.Error(),
		}
	}

	// Create parent directory if needed
	if err := fs.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: "Failed to create parent directory",
			Error:   err.Error(),
		}
	}

	// Write rendered content
	if err := fs.WriteFile(destination, []byte(rendered), 0644); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: false,
			Message: fmt.Sprintf("Failed to write file %s", destination),
			Error:   err.Error(),
		}
	}

	// Set permissions
	if err := setPermissions(fs, destination, owner, group, mode); err != nil {
		return executor.ActionResult{
			Status:  "failed",
			Changed: true,
			Message: "File rendered but failed to set permissions",
			Error:   err.Error(),
		}
	}

	return executor.ActionResult{
		Status:  "success",
		Changed: true,
		Message: fmt.Sprintf("Rendered template %s to %s (%d bytes)", source, destination, len(rendered)),
	}
}
