---
baseline_commit: cd4e78c
context:
  - _bmad-output/implementation-artifacts/3-3-provider-adapters-trio-config-injection.md
  - _bmad-output/implementation-artifacts/3-2-faithful-transcript-and-quiet-supervision.md
  - _bmad-output/implementation-artifacts/2-2-generate-deterministic-handoff.md
  - _bmad-output/implementation-artifacts/2-1-capture-faithful-git-state.md
---

# Story 3.4: Pre/post-run checkpoints with durability ordering

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want aictl to checkpoint my repo before and after each provider run, persisting state before the provider starts,
so that my progress survives even instant provider failure.

## Acceptance Criteria

1. **Pre-run handoff and checkpoint before provider start:** given a provider about to run, when `aictl run <provider>` executes, `handoff.md` and a pre-run checkpoint are persisted and fsync'd before the provider process starts. The pre-run checkpoint is written under `.ai-session/checkpoints/NNNN-before-<provider>/` and includes at least `handoff.md`, `git-status.txt`, `git-diff.patch`, `recent-commits.txt`, `command-log.md`, and `summary.md`. (FR-8, FR-11, FR-12, NFR-3)
2. **Fail-closed on pre-run persistence failure:** if handoff generation or any pre-run checkpoint persistence fails, the provider is not launched, raw mode is not entered, no transcript is opened, and `App.Run` returns a clear error. This ordering must be testable with a stubbed runner that remains uncalled. (NFR-3)
3. **Post-run checkpoint after every provider exit:** given a provider exits normally, nonzero, or instantly, when the run ends, a post-run checkpoint is written under `.ai-session/checkpoints/NNNN-after-<provider>/` and includes at least `exit-code.txt`, `transcript.ansi`, `git-status.txt`, `git-diff.patch`, and a transcript reference/metadata file. The post-run checkpoint is attempted after `shell.Run` returns and before the final post-run UI line. (FR-9)
4. **Files-changed verdict:** after the post-run capture, `aictl` computes and reports whether files changed during this run by comparing pre-run and post-run git state. The verdict must be persisted in the post-run checkpoint and printed in the concise post-run status line. Use evidence from git artifacts only; do not infer provider progress from transcript text. (FR-9, FR-21 guardrail)
5. **Preserve TUI and quiet-supervision behavior:** while the provider owns the screen, aictl emits no terminal output; pre-run checkpoint/handoff status may print before raw mode, and post-run status prints only after the provider exits. Existing transcript capture, provider exit-code propagation, prompt injection, terminal restore, resize handling, and unknown-provider bare-exec fallback must keep working. (FR-5, FR-6, FR-7, NFR-2, NFR-5)
6. **Scope boundary:** this story does not implement usage-limit stream detection, exit-reason classification, fallback-chain advancement, attempts.log, on-demand `aictl checkpoint`, or release docs. Those remain Story 3.5 / Epic 4 scope.

## Tasks / Subtasks

- [x] **Task 1 - Add checkpoint path and numbering helpers (AC: 1, 3)**
  - [x] Extend `internal/session/paths.go` with helpers for checkpoint directories and files; keep `.ai-session/` path construction centralized.
  - [x] Implement deterministic next sequence selection by scanning `paths.Checkpoints()` for existing `NNNN-before-*` / `NNNN-after-*` directories and choosing the next zero-padded sequence. Do not rely on timestamps for ordering.
  - [x] Sanitize provider names for directory names; keep readable names but avoid path separators or shell-sensitive characters. Unknown bare executables should still produce a safe checkpoint name.
  - [x] Ensure `aictl init` still creates `checkpoints/`, and `run` without prior `init` creates the session/checkpoints directories plus `.gitignore` without destroying existing state.

