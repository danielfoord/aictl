---
baseline_commit: f4d3c56
context:
  - _bmad-output/implementation-artifacts/3-4-pre-post-run-checkpoints-durability-ordering.md
  - _bmad-output/implementation-artifacts/3-3-provider-adapters-trio-config-injection.md
  - _bmad-output/planning-artifacts/epics.md
  - _bmad-output/planning-artifacts/architecture.md
  - _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md
---

# Story 3.5: On-demand checkpoint (`aictl checkpoint`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to snapshot my repo and task context on demand,
so that I can capture a labeled recovery point any time.

## Acceptance Criteria

1. **Thin CLI command exists:** given an active Session, when I run `aictl checkpoint "<label>"`, the root command dispatches to a thin Cobra command in `cmd/aictl/checkpoint.go` and all business logic runs in `internal/app`. The command requires exactly one non-empty label argument. (FR-10)
2. **Labeled checkpoint artifact:** the command writes one labeled checkpoint directory under `.ai-session/checkpoints/` using the existing zero-padded sequence convention and a sanitized label in the directory name, e.g. `0005-checkpoint-before-risky-refactor/`. The directory contains at least `git-status.txt`, `git-diff.patch`, `recent-commits.txt`, `command-log.md`, and `summary.md`. (FR-10, NFR-4)
3. **Task context included:** the checkpoint summary captures the current Task State goal, next steps, decisions, known failures, and the user label. It may include `latest-verify.txt` if present, but must not regenerate or mutate `.ai-session/handoff.md`. (FR-4, FR-10, FR-18)
4. **Offline and non-invasive:** the command works with no provider running, invokes no provider CLI, makes no network/model calls, and never mutates the working tree, branch, or index. Git access is read-only shell-out through `internal/git`. (FR-10, NFR-1)
5. **Durable and redacted persistence:** every checkpoint file write goes through `session.WriteAtomic`; directory creation is durably synced; git status/diff evidence uses the existing denylist redaction and `maxDiffChars` truncation path. No session artifact writes use `os.WriteFile`. (NFR-3, NFR-4)
6. **Failure behavior is explicit:** without an initialized Session, the command returns `ErrNoSession` and writes nothing. Outside a git repo, it still writes a checkpoint with empty or unavailable git sections, consistent with existing handoff/checkpoint behavior. If persistence fails, it returns a clear wrapped error and does not claim success.

## Tasks / Subtasks

- [x] **Task 1 - Add on-demand checkpoint API to `internal/checkpoint` (AC: 2, 3, 5, 6)**
  - [x] Add a public type/API such as `ManualOptions` and `CaptureManual(ctx, opts) (Manual, error)` in `internal/checkpoint/checkpoint.go`.
  - [x] Reuse `CaptureGit`, `SafeProviderName`, `NextSequence`, `ensureDirDurable`, `filePerm`, and `session.WriteAtomic`; do not duplicate git capture or write logic.
  - [x] Add a new checkpoint directory phase/name that does not collide with run before/after directories. Recommended form: `NNNN-checkpoint-<label>`.
  - [x] Sanitize the label with the existing filesystem-safe logic or a small label-specific wrapper; preserve readable words, strip separators, and default to `checkpoint` only after validation prevents empty labels.
  - [x] Persist `git-status.txt`, `git-diff.patch`, `recent-commits.txt`, `command-log.md`, and `summary.md`. Include `latest-verify.txt` only if doing so does not introduce extra mutation or duplicate unclear state.
  - [x] Keep git-unavailable behavior non-fatal for `git.ErrNotARepo`; persist empty/unavailable git evidence and state that in `summary.md`.

