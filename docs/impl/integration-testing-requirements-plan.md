# Implementation Plan: Integration Testing Requirements

PRD: [docs/prd/integration-testing-requirements.md](../prd/integration-testing-requirements.md)  
Status: pending review  
Date: 2026-04-11

---

## Prerequisites / gaps found

**Packager gap (blocks FR-33):** `controller/packager/packager.go`'s `collectUploadFiles`
handles `file.upload` and `file.template` sources but not `script.execute`. Without a fix,
`modules.Script` archives won't include the script file and the agent will fail. Fixed in
Phase 1 alongside the helpers extraction.

---

## Phase 1 — Shared helpers + packager script support (FR-09)

**Implements:** FR-09 + packager prerequisite for FR-33  
**Satisfies:** AC-01 (partial)

### Files

| File | Change |
|------|--------|
| `tests/integration/agent/helpers_test.go` | new — `buildPlaybookArchive` moved here |
| `tests/integration/agent/package_install_test.go` | remove `buildPlaybookArchive` |
| `controller/packager/packager.go` | add `script.execute` branch in `collectUploadFiles` |

### Functions

**`helpers_test.go`**
- `buildPlaybookArchive(t *testing.T, pb *playbook.Playbook) *packager.Package`  
  Moved verbatim from `package_install_test.go`. Generates a fresh ed25519 key pair,
  packages the playbook, signs it, and returns the `*packager.Package`.

**`packager.go` — `collectUploadFiles`**  
Add a `script.execute` branch parallel to the existing `file.upload` branch: copy the
source file to `upload/<basename>`, append `"upload/<basename>"` to the files list,
and rewrite `action.Params["source"]` to `"upload/<basename>"`.

---

## Phase 2 — Service module tests (FR-20–26)

**Implements:** FR-07 (cron suite-level setup), FR-20, FR-21, FR-22, FR-23, FR-24, FR-25, FR-26  
**Satisfies:** AC-03, AC-06 (partial)

### Files

| File | Change |
|------|--------|
| `tests/integration/agent/Containerfile` | add `cron` to apt-get install |
| `tests/integration/agent/main_test.go` | add `suite.exec(ctx, "systemctl", "stop", "cron")` in `runSuite` before `m.Run()` |
| `tests/integration/agent/service_test.go` | new |

### Tests (ordered by definition to control execution sequence)

| Test | FR | Initial state | Agent action | Verification |
|------|----|----|---|---|
| `TestService_Start_WhenStopped` | FR-20 | cron stopped (TestMain) | `service.start` | `systemctl is-active cron` exits 0 |
| `TestService_Start_Idempotent` | FR-21 | cron running (prev test) | `service.start` | agent exits 0, no error |
| `TestService_Stop_WhenRunning` | FR-22 | cron running | `service.stop` | `systemctl is-active cron` exits non-zero |
| `TestService_Stop_Idempotent` | FR-23 | cron stopped | `service.stop` | agent exits 0, no error |
| `TestService_Restart` | FR-24 | test starts cron via `systemctl start`, then runs agent | `service.restart` | `systemctl is-active cron` exits 0 |
| `TestService_Reload` | FR-25 | cron running (prev test) | `service.reload` | agent exits 0, `is-active` exits 0 |
| `TestService_DaemonReload` | FR-26 | no service dependency | `service.daemon-reload` | agent exits 0 |

---

## Phase 3 — Command module tests (FR-30–33)

**Implements:** FR-30, FR-31, FR-32, FR-33  
**Satisfies:** AC-04

### Files

| File | Change |
|------|--------|
| `tests/integration/agent/testdata/test-script.sh` | new — writes sentinel file to `/tmp/script-ran` |
| `tests/integration/agent/command_test.go` | new |

### Tests

| Test | FR | Notes |
|------|----|----|
| `TestCommand_Execute` | FR-30 | `echo hello > /tmp/cmd-out.txt`; verify with `cat /tmp/cmd-out.txt` containing "hello" |
| `TestCommand_Creates_WhenAbsent` | FR-31 | guard `/tmp/cmd-creates-guard` absent; command runs `touch /tmp/cmd-creates-guard`; verify guard exists |
| `TestCommand_Creates_WhenPresent` | FR-32 | pre-create guard via `suite.exec`; command would write `/tmp/cmd-skip-proof`; verify proof absent |
| `TestScript_Execute` | FR-33 | `modules.Script(b, "testdata/test-script.sh")`; verify sentinel `/tmp/script-ran` exists |

---

## Phase 4 — File module tests (FR-40–48)

**Implements:** FR-40, FR-41, FR-42, FR-43, FR-44, FR-45, FR-46, FR-47, FR-48  
**Satisfies:** AC-05

### Files

| File | Change |
|------|--------|
| `tests/integration/agent/testdata/config.tmpl` | new — Go `text/template` with one variable, e.g. `server={{ .Host }}` |
| `tests/integration/agent/testdata/upload-me.txt` | new — static file with known content |
| `tests/integration/agent/file_test.go` | new |

### Tests

| Test | FR | Verification |
|------|----|----|
| `TestFile_Content_Creates` | FR-40 | `cat /tmp/file-content.txt` contains expected inline string |
| `TestFile_Content_Idempotent` | FR-41 | second agent run output contains `changed: 0` |
| `TestFile_Template` | FR-42 | `cat /tmp/rendered.conf` matches expected rendered output with substituted variables |
| `TestFile_Upload` | FR-43 | `cat /tmp/uploaded.txt` matches `testdata/upload-me.txt` content |
| `TestFile_Symlink` | FR-44 | `readlink /tmp/test-link` == target path |
| `TestFile_Remove` | FR-45 | file remove + recursive directory remove; both verified with `stat` (non-zero exit) |
| `TestFile_Directory` | FR-46 | `stat -c %a /tmp/test-dir` == expected octal mode |
| `TestFile_Mode` | FR-47 | `stat -c %a /tmp/mode-file.txt` == expected octal mode |
| `TestFile_Owner` | FR-48 | `stat -c %U:%G /tmp/owned-file.txt` == `root:daemon` |

---

## Acceptance criteria mapping

| AC | Verified by |
|----|-------------|
| AC-01 (FR-01–FR-09) | Phase 1 (FR-09 + packager); existing code covers FR-01–FR-08 |
| AC-02 (FR-10, FR-11) | Existing `package_install_test.go` — no new work needed |
| AC-03 (FR-20–FR-26) | Phase 2 |
| AC-04 (FR-30–FR-33) | Phase 3 |
| AC-05 (FR-40–FR-48) | Phase 4 |
| AC-06 (TestMain setup/teardown) | Phase 2 (cron stop added); existing teardown unchanged |
| AC-07 (two successive runs) | Manual check after all phases pass |
