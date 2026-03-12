// Package yamlloader parses YAML playbook files and produces a populated
// builder.Builder. The package name avoids collision with gopkg.in/yaml.v3.
package yamlloader

import (
	"bytes"
	"fmt"
	"maps"
	"strconv"
	"strings"

	yaml3 "gopkg.in/yaml.v3"

	"github.com/typedduck/nestor/playbook"
	"github.com/typedduck/nestor/playbook/builder"
)

// phaseAllowedKinds lists the action kinds permitted in controller phases (pre/post).
var phaseAllowedKinds = map[string]bool{
	"command": true,
	"script":  true,
	"file":    true,
}

// validatePhaseActions returns an error if any action in the slice is not
// allowed in a controller phase. phase is "pre" or "post" for error messages.
func validatePhaseActions(phase string, actions []RawAction) error {
	for _, raw := range actions {
		if !phaseAllowedKinds[raw.Kind] {
			return fmt.Errorf("%s: action %q is not allowed in controller phases (allowed: command, script, file)", phase, raw.Kind)
		}
	}
	return nil
}

// buildPhase builds a *playbook.Playbook from a slice of RawActions.
// env and name are inherited from the top-level playbook definition.
func buildPhase(name string, env map[string]string, actions []RawAction) (*playbook.Playbook, error) {
	b := builder.New(name)
	for k, v := range env {
		b.SetEnv(k, v)
	}
	for i, raw := range actions {
		if err := dispatchAction(b, raw); err != nil {
			return nil, fmt.Errorf("action %d (%s): %w", i+1, raw.Kind, err)
		}
	}
	return b.Playbook(), nil
}

// Load parses YAML playbook bytes, applies variable substitution, and returns
// a *LoadResult with the three execution phases (pre, remote, post).
//
// The vars parameter contains overrides (typically from --var CLI flags) that
// take precedence over the vars: section in the YAML file.
// Pre and Post are nil when the respective YAML sections are absent.
func Load(data []byte, vars map[string]string) (*LoadResult, error) {
	// First pass: decode just enough to get the vars section.
	var raw Playbook
	if err := yaml3.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Merge vars: YAML vars are the baseline; CLI vars override.
	merged := make(map[string]string, len(raw.Vars)+len(vars))
	maps.Copy(merged, raw.Vars)
	maps.Copy(merged, vars)

	// Apply variable substitution to the raw bytes, then re-parse.
	substituted := applyVars(data, merged)

	var pb Playbook
	if err := yaml3.Unmarshal(substituted, &pb); err != nil {
		return nil, fmt.Errorf("failed to parse YAML after variable substitution: %w", err)
	}

	name := pb.Name
	if name == "" {
		name = "unnamed"
	}

	if err := validatePhaseActions("pre", pb.Pre); err != nil {
		return nil, err
	}
	if err := validatePhaseActions("post", pb.Post); err != nil {
		return nil, err
	}

	var pre *playbook.Playbook
	if len(pb.Pre) > 0 {
		var err error
		if pre, err = buildPhase(name, pb.Environment, pb.Pre); err != nil {
			return nil, fmt.Errorf("pre: %w", err)
		}
	}

	remote, err := buildPhase(name, pb.Environment, pb.Actions)
	if err != nil {
		return nil, err
	}

	var post *playbook.Playbook
	if len(pb.Post) > 0 {
		var err error
		if post, err = buildPhase(name, pb.Environment, pb.Post); err != nil {
			return nil, fmt.Errorf("post: %w", err)
		}
	}

	return &LoadResult{Pre: pre, Remote: remote, Post: post}, nil
}

// applyVars replaces ${key} references in data with values from vars.
func applyVars(data []byte, vars map[string]string) []byte {
	result := data
	for k, v := range vars {
		placeholder := "${" + k + "}"
		result = bytes.ReplaceAll(result, []byte(placeholder), []byte(v))
	}
	return result
}

// parseMode parses a mode string such as "0755" or "755" into a uint32.
// A leading "0" is stripped; the remaining digits are parsed as octal.
func parseMode(s string) (uint32, error) {
	if s == "" {
		return 0, fmt.Errorf("mode string is empty")
	}
	trimmed := strings.TrimPrefix(s, "0")
	if trimmed == "" {
		trimmed = "0"
	}
	v, err := strconv.ParseUint(trimmed, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode %q: %w", s, err)
	}
	return uint32(v), nil
}