- [x] **Task 2 - Create `internal/checkpoint` package (AC: 1, 3, 4)**
  - [x] Add `internal/checkpoint/checkpoint.go` with a small API, e.g. `CapturePre(ctx, paths, providerName, handoffMarkdown, gitSnapshot, taskState) (Pre, error)` and `CapturePost(ctx, paths, pre, exitCode, transcriptBytesOrPath, gitSnapshot) (Post, error)`. Prefer simple value types over global state.
  - [x] Use existing git functions: `git.Status`, `git.Diff`, `git.RecentCommits`, and `git.RedactStatusPaths`; pass `cfg.Denylist` and `cfg.Handoff.MaxDiffChars`.
  - [x] Use `session.WriteAtomic` for every whole-file session artifact: checkpoint files, current `handoff.md`, copied transcript metadata, verdict files. Do not use `os.WriteFile` for session artifacts.
  - [x] Create directories with `os.MkdirAll`, and make directory creation failure fail the run before provider launch for the pre-run path.
  - [x] Persist pre-run files: `handoff.md`, `git-status.txt`, `git-diff.patch`, `recent-commits.txt`, `command-log.md`, `summary.md`. `summary.md` can be deterministic text from task state and git capture, not an LLM summary.
  - [x] Persist post-run files: `exit-code.txt`, `git-status.txt`, `git-diff.patch`, `transcript.ansi`, `transcript-ref.txt` or equivalent metadata, and `files-changed.txt` / verdict metadata.

- [x] **Task 3 - Factor reusable handoff preparation (AC: 1, 2)**
  - [x] Refactor `internal/app/handoff.go` so `App.Handoff` and `App.Run` share one internal helper that loads config, task state, git status/diff/commits, verify output, command log, and calls pure `handoff.Generate`.
  - [x] Preserve `handoff.Generate` as pure; do not add I/O, clocks, randomness, or git/config/session imports to `internal/handoff`.
  - [x] Preserve existing `App.Handoff` behavior: requires an initialized session, best-effort non-repo git capture, redacted status/diff, bounded diff, and writes current `.ai-session/handoff.md` via `session.WriteAtomic`.
  - [x] In `App.Run`, write the generated current `.ai-session/handoff.md` via `session.WriteAtomic` before resolving or starting the provider's PTY, then copy the same markdown into the pre-run checkpoint.

- [x] **Task 4 - Rewire `App.Run` ordering (AC: 1, 2, 3, 5)**
  - [x] Current flow in `internal/app/run.go` is: `os.Getwd` -> `session.NewPaths` -> `config.Load` -> `providers.Resolve`/`resolveLaunch` -> `exec.LookPath` -> `openTranscript` -> `shell.Run`. Change it to: load config/state -> generate and persist handoff -> capture and persist pre-run checkpoint -> resolve provider/injection -> `exec.LookPath` -> open transcript in the post-run checkpoint path -> `shell.Run` -> capture post-run checkpoint -> final UI/exit handling.
  - [x] Keep the missing executable fail-before-side-effects rule for provider execution. Handoff/pre-run checkpoint creation is now intentional pre-run side effect, but raw mode, transcript stream, and provider launch still must not happen when `exec.LookPath` fails.
  - [x] Keep `resolveLaunch` semantics exactly: registry-resolved providers inject according to config; unknown names run as bare executables with no injection.
  - [x] Keep `a.UI.Mute()`, deferred `Flush()`, inline `Flush()`, and `ExitError{Code}` propagation. Add only concise pre/post lines outside the muted provider interval.
  - [x] Ensure post-run checkpoint is attempted even when the provider exits nonzero. If post-run checkpoint persistence fails after a provider run, return that error without losing the provider exit code in tests; document and test the chosen error precedence.

- [x] **Task 5 - Move transcript into checkpoint artifact location (AC: 3, 5)**
  - [x] Replace or adapt `openTranscript(paths)` so the live transcript writer targets `.ai-session/checkpoints/NNNN-after-<provider>/transcript.ansi` for this run, with `0o600` permissions.
  - [x] Keep `.ai-session/transcript.ansi` compatibility only if the implementation deliberately needs a "latest transcript" alias; if kept, write/copy it through `session.WriteAtomic` after run, never as the only artifact.
  - [x] Preserve the `.ai-session/.gitignore` entries for `transcript.ansi` and `checkpoints/`; do not make raw provider output commit-friendly.

