---
baseline_commit: 5e95313
---

# Story 2.4: Last-resort recovery (`aictl recover`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `aictl recover` to generate a clean continuation prompt from minimal local state,
so that I have a safety net even when providers are unavailable and optional session artifacts are missing.

## Acceptance Criteria

1. **Produces a minimal recovery prompt:** `aictl recover` produces a usable continuation prompt from exactly the current branch, current redacted/bounded git diff, and the original Goal. It must not require verify output, command log, previous handoff, provider transcript, checkpoints, or any live provider. (FR-14, NFR-1)
2. **Works when optional session artifacts are missing:** if `.ai-session/` exists with `state.yaml` containing `goal`, recovery still succeeds when `config.yaml`, `handoff.md`, `latest-verify.txt`, `command-log.md`, and `checkpoints/` are absent or empty. Missing config uses safe defaults.
3. **Fails clearly when the Goal is unavailable:** if the original Goal cannot be read from Task State, `recover` returns a clear non-zero error explaining that recovery needs the original Goal. It must not fabricate or infer a goal from git state.
4. **Preserves privacy and size guardrails:** the recovery prompt uses the existing `internal/git.Diff(ctx, root, denylist, maxDiffChars)` path so denylisted files are redacted before truncation, matching handoff behavior.
5. **Writes deterministically and safely:** the prompt rendering is a pure deterministic function, and the written recovery artifact is persisted through `session.WriteAtomic`. Re-running with identical inputs produces byte-identical prompt content.
6. **Adds the CLI surface:** `aictl recover` is a thin Cobra command (`cobra.NoArgs`) registered on the root command. It invokes `App.Recover(ctx)` and prints only concise success/error output.
7. **No provider or network path:** recovery imports no provider packages, invokes no provider CLI, performs no network/LLM calls, and remains covered by the existing no-network import guard.

## Tasks / Subtasks

- [x] **Task 1 - Recovery prompt generator (AC: 1, 5)**
  - [x] Add `internal/handoff/recover.go` with `RecoveryInput{Goal, Branch, Diff}` and `GenerateRecovery(in RecoveryInput) (string, error)`.
  - [x] Keep the generator pure: no filesystem, git, config, clock, randomness, or environment reads.
  - [x] Include explicit anti-redo instructions: continue from current state, inspect changed files first, preserve existing work, run verification before finishing when commands are known.
  - [x] Keep the output minimal. Do not include latest verify output, command log, recent commits, decisions, next steps, or known failures; those belong to the full handoff.
- [x] **Task 2 - Session path and app use-case (AC: 1-5, 7)**
  - [x] Add `Paths.Recovery()` returning `.ai-session/recovery.md`.
  - [x] Add `App.Recover(ctx)` in `internal/app/recover.go`.
  - [x] Resolve cwd with `os.Getwd()`, then `session.NewPaths(root)`.
  - [x] Require `.ai-session/state.yaml` to contain a non-empty `Goal`; if it is missing, unreadable, malformed, or goal-empty, return a clear error. Do not auto-init and do not use placeholders.
  - [x] Load config with a last-resort posture: missing config should use `config.Default()`. If a present config is unreadable/malformed, warn through `a.UI.Errorf` and continue with `config.Default()` rather than blocking recovery.
  - [x] Capture current branch with `git.Branch(ctx, root)` and current diff with `git.Diff(ctx, root, cfg.Denylist, cfg.Handoff.MaxDiffChars)`.
  - [x] Treat `git.ErrNotARepo` as an error for `recover`: this command is defined around current branch + diff, unlike `handoff` which tolerates sparse git sections.
  - [x] Write the rendered prompt to `paths.Recovery()` via `session.WriteAtomic(..., 0o644)` and print `Wrote recovery prompt to <path>`.
- [x] **Task 3 - Cobra command (AC: 6)**
  - [x] Add `cmd/aictl/recover.go` with `newRecoverCmd(a *app.App) *cobra.Command`.
  - [x] Register it in `cmd/aictl/root.go`.
  - [x] Keep command code thin; no git/session/handoff logic in `cmd/`.