- [x] **Task 2 - Add `App.Checkpoint` orchestration (AC: 1-6)**
  - [x] Add `internal/app/checkpoint.go` with `func (a *App) Checkpoint(ctx context.Context, label string) error`.
  - [x] Resolve `root := os.Getwd()` and `paths := session.NewPaths(root)` like other app methods.
  - [x] Require an existing `.ai-session/`; if absent, return `ErrNoSession` before creating directories or files.
  - [x] Load config via `config.Load(paths.Config())` for denylist and `handoff.maxDiffChars`; load Task State via `session.LoadState(paths.State())`.
  - [x] Read `command-log.md` and `latest-verify.txt` with the existing `readIfExists` helper.
  - [x] Call `checkpoint.CaptureManual` and print one concise success line through `a.UI`, e.g. `Wrote checkpoint 0005-checkpoint-label to <dir>`.
  - [x] Do not call `prepareHandoff`, `session.WriteAtomic(paths.Handoff())`, provider resolution, `exec.LookPath`, `openTranscript`, `shell.Run`, `UI.Mute`, or fallback/detector code.

- [x] **Task 3 - Add thin Cobra command (AC: 1, 4)**
  - [x] Add `cmd/aictl/checkpoint.go` with `Use: "checkpoint <label>"`, `Args: cobra.ExactArgs(1)`, and `RunE` calling `a.Checkpoint(cmd.Context(), args[0])`.
  - [x] Register `newCheckpointCmd(a)` in `cmd/aictl/root.go`.
  - [x] Keep command help terse and avoid implementation detail in user-facing text.
  - [x] Reject labels that trim to empty before entering app logic, or have `App.Checkpoint` return a wrapped validation error that root surfaces.

- [x] **Task 4 - Preserve existing run checkpoint behavior (AC: 2, 4, 5)**
  - [x] Ensure `NextSequence` scans the new manual checkpoint directory names when choosing the next sequence for any checkpoint type.
  - [x] Ensure manual checkpoint creation cannot reuse an existing `before`, `after`, or `checkpoint` sequence.
  - [x] Do not change `App.Run` ordering, transcript path, provider injection, quiet supervision, exit-code preservation, post-run checkpoint failure handling, or files-changed verdict logic.
  - [x] Keep `.ai-session/.gitignore` ignoring `checkpoints/`; manual checkpoints may contain sensitive raw diffs after redaction and are still transient/sensitive.

- [x] **Task 5 - Tests and verification (AC: 1-6)**
  - [x] Add `internal/checkpoint` tests for manual directory naming, sanitized label behavior, artifact set, sequence allocation alongside existing before/after directories, and git-unavailable summary content.
  - [x] Add `internal/app` tests proving `Checkpoint` requires a Session, writes expected artifacts with a label, uses redacted bounded git evidence, does not mutate `handoff.md`, and does not call `runProvider`.
  - [x] Add `cmd/aictl` tests proving root help/registers `checkpoint`, exact label argument enforcement, and end-to-end command output/artifact creation.
  - [x] Regression-check existing `App.Run` checkpoint tests remain green.
  - [x] Run `go test ./internal/checkpoint ./internal/app ./cmd/aictl`, `go test ./...`, `go vet ./...`, `go build ./...`, `go test -race ./internal/app ./internal/checkpoint`, and `GOOS=windows GOARCH=amd64 go build ./...`. Run `golangci-lint run` if installed.

## Dev Notes

**Fifth story of Epic 3.** Stories 3.1-3.4 established provider PTY execution, transcript capture, provider adapters, and durable before/after run checkpoints. This story adds the offline user command for a labeled checkpoint and should be a narrow extension of the existing checkpoint package and app command patterns.

> **Baseline:** Story 3.4 is committed at `f4d3c56`; this story should start from that checkpoint implementation.

### Scope Boundary

- **In scope:** `aictl checkpoint "<label>"`; labeled manual checkpoint directory; read-only git status/diff/commits capture; Task State summary; command log inclusion; config denylist and diff-size enforcement; root command registration; tests.
- **Out of scope:** provider runs, PTY/transcript changes, current `handoff.md` regeneration, usage-limit detection, exit-reason classification, fallback chain, attempts log, release docs, and any network/model integration.

### Reuse / Do Not Reinvent

