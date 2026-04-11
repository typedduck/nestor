---
title: "Integration Testing Requirements"
type: feature
status: implemented
version: "1.3"
created: 2026-03-22
updated: 2026-04-11
author: marc
---

# Integration Testing Requirements

## Overview

This document defines the requirements for the integration test suite of the Nestor agent. It covers the test infrastructure, the module operations that must be tested, ordering conventions, and error handling expectations. It serves as the quality baseline against which the agent's module implementations are verified on a real Linux system.

## Problem Statement

Nestor's agent executes playbook actions on remote hosts. Unit tests can verify individual components in isolation, but they cannot prove that a module works correctly against a real operating system — particularly for operations like service management, file ownership, or package installation, where the system state is the ground truth. Without a formal integration test specification, coverage is inconsistent and gaps accumulate silently as new modules are added.

## Goals

- Define the required test coverage for each agent module
- Specify the test infrastructure so it can be set up consistently by any contributor
- Establish conventions for test ordering and state management in a shared-container setup
- Provide a baseline that new modules must meet before being considered complete

## Non-Goals

- Windows and macOS targets
- SSH transport layer testing
- Controller-side logic
- CI/CD pipeline integration (not yet in scope)
- End-to-end controller → SSH → agent runs

## Users & Stakeholders

| Role | Description |
|------|-------------|
| Developer (current) | Runs integration tests locally during development |
| Future contributor | Uses this document to understand what to test when adding a new module |

## Functional Requirements

### Test Infrastructure

**FR-01:** The test suite shall use `testcontainers-go` to manage the container lifecycle. The container runtime (Docker or Podman) shall be selectable via the `TC_PROVIDER_TYPE` environment variable (`podman` selects Podman; any other value or absence selects Docker).

**FR-02:** The test container shall be based on Debian 12 and shall run a real systemd instance as PID 1. The container shall have `dbus` available so that systemd service management works correctly.

**FR-03:** Before running any test, `TestMain` shall cross-compile the agent binary for `linux/$GOARCH` and copy it into the container at `/usr/local/bin/nestor-agent`. A build failure shall print a descriptive error to stderr and abort the suite with a non-zero exit code.

**FR-04:** The container image shall be cached between runs (`KeepImage: true`) to avoid rebuilding on every invocation.

**FR-05:** The container shall be started once per test suite run and shared across all tests. `TestMain` shall terminate the container after all tests have completed, regardless of test outcome.

**FR-06:** If Docker or Podman is not available on the host, the suite shall print a clear, human-readable error message identifying the missing runtime and exit with a non-zero code. It shall not panic or produce an opaque stack trace.

**FR-07:** Tests that depend on prior shared container state shall declare that dependency explicitly. Suite-level setup steps (e.g. installing a package needed by a group of tests) shall be performed inside `TestMain`, before `m.Run()` is called, in the order required by the tests that follow.

**FR-08:** When a new module test requires additional packages to be present in the container, the developer adding those tests is responsible for updating the `Containerfile`.

**FR-09:** Common test helper functions shall be extracted into a single shared file (`helpers_test.go`) within the integration test package. The `buildPlaybookArchive` function currently defined in `package_install_test.go` shall be moved there. All module test files shall use this shared helper rather than duplicating the implementation.

### Package Module (existing)

**FR-10:** The suite shall verify that the agent can install a package via `modules.Package` with the `install` operation. After the agent run, the package shall be confirmed installed using `dpkg -l`.

**FR-11:** The suite shall verify that running the same package-install playbook a second time produces `changed: 0` in the agent output, confirming idempotency.

### Service Module

The service module tests require a real systemd-managed service in the container. The service shall be `cron`, installed via `apt-get` in the `Containerfile`. `cron` is lightweight, starts automatically, and supports `ExecReload=` making it suitable for all service operations.

**FR-20:** The suite shall verify that `service.start` on a stopped `cron` service results in the service reaching the `active (running)` state, as confirmed by `systemctl is-active cron`.

**FR-21:** The suite shall verify that `service.start` on an already-running `cron` service exits with code 0 and does not produce an error (idempotency: start is a no-op when the service is already running).

**FR-22:** The suite shall verify that `service.stop` on a running `cron` service results in the service reaching the `inactive` or `dead` state.

**FR-23:** The suite shall verify that `service.stop` on an already-stopped `cron` service exits with code 0 and does not produce an error (idempotency: stop is a no-op when the service is already stopped).

**FR-24:** The suite shall verify that `service.restart` on a running `cron` service results in the service being active after the restart.

**FR-25:** The suite shall verify that `service.reload` on a running `cron` service exits with code 0 and the service remains active.

**FR-26:** The suite shall verify that `modules.DaemonReload` (system-level) exits with code 0 and produces no error, confirming that `systemctl daemon-reload` was executed successfully.

### Command Module

Test scripts shall be stored in `tests/integration/testdata/` and checked into the repository.

**FR-30:** The suite shall verify that `modules.Command` executes a shell command on the remote system. The test shall confirm execution by inspecting a concrete side effect (e.g. a file written by the command, or output captured via a subsequent `exec` call).

**FR-31:** The suite shall verify that `modules.Command` with a `Creates` guard runs the command when the guarded path does not exist, and that the expected side effect is present afterwards.

**FR-32:** The suite shall verify that `modules.Command` with a `Creates` guard skips the command when the guarded path already exists. The test shall confirm the command was not executed by verifying that the side effect that would only result from execution is absent.