- [x] **Task 4 - Tests (AC: 1-7)**
  - [x] `internal/handoff/recover_test.go`: deterministic output; contains goal/branch/diff; omits full-handoff-only sections; includes anti-redo recovery instructions.
  - [x] `internal/app/recover_test.go`: started session + dirty repo writes `.ai-session/recovery.md` with goal, branch, and diff.
  - [x] App test: succeeds when optional artifacts (`config.yaml`, `handoff.md`, `latest-verify.txt`, `command-log.md`, `checkpoints/`) are missing, using safe defaults.
  - [x] App test: denylisted tracked file content is redacted and truncation marker appears for an oversized diff.
  - [x] App test: missing/malformed/goal-empty state returns a clear error and writes no recovery artifact.
  - [x] Command/root test: `recover` is registered and delegates to app behavior through the root command.

### Review Findings

- [x] [Review][Patch] Harden recovery Markdown fences for diff content [internal/handoff/recover.go:32]
- [x] [Review][Patch] Prevent recovery prompt structure injection through raw Goal text [internal/handoff/recover.go:25]
- [x] [Review][Defer] Resolve true repo root before session lookup [internal/app/recover.go:19] — deferred, pre-existing
- [x] [Review][Defer] Decide whether untracked files belong in recovery state [internal/app/recover.go:43] — deferred, pre-existing

## Dev Notes

**Fourth story of Epic 2. Depends on Epic 1 plus Stories 2.1-2.3.** This is the fallback path when the richer handoff path cannot be trusted because optional session artifacts are missing. It should reuse the proven git/session/handoff patterns, but it should not call `App.Handoff` or use the full handoff template because recovery has a stricter minimal-input contract.

### Reuse (do not reinvent)

- **Git capture:** reuse `internal/git.Branch` and `internal/git.Diff`. `git.Diff` already shells out to the real git binary, combines staged + unstaged diff, verifies the working tree, redacts denylisted sections before truncation, and respects `maxDiffChars`.
- **Config guardrails:** use `config.Load` when possible and `config.Default()` when recovery must continue despite a missing/broken config. Defaults are `DefaultDenylist = [".env*", "*.pem", "*.key", "id_*"]` and `DefaultMaxDiffChars = 30000`.
- **Session state:** read the Goal from `session.LoadState(paths.State())`. Goal is the only Task State field recovery needs.
- **Persistence:** write `.ai-session/recovery.md` via `session.WriteAtomic`; do not use `os.WriteFile` for the recovery artifact.
- **App/CLI layering:** follow the existing `Handoff` and `Verify` pattern: `internal/app/recover.go` owns orchestration; `cmd/aictl/recover.go` only wires Cobra args to `App.Recover(ctx)`.

### Current files to modify

- `internal/handoff/generator.go` / `templates.go` / `handoff.tmpl`: full handoff generation is currently pure and deterministic. Preserve that behavior. Prefer adding `recover.go` rather than changing the full handoff template.
- `internal/app/handoff.go`: current `App.Handoff` requires `.ai-session/`, loads config and state, reads optional verify/log files, captures status/diff/commits, writes `handoff.md`. `App.Recover` should reuse its shape but not its richer inputs.
- `internal/session/paths.go`: add one path accessor for `recovery.md`; keep all `.ai-session` path strings centralized here.
- `cmd/aictl/root.go`: add `newRecoverCmd(a)` alongside `newHandoffCmd(a)` and `newVerifyCmd(a)`.

### What must be preserved

- Existing `aictl handoff` behavior and golden tests must not change.
- Existing redaction behavior must stay in `internal/git`; do not add a second secret-filter implementation in handoff/app.
- Existing sparse-handoff behavior can remain tolerant of non-git directories, but `recover` should fail outside a git repo because its promise explicitly depends on branch + diff.
- No provider registry, PTY, transcript, or fallback code should be introduced in this story.
- No new external dependencies are needed.

### Recovery prompt content

Recommended sections:

1. `# Recover This Coding Task`
2. `Goal` - original Goal from Task State
3. `Current Repository State` - current branch and current redacted/bounded diff
4. `Recovery Instructions` - continue from current state; do not restart; inspect changed files before editing; preserve user work; run verification before finishing if known

Use `(no uncommitted changes)` if the diff is empty. Do not claim provider progress. The prompt is evidence-only: it may describe the current diff, but it must not say a provider completed anything.

### Testing guidance

- Use `t.TempDir()` + `t.Chdir(dir)` and the existing git helper style from `internal/git` / `internal/app` tests.
- Build test repos with real `git init`; stage tracked files when you need diff coverage on unborn repos.
- For redaction, add a tracked `.env` or `secret.pem` with known content and assert the content is absent while a redaction marker remains.
- For truncation, use a tiny `maxDiffChars` in `config.yaml` and assert the truncation marker exists.
- Run `go test ./...`; this story should not need provider CLIs or network.