- [x] **Task 6 - Files-changed verdict (AC: 4)**
  - [x] Define the comparison explicitly in `internal/checkpoint`: compare pre/post captured git status and diff artifacts after redaction for persisted evidence, and add a conservative guard so secret-redacted file changes are not incorrectly reported as "no changes" solely because bodies are redacted.
  - [x] If git capture fails outside a repo, persist an "unknown" or false-with-warning verdict consistently; do not claim files changed or unchanged without git evidence.
  - [x] Include the verdict in post-run UI, e.g. `Provider <name> exited with code <n> (files changed: yes|no|unknown)`.
  - [x] Do not inspect transcript text to infer changes or progress.

- [x] **Task 7 - Tests and verification (AC: 1-6)**
  - [x] Add `internal/checkpoint` unit tests for sequence numbering, safe provider directory names, pre-run artifact set, post-run artifact set, and changed/no-changed verdict cases.
  - [x] Add `internal/app` tests proving fail-closed ordering: pre-run persistence failure prevents runner call; missing executable prevents runner call; successful run creates before/after directories and passes transcript writer to the runner.
  - [x] Update existing `internal/app/run_test.go` expectations from latest `.ai-session/transcript.ansi` as the primary artifact to the post-run checkpoint transcript path. Keep quiet-supervision, panic flush, provider injection, unknown-name fallback, and typed `ExitError` tests.
  - [x] Add a nonzero/instant-exit test proving post-run checkpoint and exit-code file are still written.
  - [x] Run `go test ./...`, `go vet ./...`, `go build ./...`, `go test -race ./internal/app ./internal/checkpoint ./internal/shell ./internal/providers`, and `GOOS=windows GOARCH=amd64 go build ./...`. Run `golangci-lint run` if installed.

### Review Findings

- [x] [Review][Patch] Checkpoint directory creation is not durably synced [internal/checkpoint/checkpoint.go:199]
- [x] [Review][Patch] Pre-run git baseline is captured before the handoff write, causing false files-changed verdicts [internal/app/run.go:38]
- [x] [Review][Patch] Transcript is not closed if the provider runner panics [internal/app/run.go:88]
- [x] [Review][Patch] Post-run git capture failure can prevent after-checkpoint persistence [internal/checkpoint/checkpoint.go:221]
- [x] [Review][Patch] Post-checkpoint failure can hide provider exit-code/error semantics [internal/app/run.go:107]
- [x] [Review][Patch] Files-changed verdict ignores commit-only provider changes [internal/checkpoint/checkpoint.go:260]
- [x] [Review][Patch] Checkpoint sequence allocation can collide or reuse names [internal/checkpoint/checkpoint.go:112]

## Dev Notes

**Fourth story of Epic 3.** This story wires the durable pre/post-run state that Stories 3.1-3.3 deliberately deferred. The implementation must leave the provider TUI behavior unchanged while adding a fail-closed pre-run durability phase and evidence-backed post-run checkpointing.

> **Baseline:** Story 3.3 is committed at `cd4e78c`; that is this story's clean diff baseline.

### Scope Boundary

- **In scope:** pre-run handoff generation inside `run`; current `.ai-session/handoff.md` persistence before provider start; pre-run checkpoint directory and files; post-run checkpoint directory and files; per-run transcript under checkpoint; files-changed verdict; concise pre/post UI.
- **Out of scope:** `aictl checkpoint "<label>"` (Story 3.5), usage-limit detection/classification (Story 4.1), fallback chain (Story 4.2), attempts log/evidence rule enforcement beyond not claiming progress (Story 4.3), distribution/docs (Story 4.4).

### Reuse / Do Not Reinvent

