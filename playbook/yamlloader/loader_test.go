package yamlloader_test

import (
	"strings"
	"testing"

	"github.com/typedduck/nestor/playbook/yamlloader"
)

func mustLoad(t *testing.T, yaml string, vars map[string]string) *[]interface{} {
	t.Helper()
	result, err := yamlloader.Load([]byte(yaml), vars)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	out := make([]interface{}, len(actions))
	for i, a := range actions {
		out[i] = a
	}
	return &out
}

func mustLoadErr(t *testing.T, yaml string) error {
	t.Helper()
	_, err := yamlloader.Load([]byte(yaml), nil)
	if err == nil {
		t.Fatal("Load() expected error but got nil")
	}
	return err
}

func TestLoad_PackageUpdate(t *testing.T) {
	const yaml = `
name: test
actions:
  - package: update
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "package.update" {
		t.Errorf("expected type package.update, got %s", actions[0].Type)
	}
}

func TestLoad_PackageUpgrade(t *testing.T) {
	const yaml = `
name: test
actions:
  - package: upgrade
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "package.upgrade" {
		t.Errorf("expected type package.upgrade, got %s", actions[0].Type)
	}
}

func TestLoad_PackageInstall(t *testing.T) {
	const yaml = `
name: test
actions:
  - package:
      install: [nginx, vim]
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "package.install" {
		t.Errorf("expected type package.install, got %s", actions[0].Type)
	}
	pkgs, ok := actions[0].Params["packages"].([]string)
	if !ok {
		t.Fatalf("packages param is not []string, got %T", actions[0].Params["packages"])
	}
	if len(pkgs) != 2 || pkgs[0] != "nginx" || pkgs[1] != "vim" {
		t.Errorf("unexpected packages: %v", pkgs)
	}
}

func TestLoad_PackageRemove(t *testing.T) {
	const yaml = `
name: test
actions:
  - package:
      remove: [apache2]
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Type != "package.remove" {
		t.Errorf("expected type package.remove, got %s", actions[0].Type)
	}
	pkgs, ok := actions[0].Params["packages"].([]string)
	if !ok {
		t.Fatalf("packages param is not []string, got %T", actions[0].Params["packages"])
	}
	if len(pkgs) != 1 || pkgs[0] != "apache2" {
		t.Errorf("unexpected packages: %v", pkgs)
	}
}

func TestLoad_FileContent(t *testing.T) {
	const yaml = `
name: test
actions:
  - file:
      path: /etc/motd
      content: "Welcome\n"
      mode: "0644"
      owner: root
      group: root
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "file.content" {
		t.Errorf("expected type file.content, got %s", a.Type)
	}
	if a.Params["destination"] != "/etc/motd" {
		t.Errorf("unexpected destination: %v", a.Params["destination"])
	}
	if a.Params["owner"] != "root" {
		t.Errorf("unexpected owner: %v", a.Params["owner"])
	}
	if a.Params["mode"] != "0644" {
		t.Errorf("unexpected mode: %v", a.Params["mode"])
	}
}

func TestLoad_FileTemplate(t *testing.T) {
	const yaml = `
name: test
actions:
  - file:
      path: /etc/app/config.toml
      template: config.toml.tmpl
      vars:
        DBHost: db.example.com
        Port: "5432"
      owner: webapp
      mode: "0640"
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "file.template" {
		t.Errorf("expected type file.template, got %s", a.Type)
	}
	if a.Params["source"] != "config.toml.tmpl" {
		t.Errorf("unexpected source: %v", a.Params["source"])
	}
	vars, ok := a.Params["variables"].(map[string]string)
	if !ok {
		t.Fatalf("variables param is not map[string]string, got %T", a.Params["variables"])
	}
	if vars["DBHost"] != "db.example.com" {
		t.Errorf("unexpected DBHost: %v", vars["DBHost"])
	}
}

