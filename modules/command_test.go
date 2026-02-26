package modules

import (
	"testing"

	"github.com/typedduck/nestor/playbook/builder"
)

// --- Command ---

func TestCommand_Simple(t *testing.T) {
	b := builder.New("test-playbook")

	err := Command(b, "echo hello")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	actions := b.Playbook().Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Type != "command.execute" {
		t.Errorf("expected type 'command.execute', got '%s'", action.Type)
	}
	if cmd, ok := action.Params["command"].(string); !ok || cmd != "echo hello" {
		t.Errorf("expected command='echo hello', got %v", action.Params["command"])
	}
}

func TestCommand_EmptyCommand(t *testing.T) {
	b := builder.New("test-playbook")

	err := Command(b, "")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if len(b.Playbook().Actions) != 0 {
		t.Errorf("expected 0 actions after error, got %d", len(b.Playbook().Actions))
	}
}

func TestCommand_WithCreates(t *testing.T) {
	b := builder.New("test-playbook")

	err := Command(b, "useradd -m deploy", Creates("/home/deploy"))
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if creates, ok := action.Params["creates"].(string); !ok || creates != "/home/deploy" {
		t.Errorf("expected creates='/home/deploy', got %v", action.Params["creates"])
	}
}

func TestCommand_WithEnv(t *testing.T) {
	b := builder.New("test-playbook")

	err := Command(b, "make install", CommandEnv("DESTDIR=/opt", "PREFIX=/usr/local"))
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	env, ok := action.Params["env"].([]string)
	if !ok {
		t.Fatal("expected env param to be []string")
	}
	if len(env) != 2 || env[0] != "DESTDIR=/opt" || env[1] != "PREFIX=/usr/local" {
		t.Errorf("unexpected env: %v", env)
	}
}

func TestCommand_WithChdir(t *testing.T) {
	b := builder.New("test-playbook")

	err := Command(b, "make install", Chdir("/opt/src/app"))
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if chdir, ok := action.Params["chdir"].(string); !ok || chdir != "/opt/src/app" {
		t.Errorf("expected chdir='/opt/src/app', got %v", action.Params["chdir"])
	}
}

func TestCommand_NoCreatesParam(t *testing.T) {
	b := builder.New("test-playbook")

	Command(b, "echo hello")

	action := b.Playbook().Actions[0]
	if _, ok := action.Params["creates"]; ok {
		t.Error("creates param should not be present when not set")
	}
}

func TestCommand_ActionIDs(t *testing.T) {
	b := builder.New("test-playbook")

	Command(b, "echo one")
	Command(b, "echo two")

	actions := b.Playbook().Actions
	if actions[0].ID != "action-001" || actions[1].ID != "action-002" {
		t.Errorf("unexpected IDs: %s, %s", actions[0].ID, actions[1].ID)
	}
}

// --- Script ---

func TestScript_Simple(t *testing.T) {
	b := builder.New("test-playbook")

	err := Script(b, "scripts/setup.sh")
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	actions := b.Playbook().Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]
	if action.Type != "script.execute" {
		t.Errorf("expected type 'script.execute', got '%s'", action.Type)
	}
	if src, ok := action.Params["source"].(string); !ok || src != "scripts/setup.sh" {
		t.Errorf("expected source='scripts/setup.sh', got %v", action.Params["source"])
	}
}

func TestScript_EmptySource(t *testing.T) {
	b := builder.New("test-playbook")

	err := Script(b, "")
	if err == nil {
		t.Fatal("expected error for empty source")
	}
	if len(b.Playbook().Actions) != 0 {
		t.Errorf("expected 0 actions after error, got %d", len(b.Playbook().Actions))
	}
}

func TestScript_WithArgs(t *testing.T) {
	b := builder.New("test-playbook")

	err := Script(b, "scripts/setup.sh", ScriptArgs("--verbose", "--force"))
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	args, ok := action.Params["args"].([]string)
	if !ok {
		t.Fatal("expected args param to be []string")
	}
	if len(args) != 2 || args[0] != "--verbose" || args[1] != "--force" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestScript_WithCreates(t *testing.T) {
	b := builder.New("test-playbook")

	err := Script(b, "scripts/setup.sh", ScriptCreates("/etc/app/setup.done"))
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	if creates, ok := action.Params["creates"].(string); !ok || creates != "/etc/app/setup.done" {
		t.Errorf("expected creates='/etc/app/setup.done', got %v", action.Params["creates"])
	}
}

func TestScript_WithEnvAndChdir(t *testing.T) {
	b := builder.New("test-playbook")

	err := Script(b, "scripts/build.sh",
		ScriptEnv("BUILD_TYPE=release"),
		ScriptChdir("/opt/src"))
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	action := b.Playbook().Actions[0]
	env, ok := action.Params["env"].([]string)
	if !ok || len(env) != 1 || env[0] != "BUILD_TYPE=release" {
		t.Errorf("unexpected env: %v", action.Params["env"])
	}
	if chdir, ok := action.Params["chdir"].(string); !ok || chdir != "/opt/src" {
		t.Errorf("expected chdir='/opt/src', got %v", action.Params["chdir"])
	}
}

func TestScript_NoArgsParam(t *testing.T) {
	b := builder.New("test-playbook")

	Script(b, "scripts/setup.sh")

	action := b.Playbook().Actions[0]
	if _, ok := action.Params["args"]; ok {
		t.Error("args param should not be present when not set")
	}
}