// dispatchAction routes a RawAction to the appropriate load* function.
func dispatchAction(b *builder.Builder, raw RawAction) error {
	switch raw.Kind {
	case "package":
		return loadPackage(b, raw.Value)
	case "file":
		return loadFile(b, raw.Value)
	case "directory":
		return loadDirectory(b, raw.Value)
	case "symlink":
		return loadSymlink(b, raw.Value)
	case "remove":
		return loadRemove(b, raw.Value)
	case "command":
		return loadCommand(b, raw.Value)
	case "script":
		return loadScript(b, raw.Value)
	case "service":
		return loadService(b, raw.Value)
	default:
		return fmt.Errorf("unknown action kind %q", raw.Kind)
	}
}

// loadPackage handles both short forms ("update", "upgrade") and long forms
// with install/remove package lists.
func loadPackage(b *builder.Builder, node *yaml3.Node) error {
	// Short form: - package: update  or  - package: upgrade
	if node.Kind == yaml3.ScalarNode {
		switch node.Value {
		case "update":
			b.AddAction(playbook.Action{
				Type:   "package.update",
				Params: map[string]any{},
			})
			return nil
		case "upgrade":
			b.AddAction(playbook.Action{
				Type:   "package.upgrade",
				Params: map[string]any{},
			})
			return nil
		default:
			return fmt.Errorf("unknown package shorthand %q (valid: update, upgrade)", node.Value)
		}
	}

	var pkg PackageAction
	if err := node.Decode(&pkg); err != nil {
		return fmt.Errorf("failed to decode package action: %w", err)
	}

	if len(pkg.Install) > 0 {
		b.AddAction(playbook.Action{
			Type: "package.install",
			Params: map[string]any{
				"packages":     pkg.Install,
				"update_cache": true,
			},
		})
	}
	if len(pkg.Remove) > 0 {
		b.AddAction(playbook.Action{
			Type: "package.remove",
			Params: map[string]any{
				"packages": pkg.Remove,
			},
		})
	}
	if len(pkg.Install) == 0 && len(pkg.Remove) == 0 {
		return fmt.Errorf("package action must specify install or remove")
	}
	return nil
}

// loadFile handles file.content, file.template, and file.upload actions.
func loadFile(b *builder.Builder, node *yaml3.Node) error {
	var f FileAction
	if err := node.Decode(&f); err != nil {
		return fmt.Errorf("failed to decode file action: %w", err)
	}

	if f.Path == "" {
		return fmt.Errorf("file action requires path")
	}

	// Determine source type (mutually exclusive).
	sources := 0
	if f.Content != "" {
		sources++
	}
	if f.Template != "" {
		sources++
	}
	if f.Upload != "" {
		sources++
	}
	if sources == 0 {
		return fmt.Errorf("file action requires one of: content, template, upload")
	}
	if sources > 1 {
		return fmt.Errorf("file action must specify exactly one of: content, template, upload")
	}

	var action playbook.Action

	switch {
	case f.Content != "":
		action = playbook.Action{
			Type: "file.content",
			Params: map[string]any{
				"destination": f.Path,
				"content":     f.Content,
			},
		}

	case f.Template != "":
		params := map[string]any{
			"source":      f.Template,
			"destination": f.Path,
		}
		if len(f.Vars) > 0 {
			params["variables"] = f.Vars
		}
		action = playbook.Action{
			Type:   "file.template",
			Params: params,
		}

	case f.Upload != "":
		action = playbook.Action{
			Type: "file.upload",
			Params: map[string]any{
				"source":      f.Upload,
				"destination": f.Path,
			},
		}
	}

	if f.Owner != "" {
		action.Params["owner"] = f.Owner
	}
	if f.Group != "" {
		action.Params["group"] = f.Group
	}
	if f.Mode != "" {
		m, err := parseMode(f.Mode)
		if err != nil {
			return err
		}
		action.Params["mode"] = fmt.Sprintf("0%o", m)
	}

	b.AddAction(action)
	return nil
}

