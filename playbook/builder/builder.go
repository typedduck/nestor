package builder

import (
	"fmt"
	"time"

	"github.com/typedduck/nestor/playbook"
)

// Builder assembles a Playbook, assigning sequential action IDs.
type Builder struct {
	pb     *playbook.Playbook
	nextID int
}

// New creates a Builder for a playbook with the given name.
func New(name string) *Builder {
	return &Builder{
		pb: &playbook.Playbook{
			Version:     "1.0",
			Name:        name,
			Created:     time.Now().UTC(),
			Environment: make(map[string]string),
			Actions:     make([]playbook.Action, 0),
		},
		nextID: 1,
	}
}

// SetEnv sets an environment variable available to all actions.
func (b *Builder) SetEnv(key, value string) {
	b.pb.Environment[key] = value
}

// AddAction appends an action to the playbook, assigning it a unique ID.
func (b *Builder) AddAction(action playbook.Action) {
	action.ID = fmt.Sprintf("action-%03d", b.nextID)
	b.nextID++
	b.pb.Actions = append(b.pb.Actions, action)
}

// Playbook returns the assembled Playbook.
func (b *Builder) Playbook() *playbook.Playbook {
	return b.pb
}