func TestLoad_FileUpload(t *testing.T) {
	const yaml = `
name: test
actions:
  - file:
      path: /opt/app/bin
      upload: ./build/app
      mode: "0755"
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "file.upload" {
		t.Errorf("expected type file.upload, got %s", a.Type)
	}
	if a.Params["source"] != "./build/app" {
		t.Errorf("unexpected source: %v", a.Params["source"])
	}
	if a.Params["mode"] != "0755" {
		t.Errorf("unexpected mode: %v", a.Params["mode"])
	}
}

func TestLoad_Directory(t *testing.T) {
	const yaml = `
name: test
actions:
  - directory:
      path: /opt/app
      owner: webapp
      group: webapp
      mode: "0755"
      recursive: true
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "directory.create" {
		t.Errorf("expected type directory.create, got %s", a.Type)
	}
	if a.Params["path"] != "/opt/app" {
		t.Errorf("unexpected path: %v", a.Params["path"])
	}
	if a.Params["recursive"] != true {
		t.Errorf("expected recursive=true, got %v", a.Params["recursive"])
	}
}

func TestLoad_Symlink(t *testing.T) {
	const yaml = `
name: test
actions:
  - symlink:
      dest: /etc/nginx/sites-enabled/app
      target: /etc/nginx/sites-available/app
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "file.symlink" {
		t.Errorf("expected type file.symlink, got %s", a.Type)
	}
	if a.Params["destination"] != "/etc/nginx/sites-enabled/app" {
		t.Errorf("unexpected destination: %v", a.Params["destination"])
	}
	if a.Params["source"] != "/etc/nginx/sites-available/app" {
		t.Errorf("unexpected source: %v", a.Params["source"])
	}
}

func TestLoad_Remove(t *testing.T) {
	const yaml = `
name: test
actions:
  - remove:
      path: /opt/app-old
      recursive: true
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "file.remove" {
		t.Errorf("expected type file.remove, got %s", a.Type)
	}
	if a.Params["path"] != "/opt/app-old" {
		t.Errorf("unexpected path: %v", a.Params["path"])
	}
	if a.Params["recursive"] != true {
		t.Errorf("expected recursive=true, got %v", a.Params["recursive"])
	}
}

func TestLoad_CommandShort(t *testing.T) {
	const yaml = `
name: test
actions:
  - command: echo hello
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "command.execute" {
		t.Errorf("expected type command.execute, got %s", a.Type)
	}
	if a.Params["command"] != "echo hello" {
		t.Errorf("unexpected command: %v", a.Params["command"])
	}
}

func TestLoad_CommandLong(t *testing.T) {
	const yaml = `
name: test
actions:
  - command:
      run: useradd -m deploy
      creates: /home/deploy
      env: [KEY=value]
      chdir: /tmp
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "command.execute" {
		t.Errorf("expected type command.execute, got %s", a.Type)
	}
	if a.Params["command"] != "useradd -m deploy" {
		t.Errorf("unexpected command: %v", a.Params["command"])
	}
	if a.Params["creates"] != "/home/deploy" {
		t.Errorf("unexpected creates: %v", a.Params["creates"])
	}
	if a.Params["chdir"] != "/tmp" {
		t.Errorf("unexpected chdir: %v", a.Params["chdir"])
	}
	env, ok := a.Params["env"].([]string)
	if !ok {
		t.Fatalf("env param is not []string, got %T", a.Params["env"])
	}
	if len(env) != 1 || env[0] != "KEY=value" {
		t.Errorf("unexpected env: %v", env)
	}
}

