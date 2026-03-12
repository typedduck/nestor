package modules

import (
	"testing"

	"github.com/typedduck/nestor/playbook/builder"
)

func TestServiceStart(t *testing.T) {
	b := builder.New("test-playbook")

	err := Service(b, "nginx", "start")
	if err != nil {
		t.Fatalf("Service start failed: %v", err)
	}

	actions := b.Playbook().Actions
	if len(actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Type != "service.start" {
		t.Errorf("Expected type 'service.start', got '%s'", action.Type)
	}
	if name, ok := action.Params["name"].(string); !ok || name != "nginx" {
		t.Errorf("Expected name='nginx', got %v", action.Params["name"])
	}
}

func TestServiceStop(t *testing.T) {
	b := builder.New("test-playbook")

	err := Service(b, "nginx", "stop")
	if err != nil {
		t.Fatalf("Service stop failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if action.Type != "service.stop" {
		t.Errorf("Expected type 'service.stop', got '%s'", action.Type)
	}
	if name, ok := action.Params["name"].(string); !ok || name != "nginx" {
		t.Errorf("Expected name='nginx', got %v", action.Params["name"])
	}
}

func TestServiceRestart(t *testing.T) {
	b := builder.New("test-playbook")

	err := Service(b, "nginx", "restart")
	if err != nil {
		t.Fatalf("Service restart failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if action.Type != "service.restart" {
		t.Errorf("Expected type 'service.restart', got '%s'", action.Type)
	}
}

func TestServiceReload(t *testing.T) {
	b := builder.New("test-playbook")

	err := Service(b, "nginx", "reload")
	if err != nil {
		t.Fatalf("Service reload failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if action.Type != "service.reload" {
		t.Errorf("Expected type 'service.reload', got '%s'", action.Type)
	}
}

func TestServiceSequentialIDs(t *testing.T) {
	b := builder.New("test-playbook")

	Service(b, "nginx", "start")
	Service(b, "redis", "stop")

	actions := b.Playbook().Actions
	if len(actions) != 2 {
		t.Fatalf("Expected 2 actions, got %d", len(actions))
	}

	expectedIDs := []string{"action-001", "action-002"}
	for i, expectedID := range expectedIDs {
		if actions[i].ID != expectedID {
			t.Errorf("Action[%d]: expected ID '%s', got '%s'", i, expectedID, actions[i].ID)
		}
	}
}

func TestServiceInvalidOperation(t *testing.T) {
	b := builder.New("test-playbook")

	err := Service(b, "nginx", "enable")
	if err == nil {
		t.Fatal("Expected error for invalid operation")
	}

	if len(b.Playbook().Actions) != 0 {
		t.Errorf("Expected 0 actions after error, got %d", len(b.Playbook().Actions))
	}
}

func TestServiceEmptyName(t *testing.T) {
	b := builder.New("test-playbook")

	// Empty name is valid at the controller level; validation is agent-side.
	err := Service(b, "", "start")
	if err != nil {
		t.Fatalf("Expected no error for empty name (agent validates), got: %v", err)
	}

	if len(b.Playbook().Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(b.Playbook().Actions))
	}
}

func TestService_RunAs(t *testing.T) {
	b := builder.New("test-playbook")

	err := Service(b, "myapp", "restart", RunAs("alice"))
	if err != nil {
		t.Fatalf("Service with RunAs failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if action.Type != "service.restart" {
		t.Errorf("Expected type 'service.restart', got '%s'", action.Type)
	}
	if runAs, ok := action.Params["run_as"].(string); !ok || runAs != "alice" {
		t.Errorf("Expected run_as='alice', got %v", action.Params["run_as"])
	}
}

func TestService_RunAs_NotSetByDefault(t *testing.T) {
	b := builder.New("test-playbook")

	Service(b, "nginx", "reload")

	action := b.Playbook().Actions[0]
	if _, ok := action.Params["run_as"]; ok {
		t.Error("run_as param should not be present when RunAs option is not used")
	}
}