- **Use `session.WriteAtomic` for all checkpoint and handoff whole-file writes.** It already implements temp + fsync + rename + parent directory fsync. Do not bypass it with `os.WriteFile` for session artifacts.
- **Keep `handoff.Generate` pure.** It already accepts `handoff.Input`; callers assemble state. Do not put filesystem, git, time, or config reads in `internal/handoff`.
- **Reuse `git.Status`, `git.Diff`, `git.RecentCommits`, and `git.RedactStatusPaths`.** `git.Diff` is already the single secret-redaction and truncation point for diff contents.
- **Reuse provider resolution and injection from Story 3.3.** `resolveLaunch(providers.Resolve(cfg), name, paths.Handoff(), userArgs)` is the intended launch seam. Do not special-case built-in vs config providers.
- **Extend `openTranscript` rather than duplicating transcript logic.** It currently creates a live stream because transcript capture is not a whole-buffer atomic write; this remains valid. The target path changes to the per-run post checkpoint.
- **Keep `cmd/aictl/run.go` thin.** Business logic stays in `internal/app` / `internal/checkpoint`.

### Current Files to Modify (read in full before editing)

- `internal/app/run.go` - current `App.Run` resolves provider/injection, checks `exec.LookPath`, opens `.ai-session/transcript.ansi`, mutes UI during `shell.Run`, preserves exit code via `ExitError`. **What changes:** add pre-run handoff/checkpoint before launch; move transcript target to post checkpoint; capture post-run checkpoint; print files-changed verdict. **Preserve:** quiet supervision, missing executable before raw mode, injection semantics, transcript writer plumbing, typed exit error.
- `internal/app/handoff.go` - current `App.Handoff` assembles `handoff.Input` and writes `.ai-session/handoff.md`. **What changes:** factor shared preparation for run. **Preserve:** best-effort non-repo git capture and warnings for real git errors; config/state loading; redacted status and bounded diff.
- `internal/session/paths.go` - centralized `.ai-session` paths. **What changes:** add checkpoint path helpers. **Preserve:** no ad hoc `.ai-session/...` joins elsewhere.
- `internal/session/store.go` - atomic durable write helper. **What changes:** likely none. **Preserve:** single write path.
- `internal/git/git.go` / `internal/git/diff.go` - git capture and redaction. **What changes:** maybe add a helper for snapshot comparison if needed. **Preserve:** shell-out git, `ErrNotARepo`, denylist filtering, redaction before truncation.
- `internal/app/run_test.go` - current tests assert provider resolution, injection, transcript creation, quiet supervision, missing provider, exit-code propagation. **What changes:** update transcript path expectations and add ordering/checkpoint tests. **Preserve:** all behavioral coverage from Stories 3.1-3.3.

### Existing Behavior That Must Not Regress

- Unknown provider name runs as a bare executable with no prompt injection.
- Config-defined providers resolve through the same registry as built-ins; config can override built-in fields.
- Invalid configured injection mode fails before the runner is called.
- Missing provider executable fails before raw mode/transcript/runner side effects.
- `paste` / `stdin` modes write `InitialInput` into the PTY best-effort after launch.
- UI is muted while the provider owns the screen and flushed on normal return or panic.
- Nonzero provider exit returns `ExitError{Code}` so `main` can preserve the provider's exit code.
- No package writes directly to the user's terminal during the provider run except the shell fan-out path.
- No network imports or model calls.

### Edge Cases That Must Not Be Missed