### Previous Story Intelligence

- Story 2.3 review fixed context cancellation handling and append close-error handling. Carry that standard forward: every subprocess call must take `context.Context`, and every write/close error on recovery output must be surfaced by the helper used.
- Story 2.3 also deferred bounded verify/log output and content secret scanning. Recovery avoids those surfaces entirely by not embedding verify/log content; keep it that way.
- Story 2.2 review added status-path redaction and warnings for real git failures. Recovery should be stricter than handoff for git availability but should still keep warning/error messages honest: no silent "clean tree" when git capture fails.

### Latest Technical Information

- Web check on 2026-06-16: Go's current 1.26 minor is 1.26.4, which includes security fixes; the repo currently declares `go 1.26.0`. Do not change the Go directive in this story unless tests/build require it; dependency/toolchain hygiene is separate from `recover`.
- `github.com/spf13/cobra` v1.10.2 and `github.com/goccy/go-yaml` v1.19.2 are already in `go.mod`; recovery can use existing Cobra/YAML APIs and should not add dependencies.
- `github.com/creack/pty` is not needed for this story. Do not introduce PTY code until Epic 3.

### Project Structure Notes

- NEW: `internal/handoff/recover.go`, `internal/handoff/recover_test.go`, `internal/app/recover.go`, `internal/app/recover_test.go`, `cmd/aictl/recover.go`.
- MODIFIED: `internal/session/paths.go`, `cmd/aictl/root.go`.
- Optional test-only changes may touch existing app/root test files if that matches current local style.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 2.4: Last-resort recovery]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-14: recover]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md#Deterministic handoff template]
- [Source: _bmad-output/planning-artifacts/architecture.md#FR-11-14 Handoff + recover]
- [Source: internal/app/handoff.go]
- [Source: internal/git/git.go]
- [Source: internal/git/diff.go]
- [Source: internal/session/paths.go]
- [Source: _bmad-output/implementation-artifacts/2-3-verify-commands-feed-handoff.md#Review Findings]

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `go test ./internal/handoff` - passed after adding pure recovery renderer tests and implementation.
- `go test ./internal/app` - passed after adding `App.Recover`, recovery path accessor, and app-level recovery tests.
- `go test ./cmd/aictl` - passed after adding and registering the thin `recover` command.
- `go test ./...` - 91 passed across 9 packages.
- `go vet ./...` - no issues found.
- `go build ./...` - success.
- `golangci-lint run` - not run; `golangci-lint` is not installed in this environment.
- Review patch: `go test ./internal/handoff` - 9 passed.
- Review patch: `go test ./...` - 92 passed across 9 packages.
- Review patch: `go vet ./...` - no issues found.
- Review patch: `go build ./...` - success.

### Completion Notes List

Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented pure deterministic recovery prompt rendering with minimal inputs only: Goal, branch, and diff.
- Implemented `App.Recover` orchestration with strict original-goal handling, default config fallback, redacted/bounded git diff capture, and atomic `.ai-session/recovery.md` writes.
- Added `aictl recover` as a thin Cobra command delegating to `App.Recover`.
- Added recovery tests covering pure rendering, missing optional artifacts, malformed config fallback, outside-git failure, redaction/truncation, unavailable goals, command registration, and byte-identical repeated writes.
- Resolved review findings by dynamically sizing Markdown fences for arbitrary Goal and diff content, preventing embedded backticks from breaking prompt structure.

### File List

- `_bmad-output/implementation-artifacts/2-4-last-resort-recovery.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `cmd/aictl/recover.go`
- `cmd/aictl/root.go`
- `cmd/aictl/root_test.go`
- `internal/app/recover.go`
- `internal/app/recover_test.go`
- `internal/handoff/recover.go`
- `internal/handoff/recover_test.go`
- `internal/session/paths.go`

### Change Log

- 2026-06-16: Implemented Story 2.4 - `aictl recover` last-resort recovery prompt. Added pure minimal recovery renderer, app orchestration, centralized recovery path, Cobra command, and unit/integration coverage. Full Go regression suite passed.
- 2026-06-16: Addressed code review findings - 2 patch items resolved (dynamic Markdown fences for Goal and diff); 2 pre-existing items deferred.