- **Extend `internal/checkpoint`, do not create a second snapshot package.** Story 3.4 already added safe names, sequence allocation, git capture, durable directory sync, redacted evidence, and atomic artifact writes.
- **Use `session.WriteAtomic` for every manual checkpoint file.** The only acceptable exceptions are lock/append/live-stream patterns already documented elsewhere; this story has no live stream.
- **Use `checkpoint.CaptureGit` for git evidence.** It already shells out through `internal/git`, applies denylist redaction and diff truncation, and treats `git.ErrNotARepo` as unavailable.
- **Use `session.Paths` for every `.ai-session` path.** Add helpers only if the current `CheckpointDir` / `CheckpointFile` helpers are insufficient.
- **Keep `cmd/aictl` thin.** The Cobra command should parse one label argument and call `App.Checkpoint`; it should not know checkpoint file names or git details.
- **Keep `handoff.Generate` and `prepareHandoff` out of this path.** A manual checkpoint is a snapshot, not a handoff regeneration.

### Current Files to Modify (read in full before editing)

- `internal/checkpoint/checkpoint.go` - currently supports `CapturePre`, `CapturePost`, `CaptureGit`, `NextSequence`, `SafeProviderName`, durable directory creation, and `DetermineFilesChanged`. **What changes:** add manual checkpoint capture and teach sequence scanning/name parsing to include manual checkpoint directories. **Preserve:** before/after run directory names, files-changed semantics, redaction/truncation path, post-run git degradation behavior, and directory fsync.
- `internal/app/handoff.go` - currently contains `readIfExists` and `ensureRunSession`; `App.Run` uses `prepareHandoff`. **What changes:** likely none except reusing `readIfExists` from new `internal/app/checkpoint.go`. **Preserve:** handoff generation behavior and lazy run setup.
- `internal/app/run.go` - current `App.Run` persists handoff, captures pre-run checkpoint, validates executable, writes live transcript under after-checkpoint, runs provider, captures post-run checkpoint, and preserves exit-code semantics. **What changes:** none expected. **Preserve:** all existing run behavior and tests.
- `internal/session/paths.go` - central `.ai-session` path source. **What changes:** only add helper(s) if manual checkpoint naming needs them. **Preserve:** no ad hoc path joins outside owning helpers where a helper already exists.
- `internal/app/app.go` - App dependency container. **What changes:** none expected; do not add new global dependencies.
- `cmd/aictl/root.go` - registers subcommands. **What changes:** add `newCheckpointCmd(a)`.
- `cmd/aictl/checkpoint.go` - new thin command file for FR-10.
- Tests: `internal/checkpoint/checkpoint_test.go`, `internal/app/*checkpoint*_test.go` or `internal/app/run_test.go`, and `cmd/aictl/root_test.go` / new command tests.

### Existing Behavior That Must Not Regress

- `aictl run <provider>` still creates matching `NNNN-before-<provider>/` and `NNNN-after-<provider>/` directories.
- Missing provider executable still fails before raw mode, transcript, and runner side effects.
- Provider nonzero exits still return `ExitError{Code}` after post-run checkpointing.
- Post-run checkpoint failure still preserves provider exit-code information through `PostRunError`.
- Unknown provider names still run as bare executables without injection.
- Config-defined providers still resolve through the same registry as built-ins.
- UI remains muted only during provider execution; manual checkpoint never mutes UI or touches terminal raw mode.
- Secret denylist behavior remains centralized in `internal/git`/`checkpoint.CaptureGit`.
- No network imports or model calls are introduced.

### Edge Cases That Must Not Be Missed