func TestLoad_Script(t *testing.T) {
	const yaml = `
name: test
actions:
  - script:
      source: scripts/setup.sh
      args: [--verbose]
      creates: /etc/setup.done
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "script.execute" {
		t.Errorf("expected type script.execute, got %s", a.Type)
	}
	if a.Params["source"] != "scripts/setup.sh" {
		t.Errorf("unexpected source: %v", a.Params["source"])
	}
	if a.Params["creates"] != "/etc/setup.done" {
		t.Errorf("unexpected creates: %v", a.Params["creates"])
	}
	args, ok := a.Params["args"].([]string)
	if !ok {
		t.Fatalf("args param is not []string, got %T", a.Params["args"])
	}
	if len(args) != 1 || args[0] != "--verbose" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestLoad_Service(t *testing.T) {
	const yaml = `
name: test
actions:
  - service:
      name: nginx
      action: start
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	if a.Type != "service.start" {
		t.Errorf("expected type service.start, got %s", a.Type)
	}
	if a.Params["name"] != "nginx" {
		t.Errorf("unexpected name: %v", a.Params["name"])
	}
}

func TestLoad_VarSubstitution(t *testing.T) {
	const yaml = `
name: test
vars:
  app_port: "8080"
actions:
  - file:
      path: /tmp/port
      content: "port=${app_port}\n"
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	content := actions[0].Params["content"]
	if content != "port=8080\n" {
		t.Errorf("expected content %q, got %q", "port=8080\n", content)
	}
}

func TestLoad_VarOverride(t *testing.T) {
	const yaml = `
name: test
vars:
  app_port: "8080"
actions:
  - file:
      path: /tmp/port
      content: "${app_port}"
`
	result, err := yamlloader.Load([]byte(yaml), map[string]string{"app_port": "9090"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	content := actions[0].Params["content"]
	if content != "9090" {
		t.Errorf("expected content %q, got %q", "9090", content)
	}
}

func TestLoad_Environment(t *testing.T) {
	const yaml = `
name: test
environment:
  ENVIRONMENT: production
  APP_VERSION: "1.2.3"
actions: []
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	env := result.Remote.Environment
	if env["ENVIRONMENT"] != "production" {
		t.Errorf("expected ENVIRONMENT=production, got %q", env["ENVIRONMENT"])
	}
	if env["APP_VERSION"] != "1.2.3" {
		t.Errorf("expected APP_VERSION=1.2.3, got %q", env["APP_VERSION"])
	}
}

