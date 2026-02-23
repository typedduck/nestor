package playbook

import (
	"encoding/json"
	"time"
)

// Playbook represents a collection of actions to be executed on a remote system.
type Playbook struct {
	Version     string            `json:"version"`
	Name        string            `json:"name"`
	Created     time.Time         `json:"created"`
	Controller  string            `json:"controller"`
	Environment map[string]string `json:"environment"`
	Actions     []Action          `json:"actions"`
}

// Action represents a single atomic operation to be executed by the agent.
type Action struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

// ToJSON serializes the playbook to JSON format.
func (pb *Playbook) ToJSON() ([]byte, error) {
	return json.MarshalIndent(pb, "", "  ")
}