**FR-33:** The suite shall verify that `modules.Script` uploads and executes a real script file from `tests/integration/testdata/`. The test shall confirm execution via a concrete side effect.

### File Module

**FR-40:** The suite shall verify that `modules.File` with `Content` creates a file on the remote system with the exact inline content specified.

**FR-41:** The suite shall verify that running the same `file.content` playbook a second time when the file already exists with identical content produces `changed: 0` in the agent output (idempotency).

**FR-42:** The suite shall verify that `modules.File` with `FromTemplate` and `TemplateVars` renders a Go `text/template` file and writes the result to the destination. The test shall read back the rendered file and assert that the template variables were substituted correctly.

**FR-43:** The suite shall verify that `modules.File` with `FromFile` uploads a local file to the correct destination path on the remote system, with matching content.

**FR-44:** The suite shall verify that `modules.Symlink` creates a symbolic link at the specified destination pointing to the correct target, as confirmed by `readlink`.

**FR-45:** The suite shall verify that `modules.Remove` deletes a file from the remote system. The test shall also cover recursive removal of a non-empty directory as part of this test case (not as a separate case).

**FR-46:** The suite shall verify that `modules.Directory` creates a directory on the remote system with the specified permissions (mode), as confirmed by `stat`.

**FR-47:** The suite shall verify that `modules.File` with `Mode` sets the correct file permissions on the remote system, as confirmed by `stat`.

**FR-48:** The suite shall verify that `modules.File` with `Owner` sets the correct owner and group on the remote system using built-in system accounts (e.g. `root:daemon`), as confirmed by `stat`.

## Non-Functional Requirements

**NFR-01 — Shared container:** All tests in a suite run share a single container instance. This is intentional: it reflects real-world provisioning, where an agent applies multiple operations to the same host in sequence. Tests must not assume a pristine environment unless that state was explicitly established in `TestMain`.

**NFR-02 — Container reset:** The container shall be terminated by `TestMain` after the full test suite completes, even if tests fail. This ensures no orphaned containers accumulate on the developer's machine.

**NFR-03 — Image caching:** The container image shall be kept between runs to avoid rebuild overhead during iterative development. The `Containerfile` shall be the single source of truth for image contents.

**NFR-04 — Runtime availability:** When the configured container runtime is unavailable, the suite shall fail fast with a clear error identifying which runtime was expected and that it could not be reached.

## Technical Constraints

- Tests run locally only; no CI environment is defined at this time
- The agent binary is cross-compiled for `linux/$GOARCH` — CGO is disabled
- The container is Debian 12 (apt-based); module tests that install packages assume `apt-get`
- The `service` module tests assume systemd as the init system; the container runs systemd as PID 1
- Timeouts (per-test and per-suite) are left to the implementor's discretion

## Assumptions

- Docker or Podman is installed and running on the developer's machine
- The `cron` package is available in the Debian 12 apt repositories
- Built-in system accounts (e.g. `root`, `daemon`) are present in the container image without additional setup
- Go `text/template` is the only template engine used by the `file.template` operation
- Tests are run with `go test -tags integration ./tests/integration/...`

## Edge Cases & Error Handling

| Scenario | Expected Behavior |
|----------|------------------|
| Docker/Podman not available | Clear error message identifying the missing runtime; non-zero exit |
| Agent binary fails to cross-compile | Error printed to stderr; suite exits with non-zero code |
| Test leaves a service stopped that a later test expects running | Handled by explicit test ordering and `TestMain` setup — not silently tolerated |
| `Creates` guard path exists before a command test runs | The test must pre-create the path deliberately to exercise the skip path |
| Container fails to reach `basic.target` within startup timeout | testcontainers-go returns an error; suite exits with a descriptive message |

## Acceptance Criteria

- [ ] AC-01: All requirements FR-01 through FR-09 are implemented — the test infrastructure operates as specified
- [ ] AC-02: All `package` module requirements (FR-10, FR-11) have passing test cases
- [ ] AC-03: All `service` module requirements (FR-20 through FR-26) have passing test cases
- [ ] AC-04: All `command` module requirements (FR-30 through FR-33) have passing test cases
- [ ] AC-05: All `file` module requirements (FR-40 through FR-48) have passing test cases
- [ ] AC-06: `TestMain` establishes all required suite-level state before `m.Run()` and tears down the container afterwards
- [ ] AC-07: Running the suite twice in succession completes without errors related to stale container state

## Open Questions

| # | Question | Owner | Due |
|---|----------|-------|-----|
| 1 | Which specific package should be used for user-service (`RunAs`) testing? Requires a user with a running systemd user session in the container. | marc | TBD |
| 2 | User-level `daemon-reload` testing: how to set up `XDG_RUNTIME_DIR` and D-Bus socket for a non-root user inside the container? | marc | TBD |
| 3 | When should CI integration be added, and which platform (GitHub Actions, other)? What container runtime will be available? | marc | TBD |

## Changelog

| Version | Date | Author | Summary |
|---------|------|--------|---------|
| 1.3 | 2026-04-11 | marc | Status changed to implemented |
| 1.2 | 2026-04-11 | marc | Status changed to approved |
| 1.1 | 2026-04-11 | marc | Add FR-09: extract shared test helpers into helpers_test.go |
| 1.0 | 2026-03-22 | marc | Initial draft |