- **Empty/whitespace label:** reject before writing anything. Do not silently create `NNNN-checkpoint-checkpoint/` for a user-provided blank label.
- **Path-like label:** `../save me` must not escape `checkpoints/`; sanitize to a readable directory component.
- **Duplicate label:** second call with the same label must get a new sequence and must not overwrite the earlier checkpoint.
- **Mixed existing dirs:** if `0001-before-claude`, `0001-after-claude`, and `0002-checkpoint-x` exist, the next manual checkpoint should be `0003-checkpoint-...`.
- **Persistence failure after directory reserve:** return an error and do not print success. It is acceptable to leave a partial checkpoint directory for forensic evidence, matching fail-closed checkpoint behavior.
- **Outside git repo:** write a checkpoint with unavailable git evidence rather than failing, consistent with `CaptureGit` and handoff behavior for non-repo contexts.
- **Tracked secret paths:** checkpoint artifacts must not reveal denylisted paths or diff bodies.
- **Large diff:** diff must be truncated using configured `maxDiffChars` with the existing explicit truncation marker.
- **Existing `handoff.md`:** command must not modify it; test by seeding a sentinel handoff and comparing after checkpoint.
- **Windows build:** do not introduce Unix-only imports in unguarded files. This feature should be plain filesystem/git orchestration.

### Testing Guidance

- Use `t.Chdir(t.TempDir())` in app/command tests; initialize sessions with `Start` or `Init` depending on whether a goal is needed.
- For git evidence tests, initialize a real temp git repo and set user config before committing, following existing `runGit` / `gitInit` test helpers.
- Assert artifact files by path and contents: label in `summary.md`, redacted diff behavior, and unchanged `handoff.md`.
- Assert `runProvider` is not called by replacing it with a test function that fails the test if invoked.
- Prefer exact root command output assertions only for stable concise messages; avoid brittle absolute temp paths unless necessary.

### Previous Story Intelligence

- Story 3.4 added `internal/checkpoint` and proved several failure modes in review. Carry forward these fixes: directory entries must be fsync'd, sequence allocation must avoid collisions, post-run git capture failures must degrade to unknown evidence instead of blocking artifact persistence, and provider exit-code semantics must survive checkpoint errors.
- Story 3.4 deliberately left `aictl checkpoint "<label>"` for this story. Do not pull Epic 4 fallback/attempt recording into this implementation.
- Story 3.4 moved live transcripts into after-checkpoint directories. Manual checkpoints have no live transcript and should not create `transcript.ansi`.
- Story 3.2's quiet-supervision constraints matter only during provider runs; manual checkpoint can print a concise success line immediately because no provider owns the TTY.
- App tests already use `a.runProvider` as the runner seam. Use that seam to prove manual checkpoint does not launch providers.

### Git Intelligence Summary

- Recent commits show a clean progression: durable pre/post checkpoints (`f4d3c56`), provider registry/prompt injection (`cd4e78c`), raw transcript capture (`ce74d56`), PTY runner (`09fd5ae`), and recovery prompt (`6514afa`).
- The last checkpoint commit created `internal/checkpoint/checkpoint.go` and focused tests rather than spreading logic across `cmd/`; Story 3.5 should follow the same layering.
- The previous implementation touched `_bmad-output/implementation-artifacts/sprint-status.yaml`; keep story status updates separate from product code changes during implementation.

### Latest Technical Information

- Official Go downloads list `go1.26.4` as the stable version on 2026-06-16, while this repo currently declares `go 1.26.0` in `go.mod`. Do not update the toolchain in this story unless requested; no Story 3.5 requirement depends on a newer Go patch release. Source: https://go.dev/dl/
- Local dependencies are already pinned: Cobra `v1.10.2`, `creack/pty v1.1.24`, `goccy/go-yaml v1.19.2`, and `golang.org/x/term v0.44.0`. `aictl checkpoint` should need no new dependency. Sources: https://pkg.go.dev/github.com/spf13/cobra, https://pkg.go.dev/github.com/creack/pty, https://pkg.go.dev/github.com/goccy/go-yaml, https://pkg.go.dev/golang.org/x/term
- `creack/pty` remains irrelevant to this story except as behavior to avoid disturbing; no manual checkpoint path should import or touch PTY/raw-mode packages.

### Project Structure Notes