func TestLoad_ActionSequence(t *testing.T) {
	const yaml = `
name: test
actions:
  - package: update
  - package: upgrade
  - command: echo done
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	actions := result.Remote.Actions
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
	expected := []string{"action-001", "action-002", "action-003"}
	for i, a := range actions {
		if a.ID != expected[i] {
			t.Errorf("action %d: expected ID %s, got %s", i, expected[i], a.ID)
		}
	}
}

func TestLoad_UnknownAction(t *testing.T) {
	const yaml = `
name: test
actions:
  - bogus: value
`
	mustLoadErr(t, yaml)
}

func TestLoad_InvalidYAML(t *testing.T) {
	const yaml = `
name: test
actions:
  - [invalid yaml
`
	mustLoadErr(t, yaml)
}

func TestLoad_MissingFilePath(t *testing.T) {
	const yaml = `
name: test
actions:
  - file:
      content: hello
`
	mustLoadErr(t, yaml)
}

func TestLoad_MissingFileSource(t *testing.T) {
	const yaml = `
name: test
actions:
  - file:
      path: /tmp/hello
`
	mustLoadErr(t, yaml)
}

// --- New tests for pre:/post: phases ---

func TestLoad_BackwardCompat(t *testing.T) {
	const yaml = `
name: test
actions:
  - command: echo hello
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pre != nil {
		t.Errorf("expected Pre to be nil, got %v", result.Pre)
	}
	if result.Post != nil {
		t.Errorf("expected Post to be nil, got %v", result.Post)
	}
	if result.Remote == nil {
		t.Fatal("expected Remote to be non-nil")
	}
	if len(result.Remote.Actions) != 1 {
		t.Errorf("expected 1 Remote action, got %d", len(result.Remote.Actions))
	}
}

func TestLoad_WithPreSection(t *testing.T) {
	const yaml = `
name: test
pre:
  - command: echo pre
actions:
  - command: echo remote
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pre == nil {
		t.Fatal("expected Pre to be non-nil")
	}
	if len(result.Pre.Actions) != 1 || result.Pre.Actions[0].Params["command"] != "echo pre" {
		t.Errorf("unexpected Pre actions: %v", result.Pre.Actions)
	}
	if result.Post != nil {
		t.Errorf("expected Post to be nil, got %v", result.Post)
	}
	if len(result.Remote.Actions) != 1 {
		t.Errorf("expected 1 Remote action, got %d", len(result.Remote.Actions))
	}
}

func TestLoad_WithPostSection(t *testing.T) {
	const yaml = `
name: test
actions:
  - command: echo remote
post:
  - command: echo post
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pre != nil {
		t.Errorf("expected Pre to be nil, got %v", result.Pre)
	}
	if result.Post == nil {
		t.Fatal("expected Post to be non-nil")
	}
	if len(result.Post.Actions) != 1 || result.Post.Actions[0].Params["command"] != "echo post" {
		t.Errorf("unexpected Post actions: %v", result.Post.Actions)
	}
}

func TestLoad_WithPreAndPost(t *testing.T) {
	const yaml = `
name: test
pre:
  - command: echo pre
actions:
  - command: echo remote
post:
  - command: echo post
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pre == nil {
		t.Fatal("expected Pre to be non-nil")
	}
	if result.Remote == nil {
		t.Fatal("expected Remote to be non-nil")
	}
	if result.Post == nil {
		t.Fatal("expected Post to be non-nil")
	}
	if len(result.Pre.Actions) != 1 {
		t.Errorf("expected 1 Pre action, got %d", len(result.Pre.Actions))
	}
	if len(result.Remote.Actions) != 1 {
		t.Errorf("expected 1 Remote action, got %d", len(result.Remote.Actions))
	}
	if len(result.Post.Actions) != 1 {
		t.Errorf("expected 1 Post action, got %d", len(result.Post.Actions))
	}
}

func TestLoad_PreRejectsPackage(t *testing.T) {
	const yaml = `
name: test
pre:
  - package: update
actions:
  - command: echo hello
`
	err := mustLoadErr(t, yaml)
	if !strings.Contains(err.Error(), "pre") {
		t.Errorf("expected error to mention 'pre', got: %v", err)
	}
}

func TestLoad_PreRejectsService(t *testing.T) {
	const yaml = `
name: test
pre:
  - service:
      name: nginx
      action: start
actions:
  - command: echo hello
`
	err := mustLoadErr(t, yaml)
	if !strings.Contains(err.Error(), "pre") {
		t.Errorf("expected error to mention 'pre', got: %v", err)
	}
}

func TestLoad_PostRejectsDirectory(t *testing.T) {
	const yaml = `
name: test
actions:
  - command: echo hello
post:
  - directory:
      path: /tmp/out
`
	err := mustLoadErr(t, yaml)
	if !strings.Contains(err.Error(), "post") {
		t.Errorf("expected error to mention 'post', got: %v", err)
	}
}

func TestLoad_EnvironmentSharedAcrossPhases(t *testing.T) {
	const yaml = `
name: test
environment:
  MY_VAR: hello
pre:
  - command: echo pre
actions:
  - command: echo remote
post:
  - command: echo post
`
	result, err := yamlloader.Load([]byte(yaml), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, phase := range []*struct {
		name string
		env  map[string]string
	}{
		{"pre", result.Pre.Environment},
		{"remote", result.Remote.Environment},
		{"post", result.Post.Environment},
	} {
		if phase.env["MY_VAR"] != "hello" {
			t.Errorf("%s phase: expected MY_VAR=hello, got %q", phase.name, phase.env["MY_VAR"])
		}
	}
}
