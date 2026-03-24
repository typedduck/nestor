# Integration Test Plan

Location: `tests/integration/agent/`
Framework: testcontainers-go + Podman, Debian 12 + systemd container, Ed25519 signing per-test.

This plan covers only test cases **not already specified in `docs/prd/integration-testing-requirements.md`**.
Infrastructure setup and baseline module coverage are defined there (FR-01–FR-48).

## Current State (as of 2026-03-24)

- 2 tests exist: `package_install_test.go` — `package.install` basic + idempotency
- All other action types are untested end-to-end

## Planned Test Files

### 1. `package_test.go`

Not covered by PRD (FR-10/FR-11 cover install only):

- `TestPackageRemove_RemovesPackage` — pre-install, remove, verify gone
- `TestPackageRemove_Idempotent` — remove non-existent → `changed: 0`
- `TestPackageUpdate_UpdatesCache` — apt-get update, verify exit 0
- `TestPackageUpgrade_Upgrades` — upgrade, verify success

### 2. `file_content_test.go`

Not covered by PRD (FR-40–FR-43 cover create, idempotency, template render, upload):

- `TestFileContent_UpdatesOnChange` — different content → `changed: 1`
- `TestFileUpload_Idempotent` — same file twice → `changed: 0`
- `TestFileTemplate_Idempotent` — same render twice → `changed: 0`

### 3. `file_ops_test.go`

Not covered by PRD (FR-44–FR-46 cover create cases only):

- `TestSymlink_Idempotent` — same target twice → `changed: 0`
- `TestSymlink_ReplacesStaleLink` — wrong-target existing symlink → redirected, `changed: 1`
- `TestDirectory_Idempotent` — create existing dir → `changed: 0`
- `TestFileRemove_Idempotent` — remove non-existent → `changed: 0`

### 4. `command_test.go`

Not covered by PRD (FR-30–FR-33 cover execute, creates guard, and script execute):

- `TestCommand_WithEnvAndChdir` — verify env vars and working dir are applied
- `TestScript_WithArgs` — script echoes args to file, verify
- `TestScript_WithCreatesGuard` — script guarded by existing path → skipped

### 5. `sequence_test.go`

Not in PRD:

- `TestSequence_InstallAndConfigure` — install nginx + write config + start in one playbook
- `TestSequence_Idempotent` — 5-action playbook run twice; second run `changed: 0` for all
- `TestSequence_FileLifecycle` — create dir → write file → create symlink
- `TestSequence_CommandChain` — multiple commands with `creates` guards

## Infrastructure Additions

Implementation details not specified in the PRD:

- **`runPlaybook()` helper** in `container_test.go` — extract boilerplate (build → sign → copy → exec → parse output)
- **Output parser** — parse `changed: N` from agent stdout
- **`tests/integration/testdata/`** — template files and scripts for upload/template/script tests
- **Containerfile update** — pre-install `cron` for service tests

## Priority Order

1. File content/upload/template
2. Directory + symlink + remove
3. Command + script
4. Service (requires Containerfile fixture)
5. Package remove/update/upgrade
6. Sequences