- NEW: `cmd/aictl/checkpoint.go`.
- NEW: `internal/app/checkpoint.go`.
- MODIFIED: `cmd/aictl/root.go`, `internal/checkpoint/checkpoint.go`, focused tests.
- POSSIBLY MODIFIED: `internal/session/paths.go` only for helper clarity.
- No changes expected in `internal/shell`, `internal/providers`, `internal/fallback`, `internal/handoff`, or provider adapter code.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.5: On-demand checkpoint (`aictl checkpoint`)]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-10: On-demand Checkpoint]
- [Source: _bmad-output/planning-artifacts/architecture.md#Data Architecture - Session state & on-disk layout]
- [Source: _bmad-output/planning-artifacts/architecture.md#Architecture and Coding Patterns]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure]
- [Source: _bmad-output/planning-artifacts/architecture.md#Requirements to Structure Mapping]
- [Source: _bmad-output/implementation-artifacts/3-4-pre-post-run-checkpoints-durability-ordering.md#Previous Story Intelligence]
- [Source: internal/checkpoint/checkpoint.go]
- [Source: internal/app/run.go]
- [Source: internal/app/handoff.go]
- [Source: internal/session/paths.go]
- [Source: internal/session/store.go]
- [Source: cmd/aictl/root.go]

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- RED: `go test ./internal/checkpoint` failed on undefined `CaptureManual` and `ManualOptions`.
- GREEN: `go test ./internal/checkpoint` - 16 passed.
- RED: `go test ./internal/app` failed on missing `App.Checkpoint`.
- GREEN: `go test ./internal/app` - 57 passed.
- RED: `go test ./cmd/aictl` failed because `checkpoint` was not registered.
- GREEN: `go test ./cmd/aictl` - 13 passed.
- GREEN: `go test ./internal/checkpoint ./internal/app ./cmd/aictl` - 86 passed.
- GREEN: `go test ./...` - 164 passed.
- GREEN: `go vet ./...` - no issues.
- GREEN: `go build ./...` - success.
- GREEN: `go test -race ./internal/app ./internal/checkpoint` - 73 passed.
- GREEN: `GOOS=windows GOARCH=amd64 go build ./...` - success.
- NOT RUN: `golangci-lint run` - `golangci-lint` is not installed in this environment.

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented `checkpoint.CaptureManual` with sanitized labeled checkpoint directories, durable atomic artifact writes, optional latest verify capture, and non-fatal git-unavailable summaries.
- Added `App.Checkpoint` orchestration that requires an existing Session, loads config and Task State, captures manual checkpoints without touching handoff/provider run paths, and reports a concise success line.
- Added the thin `aictl checkpoint <label>` Cobra command and root registration with command-level coverage.
- Preserved existing provider-run checkpoint behavior; targeted `TestRun*` regressions and full-suite checks remain green.
- Added checkpoint, app, and root command tests for manual directory naming, artifact sets, blank-label rejection, session requirements, secret redaction, unchanged handoff behavior, and command dispatch.

### File List

- `_bmad-output/implementation-artifacts/3-5-on-demand-checkpoint.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `cmd/aictl/checkpoint.go`
- `cmd/aictl/root.go`
- `cmd/aictl/root_test.go`
- `internal/app/checkpoint.go`
- `internal/app/checkpoint_test.go`
- `internal/checkpoint/checkpoint.go`
- `internal/checkpoint/checkpoint_test.go`

### Review Findings

- [x] [Review][Patch] Infinite loop in `reserveManualCheckpointDir` — no max-attempts guard in retry loop [`internal/checkpoint/checkpoint.go`]
- [x] [Review][Patch] Error message preserves `os.ErrExist` via `%w` wrapping — already handled [`internal/checkpoint/checkpoint.go`]
- [x] [Review][Patch] `.ai-session` path not validated as directory — added `!info.IsDir()` check [`internal/app/checkpoint.go:28-33`]
- [x] [Review][Patch] `CaptureGit` non-`ErrNotARepo` error causes total failure — degraded gracefully [`internal/checkpoint/checkpoint.go`]
- [x] [Review][Defer] No input size limits on CommandLog/LatestVerify (pre-existing pattern) — deferred, pre-existing
- [x] [Review][Defer] `strings.Contains` could match substrings in future phase constants — deferred, pre-existing
- [x] [Review][Defer] `ManualOptions.Git` field lets callers bypass denylist (consistent with `PreOptions` pattern) — deferred, pre-existing
- [x] [Review][Defer] No state lock during concurrent mutation (pre-existing codebase pattern) — deferred, pre-existing

### Review Findings — independent adversarial review (2026-06-16)

_Blind Hunter + Edge Case Hunter + Acceptance Auditor, clean diff vs `f4d3c56`. AC1–AC6 all audited as satisfied with solid coverage. Most Blind-Hunter "Critical/High" items were verified false positives by the code-access layers (path traversal: `SafeProviderName` strips all separators; `WriteAtomic` already `fsyncDir`s; `App.Checkpoint` checks every error)._

**Patch**

- [x] [Review][Patch] Out-of-scope, untested `CapturePre` fail-closed → fail-degrade change [internal/checkpoint/checkpoint.go:209-214] — FIXED: reverted to `return Pre{}, err` (fail-closed before a run, NFR-3). `CaptureManual` keeps its own in-scope offline degradation. Full suite still green (no test depended on the degradation).
- [x] [Review][Patch] No length bound on the sanitized checkpoint label [internal/checkpoint/checkpoint.go `SafeLabelName`] — FIXED: `SafeLabelName` is now capped at `maxLabelNameLen` (80) with a re-trim; covered by `TestSafeLabelNameBounds`.

**Deferred**

- [x] [Review][Defer] Checkpoint dir `0o755` / files `filePerm` (≈`0o644`) vs the transcript's `0o600` [internal/checkpoint/checkpoint.go] — the redacted `git-diff.patch` can still hold sensitive non-denylisted content. Pre-existing Story 3.4 perm posture (shared with run before/after checkpoints; `checkpoints/` is gitignored). Tighten checkpoint perms as a broader follow-up.
- [x] [Review][Defer] Nondeterministic write order + partial-checkpoint dir on mid-write failure [internal/checkpoint/checkpoint.go `CaptureManual`] — the spec explicitly accepts leaving a partial checkpoint dir for forensics. Deterministic (ordered-slice) writes and/or a completion marker would make a partial dir distinguishable from a complete one; deferred as a minor robustness improvement.

_Dismissed (verified): path-traversal via label (separators stripped), `App.Checkpoint` error discarding (real code checks all errors), `MaxDiffChars==0` (config normalized), `WriteAtomic` dir durability (it `fsyncDir`s), Windows reserved names (prefix neutralizes), `requestedSeq>0` "dead loop" (intentional, matches 3.4), `os.Getwd` subdir resolution (pre-existing deferred repo-root gap), degenerate label → `checkpoint-checkpoint` (success line echoes the effective name), run-path regression (App.Run still fails closed)._

## Change Log

- 2026-06-16: Created Story 3.5 context with artifact analysis, architecture guardrails, previous-story intelligence, implementation tasks, and test guidance. Status set to ready-for-dev.
- 2026-06-16: Implemented Story 3.5 - on-demand labeled checkpoint API, app orchestration, Cobra command, focused tests, and validation suite. Status set to review.
- 2026-06-16: Code review completed. 4 patches applied (max-attempts guard, IsDir validation, CaptureGit degradation), 4 deferred, 7 dismissed. Status set to done.
- 2026-06-16: Independent adversarial review (3 layers). 2 further patches applied — reverted the out-of-scope `CapturePre` fail-closed→degrade change (restored NFR-3 fail-closed before a run); bounded `SafeLabelName` length (+`TestSafeLabelNameBounds`). 2 deferred (checkpoint perms, partial-checkpoint marker). ~10 dismissed as verified false positives. Status set to done.
