package yamlloader

import (
	"fmt"

	yaml3 "gopkg.in/yaml.v3"
)

// Playbook is the top-level structure of a YAML playbook file.
type Playbook struct {
	Name        string            `yaml:"name"`
	Environment map[string]string `yaml:"environment"`
	Vars        map[string]string `yaml:"vars"`
	Actions     []RawAction       `yaml:"actions"`
}

// RawAction captures one action entry as (kind, raw node) by custom unmarshaling.
// Each action is a YAML mapping with exactly one key.
type RawAction struct {
	Kind  string
	Value *yaml3.Node
}

// UnmarshalYAML implements yaml.Unmarshaler for RawAction.
// It expects a mapping node with exactly one key-value pair.
func (a *RawAction) UnmarshalYAML(value *yaml3.Node) error {
	if value.Kind != yaml3.MappingNode || len(value.Content) != 2 {
		return fmt.Errorf("action must have exactly one key, got %d fields", len(value.Content)/2)
	}
	a.Kind = value.Content[0].Value
	a.Value = value.Content[1]
	return nil
}

// PackageAction holds structured package action fields.
type PackageAction struct {
	Install []string `yaml:"install"`
	Remove  []string `yaml:"remove"`
}

// FileAction holds structured file action fields.
type FileAction struct {
	Path     string            `yaml:"path"`
	Content  string            `yaml:"content"`
	Template string            `yaml:"template"`
	Upload   string            `yaml:"upload"`
	Vars     map[string]string `yaml:"vars"`
	Owner    string            `yaml:"owner"`
	Group    string            `yaml:"group"`
	Mode     string            `yaml:"mode"`
}

// DirectoryAction holds structured directory action fields.
type DirectoryAction struct {
	Path      string `yaml:"path"`
	Owner     string `yaml:"owner"`
	Group     string `yaml:"group"`
	Mode      string `yaml:"mode"`
	Recursive bool   `yaml:"recursive"`
}

// SymlinkAction holds structured symlink action fields.
type SymlinkAction struct {
	Dest   string `yaml:"dest"`
	Target string `yaml:"target"`
}

// RemoveAction holds structured remove action fields.
type RemoveAction struct {
	Path      string `yaml:"path"`
	Recursive bool   `yaml:"recursive"`
}

// CommandAction holds structured command action fields.
type CommandAction struct {
	Run     string   `yaml:"run"`
	Creates string   `yaml:"creates"`
	Env     []string `yaml:"env"`
	Chdir   string   `yaml:"chdir"`
}

// ScriptAction holds structured script action fields.
type ScriptAction struct {
	Source  string   `yaml:"source"`
	Args    []string `yaml:"args"`
	Creates string   `yaml:"creates"`
	Env     []string `yaml:"env"`
	Chdir   string   `yaml:"chdir"`
}

// ServiceAction holds structured service action fields.
type ServiceAction struct {
	Name   string `yaml:"name"`
	Action string `yaml:"action"`
}
