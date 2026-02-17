package playbook

import (
	"encoding/json"
	"fmt"
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
	nextID      int
}

// Action represents a single atomic operation to be executed by the agent.
type Action struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Params map[string]any `json:"params"`
}

// New creates a new Playbook with the given name.
func New(name string) *Playbook {
	return &Playbook{
		Version:     "1.0",
		Name:        name,
		Created:     time.Now().UTC(),
		Environment: make(map[string]string),
		Actions:     make([]Action, 0),
		nextID:      1,
	}
}

// SetEnv sets an environment variable that will be available to all actions.
func (pb *Playbook) SetEnv(key, value string) {
	pb.Environment[key] = value
}

// AddAction adds an action to the playbook and generates a unique ID for it.
func (pb *Playbook) AddAction(action Action) {
	action.ID = pb.generateActionID()
	pb.Actions = append(pb.Actions, action)
}

// generateActionID creates a unique sequential ID for actions.
func (pb *Playbook) generateActionID() string {
	id := fmt.Sprintf("action-%03d", pb.nextID)
	pb.nextID++
	return id
}

// ToJSON serializes the playbook to JSON format.
func (pb *Playbook) ToJSON() ([]byte, error) {
	return json.MarshalIndent(pb, "", "  ")
}

// // Execute packages the playbook and executes it on the remote host.
// // This is a placeholder for the actual implementation which would:
// //  1. Package the playbook into a tar.gz archive
// //  2. Sign the archive with the controller's private key
// //  3. Transfer the archive to the remote host via SSH
// //  4. Execute the agent via SSH with sudo privileges
// //  5. Monitor execution and return results
// func (pb *Playbook) Execute(host string) error {
// 	// Import the controller executor to handle the actual execution
// 	// For now, this is a placeholder that shows the playbook structure

// 	// Print the playbook JSON for demonstration
// 	jsonData, err := pb.ToJSON()
// 	if err != nil {
// 		return fmt.Errorf("failed to serialize playbook: %w", err)
// 	}

// 	fmt.Println(string(jsonData))

// 	exec, err := executor.New(&executor.Config{})
// 	if err != nil {
// 		return fmt.Errorf("failed to create executor: %w", err)
// 	}

// 	return exec.Execute(pb, host)

// 	return nil
// }

// // generateRandomID creates a random hex string for use in IDs.
// func generateRandomID(length int) (string, error) {
// 	bytes := make([]byte, length)
// 	if _, err := rand.Read(bytes); err != nil {
// 		return "", err
// 	}
// 	return hex.EncodeToString(bytes), nil
// }