- **Provider exits instantly:** pre-run checkpoint must already exist; post-run checkpoint still writes `exit-code.txt` and git artifacts.
- **Pre-run write failure:** provider must not launch. This includes failures writing current `handoff.md`, the before checkpoint directory/files, and any required session `.gitignore`.
- **Post-run write failure:** do not hide that failure. Tests must define whether `ExitError` or checkpoint persistence error takes precedence when both occur.
- **Outside a git repo:** `App.Handoff` currently treats `git.ErrNotARepo` as empty git sections. Run checkpointing should be consistent and must not crash.
- **No prior `aictl init`:** Story 3.2 allowed `run` to lazily create `.ai-session/` for transcripts. Story 3.4 now needs state for handoff generation; choose deterministic behavior. Preferred: require an initialized/started session for checkpointed run because handoff needs Task State. If keeping lazy run, create default state/config safely and document how an empty Goal appears in handoff.
- **Secret denylist:** persisted status/diff artifacts must not reveal denylisted paths or bodies. Do not persist raw unredacted diff artifacts in checkpoints.
- **Secret-only file modifications:** redacted diff bodies may obscure content differences. The files-changed verdict must avoid a false "no" when only redacted file contents changed.
- **Sequence collisions:** back-to-back runs must create `0001`, `0002`, etc.; partial before/after directories from a failed run must not be overwritten.
- **Windows build:** PTY runtime is Unix, but the project still currently builds for Windows using platform files; do not introduce Unix-only imports outside guarded files.

### Testing Guidance

- Use `t.Chdir(t.TempDir())` for every app test that calls `Run`; run writes session artifacts.
- Prefer stubbing `a.runProvider` to assert `shell.Options` and ordering instead of launching real CLIs.
- For checkpoint package tests, create a temporary git repo where needed using the real `git` binary; otherwise test pure path/sequence/verdict helpers without git.
- Preserve race-clean behavior for shell/app paths; Story 3.1 review made `-race` part of the gate for run changes.
- Keep tests independent of real `claude`, `codex`, or `gemini` binaries.

### Previous Story Intelligence

- Story 3.3 implemented `internal/providers` as resolved values, not interfaces. `Provider.Inject(handoffPath, userArgs)` returns final args plus optional `InitialInput`.
- Story 3.3 explicitly deferred pre-run handoff generation, pre/post checkpoints, usage-limit detection, fallback, and attempts log. Do not pull Epic 4 scope forward.
- Story 3.2 made transcript capture a live stream and quiet supervision structural via `UI.Mute()` / `Flush()`. Keep this instead of buffering provider output.
- Story 3.1 established fail-before-side-effects for missing provider executables and terminal restore as release-blocker behavior.
- App tests already use `a.runProvider` as the seam; continue using that seam for order and option assertions.

### Latest Technical Information

- Local module versions are already pinned in `go.mod`: Go `1.26.0`, `github.com/spf13/cobra v1.10.2`, `github.com/creack/pty v1.1.24`, `github.com/goccy/go-yaml v1.19.2`, and `golang.org/x/term v0.44.0`. No dependency change is needed for this story.
- I attempted live web verification for latest upstream versions during story creation, but no searchable results were returned in this environment. Treat the architecture-approved and locally pinned versions as authoritative for this story unless a maintainer deliberately updates dependencies.

### Project Structure Notes

