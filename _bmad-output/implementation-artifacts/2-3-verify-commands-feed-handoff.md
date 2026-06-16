---
baseline_commit: 2b2a870bb8bbd4a61490c37407d1879841e721c6
---

# Story 2.3: Verify commands feed the Handoff (`aictl verify`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `aictl verify` to run my configured verification commands and capture their output,
so that the next handoff carries the real build/test state — including failures — for the next agent to see.

## Acceptance Criteria

1. **Runs configured Verify Commands:** `aictl verify` runs each command in the session Config's `verify` list (in order), shelling out so commands like `go test ./...` / `npm run build` work, capturing combined stdout+stderr. (FR-18)
2. **Stores latest output:** the combined output of the run is written to `.ai-session/latest-verify.txt` (overwriting the previous run) via `session.WriteAtomic`. (FR-18, NFR-3)
3. **Feeds the handoff:** the next `aictl handoff` embeds that output (already wired in Story 2.2 via include-if-present `latest-verify.txt`). (FR-18)
4. **Logs the invocation:** each verify run is appended to `.ai-session/command-log.md` (the aictl-invoked command log the handoff reads). (FR-18)
5. **Captures failures, doesn't crash on them:** a verify command exiting non-zero (e.g. failing tests) is captured into `latest-verify.txt` — that is the point (test-aware handoff), not an aictl error. `aictl verify`'s own exit reflects whether all commands passed.
6. **Requires a session; graceful when unconfigured:** no `.ai-session/` → clear error + non-zero exit (`ErrNoSession`). No `verify` commands configured → a clear, non-error message telling the user to add some to `config.yaml`. Offline (aictl itself makes no network calls; the user's commands may do whatever). (NFR-1)

## Tasks / Subtasks

- [x] **Task 1 — Verify runner `internal/verify` (AC: 1, 5)**
  - [x] `internal/verify/verify.go`: `Run(ctx, dir, commands) (Result, error)` — `Result{Commands []CommandResult, Combined string, AllPassed bool}`; shells `sh -c`, captures `CombinedOutput`.
  - [x] Non-zero exit recorded (flips `AllPassed`), not returned as error; `Run` errors only on inability to execute. No FS writes — unit-tested.
- [x] **Task 2 — `App.Verify` use-case (AC: 2, 3, 4, 6)**
  - [x] `internal/app/verify.go`: cwd → require `.ai-session/` (`ErrNoSession`) → `config.Load` → empty `Verify` → guidance + exit 0 → `verify.Run` → `WriteAtomic(LatestVerify)` → append `CommandLog` → per-command summary → error (non-zero exit) iff a command failed (output captured first).
  - [x] `command-log.md` append via `O_CREATE|O_APPEND` (the one append-artifact; documented); entries `<RFC3339 UTC> $ <cmd> (exit N)`.
- [x] **Task 3 — Command `cmd/aictl/verify.go` (AC: 1)**
  - [x] `aictl verify` (`cobra.NoArgs`) → `app.Verify`; registered on root.
- [x] **Task 4 — Tests (AC: 1–6)**
  - [x] `internal/verify/verify_test.go`: pass+fail capture, exit codes, `AllPassed`, order, combined output.
  - [x] `internal/app/verify_test.go`: writes `latest-verify.txt` + `command-log.md`; failing command → still captured + returns error; no session → `ErrNoSession`; empty config → no error, nothing written.
  - [x] Integration `TestVerifyOutputFeedsHandoff`: after `Verify`, `Handoff` contains the verify text (confirms 2.2 wiring).

### Review Findings (code review 2026-06-16)

_3 adversarial layers. Acceptance Auditor: all 6 ACs PASS (integration test proves handoff-embed). Hunters found a real ctx-handling bug in the runner + the expected size/secret deferrals._

**Patch (applied 2026-06-16):**

- [x] [Review][Patch] `verify.Run` is now ctx-aware and non-destructive: returns `ctx.Err()` on cancel/timeout (no more "tests failed" for a Ctrl-C'd run; no churning the rest against a dead ctx), records start-failures (non-`ExitError`) as captured failed commands instead of discarding prior output, and skips blank commands. Tests `TestRunStopsOnCanceledContext`, `TestRunSkipsBlankCommands` [internal/verify/verify.go]
- [x] [Review][Patch] `appendCommandLog` surfaces the `f.Close()` error via named return [internal/app/verify.go]

**Deferred (tracked in deferred-work.md):**

- [x] [Review][Defer] Bound the verify channels: cap captured output size (capture-side limited writer), truncate `latest-verify.txt`, bound what the handoff embeds, and rotate/cap `command-log.md` (it's append-only and embedded whole today) — the verify analogue of the diff's `maxDiffChars` → handoff/verify hardening
- [x] [Review][Defer] Dedicated verify timeout (Q4) — commands run under the inherited signal-aware ctx, so Ctrl-C now interrupts cleanly (after the patch above); a configurable per-run deadline for unattended use remains a follow-up
- [x] [Review][Defer] Content secret-scanner over verify output / command log (reinforces the 2.2-deferred scanner; both files are gitignored, but their *content* embedded in the handoff isn't redacted) → post-v1, Epic-3 gate

**Dismissed:** `0o644` mode on the gitignored local artifacts (mode isn't the guardrail — redaction/content-scanner is); env not scrubbed (verify commands *need* PATH/HOME/etc. to run); `$ cmd (exit N)` block-separator "spoofing" (nothing parses the file programmatically — it's for human/LLM reading); per-batch timestamp in the log (minor diagnostic nicety; would need per-command timing); missing-vs-empty config message nuance.

## Dev Notes

**Third story of Epic 2. Depends on Epic 1 + 2.1/2.2.** Small: 2.2 already built the consumption side — `paths.LatestVerify()` / `paths.CommandLog()` exist and `App.Handoff` reads them include-if-present. This story is the **producer**.

### Reuse (don't reinvent)

- **Config:** `cfg.Verify []string` already exists (`yaml:"verify"`), default empty. Load via `config.Load(paths.Config())` (now normalizes guardrails — Story 2.2 patch).
- **Paths:** `paths.LatestVerify()` (`.ai-session/latest-verify.txt`) and `paths.CommandLog()` (`.ai-session/command-log.md`) added in Story 2.2. **Don't add new path accessors.** Both are already gitignored (Story 2.2 review patch) — good, they're transient/secret-prone.
- **Session:** `session.WriteAtomic` for `latest-verify.txt`; `ErrNoSession` from `internal/app` (Story 1.4); app+thin-cmd pattern (`internal/app/*.go` ↔ `cmd/aictl/*.go` ↔ `root.go`).

### Shelling out for verify commands

- Use `sh -c "<cmd>"` so a config entry like `go test ./...` runs as the user expects. `cmd.Dir = root`. Capture combined stdout+stderr (e.g. assign both to one buffer). Unix-only is fine (PRD targets macOS+Linux).
- **No-network note:** NFR-1 forbids *aictl* from making network/LLM calls — it does not constrain the user's verify commands (which may build/test however they like). The `internal/verify` package itself must not import networking packages (depguard/guard test still apply to it).
- Bound execution: consider a generous timeout via `ctx` (verify commands can be slow — don't reuse git's tight 10s; this is user-controlled work). A per-command or overall deadline is a reasonable safety net; document whatever you choose. (Avoid hanging forever on an interactive command.)

### Secrets in verify output (known, deferred)

- `latest-verify.txt` is embedded into the handoff **unredacted** — verify output can contain tokens/URLs. This is exactly the **content secret-scanner** gap deferred from the Story 2.2 review (`deferred-work.md`): path-based redaction can't catch secrets in command output. Do **not** try to solve content-scanning here. Two cheap, in-scope mitigations are already in place / appropriate: (a) `latest-verify.txt` + `command-log.md` are gitignored (2.2), so they aren't committed; (b) note in the story that bounding their embedded size in the handoff is deferred. The content scanner remains the gate before Epic-3 auto-injection.

### Determinism note

- `command-log.md` entries carry timestamps — that's fine. The handoff *generator* is deterministic for identical inputs; `command-log.md` is an input that legitimately changes over time. No timestamps go inside `handoff.Generate` itself.

### Conventions (inherited — CI-enforced)

- Pure-ish runner (no FS in `internal/verify`); thin `cmd/`; `context.Context` first; `%w` wrapping; atomic write for `latest-verify.txt`; no-network imports; gofmt import alignment; errcheck on (handle every return — capture `cmd.Run()` / exit errors deliberately). [Source: architecture.md#Implementation Patterns]

### Learnings carried from Epic 1/2 (real)

- Reviews keep rewarding **faithful capture + safe failure**: AC 5 (capture failing tests rather than erroring) and the "still write output even when a command fails" requirement are the analogues here — don't skip them.
- `command-log.md` is the first **append** artifact; everything else uses full-rewrite `WriteAtomic`. Appending is correct for a log, but guard the open/write/close errors (errcheck) and create-if-missing.
- Tests: `t.Chdir(t.TempDir())`, `newTestApp()` (UI→`io.Discard`), temp `git init` only if you need git (verify doesn't need a repo — it just runs commands; but `init`/`start` set up the session). Use trivial shell commands (`true`, `false`, `echo hi`) so tests are fast and portable.

### Project Structure Notes

- NEW: `internal/verify/{verify.go,verify_test.go}`, `internal/app/{verify.go,verify_test.go}`, `cmd/aictl/verify.go`. MODIFIED: `cmd/aictl/root.go` (register `verify`). No new external dependencies, no new path accessors, no config schema change.

### References

- [Source: epics.md#Epic 2 → Story 2.3]
- [Source: prd.md FR-18 (verify commands feed handoff); NFR-1]
- [Source: architecture.md#Features (Verify Commands); #Implementation Patterns]
- [Source: implementation-artifacts/2-2-generate-deterministic-handoff.md — handoff already reads latest-verify.txt/command-log.md include-if-present]
- [Source: deferred-work.md — content secret-scanner (verify output) + bound embedded verify/log size]

### Open Questions

1. **`aictl verify` exit code** — reflect the verify result (non-zero if any command failed) so it composes in scripts, while always writing `latest-verify.txt`. Recommended; confirm vs always-exit-0.
2. **`command-log.md` entry format** — `<RFC3339 UTC> $ <cmd> (exit N)` per command assumed. Confirm.
3. **Runner location** — `internal/verify` (recommended, unit-testable) vs inline in `app`. Confirm.
4. **Verify timeout** — a generous per-command/overall deadline vs none. Recommend a generous overall ctx deadline (not git's 10s). Confirm.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (BMad dev-story workflow)

### Debug Log References

- `go mod tidy` clean; `go build ./...` ok; `go vet ./...` no issues; `gofmt` clean
- `go test ./...` → **75 passed** across 9 packages (new `internal/verify`)
- Smoke (temp git repo): config `verify: [echo building ok, false]` → `aictl verify` ran both, printed `ok` / `FAILED (exit 1)`, **exit 1** while still writing `latest-verify.txt` (both blocks) and appending two timestamped `command-log.md` lines; `aictl handoff` then embedded "building ok" → integration confirmed.

### Completion Notes List

- **Producer-only story:** 2.2 already wired consumption (`paths.LatestVerify()`/`CommandLog()` + handoff include-if-present), so this story just writes those files — handoff embedding works with no handoff changes.
- **Test-aware capture (AC5):** a failing verify command's output is captured to `latest-verify.txt` regardless; `App.Verify` returns an error *after* writing, so the process exits non-zero while the failing state is preserved for the handoff.
- **`internal/verify` is a pure-ish runner** (no FS) returning structured results — unit-tested independent of the app; `App.Verify` does the FS writes. Non-zero exits are recorded outcomes, not errors.
- **`command-log.md` is the first append artifact** — `O_CREATE|O_APPEND|O_WRONLY` (a full-rewrite of a growing log would be wasteful); errcheck-clean (Close deferred-ignored, Fprintf checked).
- **Open-question dispositions:** Q1 exit reflects verify result (output always captured first); Q2 log format `<RFC3339 UTC> $ <cmd> (exit N)`; Q3 separate `internal/verify` package; Q4 verify runs under the inherited (signal-aware) ctx without git's tight 10s — a dedicated generous timeout left as a future option (no hang observed; ctx cancellation honored).
- **Known/deferred (unchanged):** `latest-verify.txt` is embedded unredacted → the content secret-scanner (deferred, Epic-3 gate) still applies; bounding its embedded size in the handoff remains deferred. Both `latest-verify.txt`/`command-log.md` are gitignored (2.2).
- Conventions: thin `cmd/`, context-first, `%w` wrapping, atomic write for verify output, no network imports, gofmt clean.

### File List

- `internal/verify/verify.go` (new)
- `internal/verify/verify_test.go` (new)
- `internal/app/verify.go` (new)
- `internal/app/verify_test.go` (new)
- `cmd/aictl/verify.go` (new)
- `cmd/aictl/root.go` (modified — register verify)

### Change Log

- 2026-06-16: Implemented Story 2.3 — `aictl verify` (FR-18). Added `internal/verify` runner (shell-out, captures pass/fail), `App.Verify` (writes `latest-verify.txt` atomically, appends `command-log.md`, exit reflects result), and the `verify` command. Handoff embeds the output via the existing 2.2 wiring. 75 tests passing.
- 2026-06-16: Addressed code review — 2 patches: ctx-aware/non-destructive `verify.Run` (cancel ≠ test failure; record start-failures; skip blanks) and `appendCommandLog` Close-error surfacing. 3 deferred (bound verify channels, configurable verify timeout, content secret-scanner). 77 tests passing.