// loadDirectory handles directory.create actions.
func loadDirectory(b *builder.Builder, node *yaml3.Node) error {
	var d DirectoryAction
	if err := node.Decode(&d); err != nil {
		return fmt.Errorf("failed to decode directory action: %w", err)
	}

	if d.Path == "" {
		return fmt.Errorf("directory action requires path")
	}

	modeStr := "0755"
	if d.Mode != "" {
		m, err := parseMode(d.Mode)
		if err != nil {
			return err
		}
		modeStr = fmt.Sprintf("0%o", m)
	}

	params := map[string]any{
		"path":      d.Path,
		"mode":      modeStr,
		"recursive": d.Recursive,
	}
	if d.Owner != "" {
		params["owner"] = d.Owner
	}
	if d.Group != "" {
		params["group"] = d.Group
	}

	b.AddAction(playbook.Action{
		Type:   "directory.create",
		Params: params,
	})
	return nil
}

// loadSymlink handles file.symlink actions.
func loadSymlink(b *builder.Builder, node *yaml3.Node) error {
	var s SymlinkAction
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("failed to decode symlink action: %w", err)
	}

	if s.Dest == "" {
		return fmt.Errorf("symlink action requires dest")
	}
	if s.Target == "" {
		return fmt.Errorf("symlink action requires target")
	}

	b.AddAction(playbook.Action{
		Type: "file.symlink",
		Params: map[string]any{
			"destination": s.Dest,
			"source":      s.Target,
		},
	})
	return nil
}

// loadRemove handles file.remove actions.
func loadRemove(b *builder.Builder, node *yaml3.Node) error {
	var r RemoveAction
	if err := node.Decode(&r); err != nil {
		return fmt.Errorf("failed to decode remove action: %w", err)
	}

	if r.Path == "" {
		return fmt.Errorf("remove action requires path")
	}

	b.AddAction(playbook.Action{
		Type: "file.remove",
		Params: map[string]any{
			"path":      r.Path,
			"recursive": r.Recursive,
		},
	})
	return nil
}

// loadCommand handles command.execute actions.
// Supports short form (- command: echo hello) and long form.
func loadCommand(b *builder.Builder, node *yaml3.Node) error {
	// Short form: - command: echo hello
	if node.Kind == yaml3.ScalarNode {
		if node.Value == "" {
			return fmt.Errorf("command must not be empty")
		}
		b.AddAction(playbook.Action{
			Type:   "command.execute",
			Params: map[string]any{"command": node.Value},
		})
		return nil
	}

	var c CommandAction
	if err := node.Decode(&c); err != nil {
		return fmt.Errorf("failed to decode command action: %w", err)
	}

	if c.Run == "" {
		return fmt.Errorf("command action requires run")
	}

	params := map[string]any{"command": c.Run}
	if c.Creates != "" {
		params["creates"] = c.Creates
	}
	if len(c.Env) > 0 {
		params["env"] = c.Env
	}
	if c.Chdir != "" {
		params["chdir"] = c.Chdir
	}

	b.AddAction(playbook.Action{
		Type:   "command.execute",
		Params: params,
	})
	return nil
}

// loadScript handles script.execute actions.
func loadScript(b *builder.Builder, node *yaml3.Node) error {
	var s ScriptAction
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("failed to decode script action: %w", err)
	}

	if s.Source == "" {
		return fmt.Errorf("script action requires source")
	}

	params := map[string]any{"source": s.Source}
	if len(s.Args) > 0 {
		params["args"] = s.Args
	}
	if s.Creates != "" {
		params["creates"] = s.Creates
	}
	if len(s.Env) > 0 {
		params["env"] = s.Env
	}
	if s.Chdir != "" {
		params["chdir"] = s.Chdir
	}

	b.AddAction(playbook.Action{
		Type:   "script.execute",
		Params: params,
	})
	return nil
}

// loadService handles service.{start,stop,restart,reload} actions.
func loadService(b *builder.Builder, node *yaml3.Node) error {
	var s ServiceAction
	if err := node.Decode(&s); err != nil {
		return fmt.Errorf("failed to decode service action: %w", err)
	}

	if s.Name == "" {
		return fmt.Errorf("service action requires name")
	}
	if s.Action == "" {
		return fmt.Errorf("service action requires action")
	}

	switch s.Action {
	case "start", "stop", "restart", "reload":
	default:
		return fmt.Errorf("unknown service action %q (valid: start, stop, restart, reload)", s.Action)
	}

	params := map[string]any{"name": s.Name}
	if s.RunAs != "" {
		params["run_as"] = s.RunAs
	}

	b.AddAction(playbook.Action{
		Type:   "service." + s.Action,
		Params: params,
	})
	return nil
}