- NEW: `internal/checkpoint/checkpoint.go` and focused `*_test.go`.
- MODIFIED: `internal/app/run.go`, `internal/app/handoff.go`, `internal/app/run_test.go`, `internal/session/paths.go`.
- POSSIBLY MODIFIED: `internal/git/*` only if a comparison helper is needed; `internal/app/init.go` only if `.gitignore` or checkpoints setup must be adjusted.
- No `internal/fallback`, `internal/checkpoint` CLI command, detector, classifier, or attempts log in this story.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.4: Pre/post-run checkpoints with durability ordering]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-8]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-9]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-11]
- [Source: _bmad-output/planning-artifacts/architecture.md#Data Architecture - Session state & on-disk layout]
- [Source: _bmad-output/planning-artifacts/architecture.md#Integration Points]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md#Session Directory contents]
- [Source: internal/app/run.go]
- [Source: internal/app/handoff.go]
- [Source: internal/session/store.go]
- [Source: internal/session/paths.go]
- [Source: internal/git/git.go]
- [Source: internal/git/diff.go]
- [Source: _bmad-output/implementation-artifacts/3-3-provider-adapters-trio-config-injection.md#Previous Story Intelligence]

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- RED: `go test ./internal/session ./internal/checkpoint` failed on missing `CheckpointDir`, `CheckpointFile`, and checkpoint package APIs.
- RED: `go test ./internal/app -run 'TestRun(WritesTranscriptFile|CreatesPreAndPostCheckpoints|PreRunPersistenceFailurePreventsRunner|NonzeroExitStillWritesPostCheckpoint)'` failed because the current run path still wrote the old transcript path and did not create checkpoints.
- GREEN: `go test ./internal/session ./internal/checkpoint` - 18 passed.
- GREEN: `go test ./internal/app` - 52 passed.
- GREEN: `go test ./...` - 149 passed.
- GREEN: `go vet ./...` - no issues.
- GREEN: `go build ./...` - success.
- GREEN: `go test -race ./internal/app ./internal/checkpoint ./internal/shell ./internal/providers` - 86 passed.
- GREEN: `GOOS=windows GOARCH=amd64 go build ./...` - success.
- GREEN: post-review `go test ./internal/app ./internal/checkpoint ./internal/session ./cmd/aictl` - 85 passed.
- GREEN: post-review `go test ./...` - 154 passed.
- GREEN: post-review `go vet ./...` - no issues.
- GREEN: post-review `go build ./...` - success.
- GREEN: post-review `go test -race ./internal/app ./internal/checkpoint ./internal/shell ./internal/providers` - 91 passed.
- GREEN: post-review `GOOS=windows GOARCH=amd64 go build ./...` - success.
- NOT RUN: `golangci-lint run` - `golangci-lint` is not installed in this environment.

### Completion Notes List

- Added centralized checkpoint path helpers and a new `internal/checkpoint` package for safe provider names, deterministic sequence selection, redacted git evidence capture, before/after artifact persistence, and conservative files-changed verdicts.
- Refactored app handoff preparation so `App.Handoff` and `App.Run` share config/state/git/handoff generation while preserving pure `handoff.Generate` and existing handoff behavior.
- Rewired `App.Run` to persist `.ai-session/handoff.md`, create the before checkpoint, launch the provider with the existing registry/injection behavior, stream transcript into the after checkpoint, capture post-run artifacts, and print the files-changed verdict after provider exit.
- Preserved quiet supervision, typed provider exit-code propagation, missing executable fail-before-runner behavior, unknown-provider bare execution, and lazy run setup for `.ai-session/`, `state.yaml`, `.gitignore`, and `checkpoints/`.
- Addressed review findings by making checkpoint directory entries durable, allocating pre-checkpoint sequence directories exclusively, recapturing the pre-run git baseline after the handoff write, preserving transcript close on panic, degrading post-run git capture failures to an unknown verdict, preserving provider exit-code semantics on post-checkpoint failures, and comparing commit evidence in files-changed verdicts.

### File List

- `_bmad-output/implementation-artifacts/3-4-pre-post-run-checkpoints-durability-ordering.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `cmd/aictl/run_test.go`
- `internal/app/handoff.go`
- `internal/app/run.go`
- `internal/app/run_test.go`
- `internal/checkpoint/checkpoint.go`
- `internal/checkpoint/checkpoint_test.go`
- `internal/session/paths.go`
- `internal/session/paths_test.go`
- `internal/session/store.go`

## Change Log

- 2026-06-16: Created Story 3.4 context with architecture guardrails, previous-story intelligence, implementation tasks, and test guidance. Status set to ready-for-dev.
- 2026-06-16: Implemented Story 3.4 - pre/post-run checkpoint directories and artifacts, run-time handoff durability ordering, checkpoint transcript relocation, files-changed verdict, and focused regression coverage. Status set to review.
- 2026-06-16: Addressed code-review findings for durability fsync, sequence collision avoidance, files-changed baselining, panic transcript close, post-run git degradation, post-checkpoint exit-code preservation, and commit-only verdict detection. Status set to done.
