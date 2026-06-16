---
baseline_commit: 6514afa
context:
  - _bmad-output/implementation-artifacts/3-1-run-provider-in-pty.md
---

# Story 3.2: Faithful transcript & quiet supervision

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `aictl run <provider>` to record the run to a faithful raw-ANSI transcript while staying completely silent on the controlling terminal during the run,
so that I get a byte-faithful log of the session without the supervisor ever interfering with the provider's TUI.

## Acceptance Criteria

1. **Faithful transcript via single-write fan-out:** during a provider run, the PTY output stream is written **once** to a fan-out writer that mirrors it to the user's terminal **and** tees it to a per-run transcript file. The bytes captured in the transcript are byte-identical to the bytes written to the terminal (raw ANSI/control sequences preserved verbatim, no reformatting, no extra buffering that changes ordering). (FR-6, NFR-2)
2. **Capture never degrades the UI:** transcript capture must not alter what the user sees on screen. A transcript write failure (e.g. disk full, closed file) must **not** abort or corrupt the on-screen passthrough — the run continues mirroring to the terminal; the capture failure is recorded but non-fatal. A terminal write failure remains authoritative and ends the copy as before. (FR-6, NFR-2)
3. **Transcript written for each run:** a transcript file is written for every run at a single deterministic location (`.ai-session/transcript.ansi`). It contains the full raw output stream of the run and is flushed/closed on every exit path (normal, error, signal, panic). (FR-6)
4. **Quiet during the run:** `aictl` emits **no** output to the controlling terminal between provider start and provider exit. Any `aictl` user-facing output produced while the provider owns the screen is buffered and flushed only after the provider exits. (FR-7, NFR-2, NFR-5)
5. **Concise pre/post status only:** `aictl` prints concise status lines **before** the provider starts (e.g. "Launching <provider>") and **after** it exits (e.g. exit reason / "exited with code N"). Pre/post lines remain short; no per-chunk or progress logging is added. (FR-7, NFR-5)
6. **No later-story scope:** this story does not implement usage-limit detection, exit-reason classification, pre/post checkpoints, per-Attempt/checkpoint directories, the attempts log, provider adapters/injection, or content-level secret scanning of the transcript. The fan-out writer must leave a clean extension point for the usage detector tap (Story 4.1) without implementing it. (Scope boundary)

## Tasks / Subtasks

- [x] **Task 1 — Single-write fan-out writer (AC: 1, 2, 6)**
  - [x] Create `internal/shell/fanwriter.go` with a small `io.Writer` (e.g. `fanWriter`) constructed from a primary terminal writer and zero-or-more secondary "tap" writers (the transcript for this story).
  - [x] On each `Write`, write to the terminal **first** (authoritative); a terminal write error is returned and ends the copy (preserves Story 3.1 behavior).
  - [x] After a successful terminal write, tee the same bytes to each secondary tap. A tap (transcript) write error must be recorded once and that tap dropped/disabled, but `Write` must report success for the byte count the terminal accepted so the on-screen path is never degraded (NFR-2).
  - [x] Keep the design extensible for additional taps (the usage detector is added in Story 4.1) — e.g. a `[]io.Writer` of taps or an explicit `addTap` seam — but do **not** add detection logic here.
  - [x] Do not introduce buffering that reorders or delays bytes relative to a plain copy (NFR-2/NFR-6: single pass-through).
- [x] **Task 2 — Wire the fan-out into the runner (AC: 1, 2, 3)**
  - [x] Add a `Transcript io.Writer` field to `shell.Options` (alongside the existing `Stdout`). When nil, the runner mirrors to the terminal only (preserves Story 3.1 tests and the no-transcript path).
  - [x] In `Run`, replace `io.Copy(stdout, ptmx)` with a copy into the fan-out writer built from `stdout` + (`opts.Transcript` if non-nil). All existing PTY lifecycle, raw-mode restore, resize, signal-restore, and exit-code behavior is unchanged.
  - [x] The runner does **not** open or own the transcript file (process/persistence boundary): it writes to the injected `io.Writer` only. File creation/closing is the app layer's responsibility.
- [x] **Task 3 — Transcript destination wired in the app (AC: 3, 6)**
  - [x] Add a `Transcript()` path accessor to `internal/session/paths.go` returning `<repo>/.ai-session/transcript.ansi` (single source of truth for the path; no hand-built paths elsewhere).
  - [x] In `internal/app/run.go`, before invoking the runner, ensure the `.ai-session/` directory exists (lightweight `os.MkdirAll`; do **not** run full `init` and do not require a prior session) and open/create the transcript file as a streaming writer. Pass it as `shell.Options.Transcript`.
  - [x] The transcript is a **stream**, so it is opened directly (e.g. `os.Create`) — it is the documented exception to the `session.store.WriteAtomic` rule, which is for whole-buffer atomic snapshots, not append streams. Document this rationale in a code comment.
  - [x] Guarantee the transcript file is closed on every exit path (success, provider error, runner error) via `defer`.
- [x] **Task 4 — Quiet supervision: muted/buffered UI (AC: 4, 5)**
  - [x] Add a mute/flush capability to `internal/ui/ui.go`: while muted, `Out`/`Printf` (and `Err`/`Errorf`) writes are buffered in memory rather than written to the terminal; an explicit flush emits the buffered bytes and resumes direct writes. Make mute/flush idempotent and safe to call once per run.
  - [x] In `internal/app/run.go`, print the concise pre-run line, **mute** the UI immediately before calling the runner, and **flush + unmute** immediately after the runner returns (before printing the post-run line). This structurally guarantees nothing `aictl` produces can reach the terminal mid-run (NFR-2/NFR-5), even for future mid-run callers.
  - [x] Update the now-stale `ui.go` doc comment that says the "output-error policy is deferred to the Epic 3 fan-out writer" — that policy now lives in `fanwriter.go` (Task 1); reflect this.
- [x] **Task 5 — Test coverage and verification (AC: 1–6)**
  - [x] `internal/shell` fan-out unit tests: terminal + transcript receive byte-identical output; a failing transcript tap does not abort terminal mirroring and does not reduce the reported terminal byte count; a failing terminal write propagates.
  - [x] `internal/shell` PTY integration test (extend the Story 3.1 `TestHelperProcess` fake CLI): child emits ANSI/control sequences; assert the captured transcript is byte-identical to the mirrored terminal output and that raw escape sequences survive verbatim. Skip only where no usable PTY/TTY exists, matching Story 3.1.
  - [x] `internal/ui` unit test: while muted, writes are buffered (terminal sees nothing); after flush, buffered content appears exactly once and subsequent writes pass through.
  - [x] `internal/app` test: a run creates `.ai-session/transcript.ansi` containing the provider output; the pre-run line appears before and the post-run line after, with nothing emitted during (assert ordering / that mid-run output is buffered).
  - [x] Run `go test ./...`, `go vet ./...`, `go build ./...`, **and `go test -race ./internal/shell ./internal/app ./internal/ui`** (Story 3.1 review found a `-race`-only test data race; new concurrent tests must be race-clean). Run `golangci-lint run` if installed; cross-build `GOOS=windows go build ./...`.

## Dev Notes

**Second story of Epic 3.** Builds directly on the Story 3.1 PTY runner. Story 3.1 delivered the PTY launch, raw-mode/restore, resize, signal-driven restore, and exit-code preservation; it deliberately mirrored PTY output with a plain `io.Copy(stdout, ptmx)` and printed only pre/post lines. This story replaces that plain copy with the **single-write fan-out** (terminal + transcript) and makes the quiet-supervision guarantee structural via a muted UI.

> **Prerequisite:** Story 3.1's changes are in the working tree (uncommitted at the time this story was authored). Commit Story 3.1 before starting so `baseline_commit: 6514afa` plus that commit is your clean diff baseline.

### Scope Boundary

- **In scope:** the fan-out writer (`internal/shell/fanwriter.go`), wiring it into the runner's output path, a per-run raw-ANSI transcript at `.ai-session/transcript.ansi`, the streaming transcript file lifecycle in the app, and a muted/buffered `internal/ui` that keeps `aictl` silent between provider start and exit.
- **Out of scope:** usage-limit detection / stream scanning (FR-19, Story 4.1), `classify(exitCode, detectorState)` exit-reason logic (Story 4.1), pre/post checkpoints and `checkpoints/NNNN-{before,after}-<provider>/` directories (Story 3.4), per-Attempt directory layout and `attempts.log` (Epic 4), provider adapters / built-in Trio / config-driven providers / prompt injection (Story 3.3), and content-level secret scanning/redaction of transcript bytes (deferred post-v1 — see Edge Cases).
- **Do not make `aictl` an alternate UI.** The provider's own TUI remains the UX. More status messaging is explicitly an anti-goal (PRD SM-C2 "Don't get chatty"). Add no new mid-run output.

### Reuse / Do Not Reinvent

- **Extend the Story 3.1 runner; do not fork it.** Add the `Transcript` field to the existing `shell.Options` and swap the existing output `io.Copy` for the fan-out writer. Keep all 3.1 lifecycle code (`enterRawMode`, `watchTerminalRestore`, `startResizeWatcher`, `exitCode`, double-`ptmx.Close` unblock pattern) intact.
- **Reuse `internal/ui.UI`** for all pre/post messages — never write to `os.Stdout`/`os.Stderr` directly from `app`/`cmd` during a run (Architecture output boundary). The fan-out writer is the **only** thing writing the terminal during the run, and it owns nothing but PTY bytes.
- **Reuse `internal/session.Paths`** for the transcript path — add a `Transcript()` accessor; never hand-build `.ai-session/...` paths (Architecture: centralized paths).
- **Process boundary:** only `internal/shell` spawns/owns the child PTY and writes terminal bytes during the run. `shell` must not import `session`/`app` or learn the transcript path — it receives an `io.Writer`. The app opens the file and injects the writer.
- **Do not route the streaming transcript through `session.store.WriteAtomic`** — that helper is for atomic whole-buffer snapshots (state/handoff/checkpoints). A live output stream is a deliberate, documented exception.

### Expected Implementation Shape

Recommended new/changed files (mirrors `architecture.md` §Project Structure: `internal/shell/fanwriter.go` is already named there as "single-write fan-out → terminal+transcript+detector (FR-6, NFR-2)"):

- `internal/shell/fanwriter.go` (NEW): the fan-out `io.Writer`.
- `internal/shell/runner.go` (MODIFY): add `Options.Transcript`; build the fan-out and copy into it.
- `internal/session/paths.go` (MODIFY): add `Transcript()` accessor.
- `internal/app/run.go` (MODIFY): ensure `.ai-session/` exists, open/close the transcript stream, mute/flush the UI around the run.
- `internal/ui/ui.go` (MODIFY): add mute/buffer + flush; update the stale doc comment.

Suggested fan-out contract (evolve as needed, keep testable — prefer dependency injection over globals, consistent with 3.1):

```go
// fanWriter mirrors PTY output to the terminal and tees it to taps
// (the transcript now; the usage detector is added in Story 4.1).
type fanWriter struct {
    term io.Writer   // authoritative: terminal write errors end the copy
    taps []io.Writer // best-effort: a tap error drops that tap, never the screen
}

func newFanWriter(term io.Writer, taps ...io.Writer) *fanWriter { /* skip nil taps */ }

func (w *fanWriter) Write(p []byte) (int, error) {
    n, err := w.term.Write(p)
    if err != nil {
        return n, err // terminal is authoritative
    }
    for i, t := range w.taps {
        if t == nil { continue }
        if _, terr := t.Write(p[:n]); terr != nil {
            w.taps[i] = nil // disable a failing tap; never fail the screen
        }
    }
    return n, nil
}
```

### Current Files to Modify (read these in full before editing)

- `internal/shell/runner.go` — Story 3.1 runner. Output is mirrored at the goroutine `go func() { _, copyErr := io.Copy(stdout, ptmx); outputDone <- copyErr }()`. **What changes:** `stdout` becomes the fan-out writer (`newFanWriter(stdout, opts.Transcript)`). **What must be preserved:** the `outputDone` channel + `ptmx.Close()`-to-unblock sequence, deferred raw-mode restore, `watchTerminalRestore`, resize watcher, and exit-code mapping. The fan-out writer must be a pure `io.Writer` so the copy/cleanup choreography is untouched.
- `internal/app/run.go` — Story 3.1 `App.Run`. Currently: `LookPath` → `UI.Printf("Launching ...")` → `runProvider(ctx, Options{...})` → `UI.Printf("Provider ... exited ...")` → `ExitError` on nonzero. **What changes:** open the transcript stream and pass it; mute the UI around `runProvider`; flush+unmute before the post-run line. **What must be preserved:** `exec.LookPath` pre-flight (fail before any side effect, AC6 of 3.1), the `ExitError{Code}` propagation through `main` (3.1), and `context` threading.
- `internal/ui/ui.go` — single output surface. `Printf`/`Errorf` swallow write errors (EPIPE) by design. **What changes:** add mute/buffer + flush; correct the comment that defers output-error policy to "the Epic 3 fan-out writer" (now realized here). **What must be preserved:** `Out()`/`Err()` accessors used by Cobra wiring in `root.go`.
- `internal/session/paths.go` — centralized `.ai-session/` paths. **What changes:** add `Transcript()` → `filepath.Join(p.root, "transcript.ansi")`. Follow the existing accessor style exactly.

### Edge Cases That Must Not Be Missed

- **Transcript write failure mid-run:** disk full / closed file must not abort or visibly disturb the provider's screen output (NFR-2). Drop the tap, keep mirroring. Cover with a test using a writer that errors after N bytes.
- **`.ai-session/` does not exist:** Story 3.1 intentionally did not require it. This story creates the directory lazily for the transcript (`os.MkdirAll`) without running `init` and without requiring an active session. Do not error if it already exists.
- **No-transcript path:** `Options.Transcript == nil` must behave exactly like Story 3.1 (terminal-only mirroring) so existing runner tests keep passing.
- **Flush ordering for quiet supervision:** the muted UI must be flushed **after** the provider exits and **before** the post-run line, so buffered mid-run notices (if any future caller emits them) appear in order, after the screen is back to `aictl`.
- **Restore/close ordering:** the transcript file close (app `defer`) and the runner's terminal restore are independent; ensure a panic or signal still closes the transcript (app-level `defer`) and still restores the terminal (runner-level, from 3.1). Do not let transcript handling swallow the runner's error/exit code.
- **Byte fidelity:** do not run the stream through anything that rewrites newlines, strips ANSI, or normalizes `\r\n` (the Story 3.1 tests `normalizePTYOutput` only for *assertion* convenience — the transcript itself must stay raw).
- **Secrets in the transcript (known, deferred):** the transcript is the raw on-screen stream and may contain secrets the provider printed. Per PRD, v1 keeps a **raw ANSI transcript only**; content-level secret scanning/redaction is explicitly deferred post-v1 (consistent with the 2.2/2.3 deferrals and PRD "rich transcript cleaning" non-goal). Do not attempt redaction here; note it as deferred.

### Testing Guidance

- **Fan-out fidelity is the headline test:** feed known bytes (including raw `ESC[` sequences and `\r`) and assert `terminal == transcript` byte-for-byte.
- Reuse the Story 3.1 `TestHelperProcess` fake-CLI pattern and `os.Args[0] -test.run=TestHelperProcess` self-exec; add a case that emits ANSI control output. Prefer Go test helper processes / `testdata/fakecli` over real provider CLIs (Architecture testing guidance).
- Inject failing writers to test the tap-failure-doesn't-degrade-screen path deterministically, rather than simulating disk-full.
- Keep PTY tests platform-aware (macOS/Linux; Windows/ConPTY out of scope) and **race-clean** — Story 3.1's review caught a `-race`-only data race in a watcher test (unsynchronized counter written from a goroutine); use `sync/atomic` or channels for any cross-goroutine assertions.
- The quiet-supervision test should assert *ordering and absence*: capture the UI's terminal writer in a buffer and assert it is empty between the launching line and the provider's exit, with the post-run line appearing only after.

### Previous Story Intelligence (Story 3.1)

- **Runner API in place:** `shell.Run(ctx, Options{Command, Args, Dir, Env, Stdin, Stdout, Stderr, /* add Transcript */})` returning `Result{ExitCode}`. `New(u)` injects `runProvider = shell.Run`; tests swap `a.runProvider` — reuse this seam for app-level tests.
- **`Options.Stderr` is intentionally retained but unused** (a PTY merges stdout+stderr); it is documented as reserved. Do **not** repurpose it for the transcript — add a dedicated `Transcript` field.
- **Exit-code path:** nonzero provider exit → `app.ExitError{Code}` → `main.exitCodeFromError` → `os.Exit(code)`. Preserve this; the transcript/quiet work must not swallow or alter exit codes.
- **Restore lifecycle:** raw-mode restore is `sync.Once`-idempotent and also driven proactively by `watchTerminalRestore` on SIGINT/SIGTERM; output goroutine is unblocked by `ptmx.Close()` then `<-outputDone`. Inserting the fan-out writer must not change this choreography.
- **Review deferrals carried (do not regress, do not pull in):** the stdin→ptmx copy goroutine can park on `os.Stdin` after child exit (inherent PTY tradeoff, deferred); no `cmd.WaitDelay` yet (deferred). These remain out of scope here. See `deferred-work.md` (code review of 3-1-run-provider-in-pty).
- **Project conventions confirmed:** thin `cmd/` → `internal/app` orchestration → feature package (`internal/shell`); `context.Context` first arg; conventional-commit one-feature-per-story history; no networking imports (CI import-check, NFR-1) — the transcript/fan-out work adds none.

### Latest Technical Information

- No new dependencies are required. `github.com/creack/pty v1.1.24` and `golang.org/x/term v0.44.0` were pinned in Story 3.1 and are sufficient; the fan-out writer is pure stdlib `io`. Adding a network import would break the NFR-1 CI import-check — do not.
- Go toolchain remains `go 1.26.0` in `go.mod` (1.26.4 current stable as of 2026-06-16); do not bump unless the build requires it.
- `io.MultiWriter` is **not** a drop-in here: it fails the whole write if any writer errors, which would let a transcript failure degrade the screen (violates NFR-2/AC2). Implement the custom fan-out so a tap error is isolated.

### Project Structure Notes

- NEW: `internal/shell/fanwriter.go` (+ `fanwriter_test.go`).
- MODIFIED: `internal/shell/runner.go`, `internal/shell/runner_test.go`, `internal/app/run.go`, `internal/app/run_test.go`, `internal/ui/ui.go` (+ `ui_test.go`), `internal/session/paths.go` (+ existing paths test if present).
- Runtime artifact: `<repo>/.ai-session/transcript.ansi` (latest per-run transcript). Relocation/copy into the per-Attempt `checkpoints/NNNN-after-<provider>/transcript.ansi` is **Story 3.4**, not here.
- No `cmd/aictl/run.go` change is expected — the Cobra surface from Story 3.1 already delegates to `App.Run`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.2: Faithful transcript & quiet supervision]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-6: Capture a faithful transcript without disturbing the UI]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-7: Keep the supervisor quiet while the Provider owns the screen]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#NFR-2 / NFR-5]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md#PTY runner shape (from brief) — tee + scan]
- [Source: _bmad-output/planning-artifacts/architecture.md#API & Communication — Child process I/O (PTY runner) fan-out]
- [Source: _bmad-output/planning-artifacts/architecture.md#Process Patterns — Quiet-supervision logging]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure — internal/shell/fanwriter.go, internal/ui/ui.go]
- [Source: _bmad-output/implementation-artifacts/3-1-run-provider-in-pty.md (runner, Options, ExitError, restore lifecycle)]
- [Source: internal/shell/runner.go]
- [Source: internal/app/run.go]
- [Source: internal/ui/ui.go]
- [Source: internal/session/paths.go]

## Review Findings

_Code review 2026-06-16 (Blind Hunter + Edge Case Hunter + Acceptance Auditor; clean diff vs committed 3.1 baseline `09fd5ae`). All three layers completed. AC1, AC2, AC5, AC6 audited as satisfied._

### Decision Needed (resolved 2026-06-16)

- Resolved → keep current behavior: transcript open failure stays **fatal before launch** (after `LookPath`, no side effects), honoring AC3's "written for every run" literally. No code change. Daniel's call.

### Patch

- [x] [Review][Patch] cmd-level run test leaks a real `.ai-session/` into the source tree [cmd/aictl/run_test.go] — FIXED: added `t.Chdir(t.TempDir())` to `TestRunCommandPassesProviderArgsAndReturnsExitError`; deleted the leaked `cmd/aictl/.ai-session/`. The full suite now leaves no `.ai-session/` in the tree.
- [x] [Review][Patch] UI left muted on a panic/early-unwind [internal/app/run.go] — FIXED: added `defer a.UI.Flush()` right after `Mute()` (idempotent, so the inline Flush still wins on the normal path). Added `TestRunFlushesUIEvenIfRunnerPanics` (panicking `runProvider` ⇒ buffered notice still flushed).
- [x] [Review][Patch] `run` without prior `init` leaves a secret-bearing transcript un-gitignored [internal/app/run.go `openTranscript`] — FIXED: when the Session Directory is lazily created and no `.gitignore` exists, `openTranscript` writes `gitignoreContents` via `session.WriteAtomic`. Added `TestRunWithoutInitWritesGitignore`.
- [x] [Review][Patch] Transcript created world-readable despite holding secrets [internal/app/run.go `openTranscript`] — FIXED: `os.OpenFile(paths.Transcript(), O_CREATE|O_WRONLY|O_TRUNC, 0o600)` so the transcript is owner-only.
- [x] [Review][Patch] init gitignore test not updated for the new entry [internal/app/init_test.go] — FIXED: `transcript.ansi` added to the `TestInitGitignoreIgnoresTransient` asserted list.

### Deferred

- [x] [Review][Defer] Concurrent `aictl run` truncates/corrupts the shared `transcript.ansi` [internal/app/run.go] — deferred. `os.Create` truncates and there is no run lock; per-Attempt transcript directories (`checkpoints/NNNN-after-<provider>/transcript.ansi`) and run-time locking are Story 3.4 scope.
- [x] [Review][Defer] No negative-path test asserting the transcript is closed on the provider-error / signal path [internal/app/run_test.go] — deferred. Close-on-error and close-on-panic are correct by construction (deferred `closeTranscript` runs on `return res, err` and during panic unwind); the panic test added for the UI-flush patch covers the panic path. A dedicated error-path close assertion can follow.
- [x] [Review][Defer] Tap short-write / close error swallowed [internal/shell/fanwriter.go, internal/app/run.go] — deferred. The transcript is a best-effort tap; a partial tap write on `ENOSPC` (or a swallowed `Close` error) could make it non-byte-identical. Low probability; acceptable for v1 given capture must never degrade the screen.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.8 (claude-opus-4-8)

### Debug Log References

- `go test ./internal/shell -run TestFanWriter` — RED (undefined `newFanWriter`), GREEN after adding `fanwriter.go` (4 tests).
- `go test ./internal/shell -run TestRunCapturesTranscript` — RED (unknown `Options.Transcript`), GREEN after adding the field + fan-out wiring.
- `go test ./internal/ui` — RED (undefined `Mute`/`Flush`), GREEN after adding muted buffering (4 tests).
- `go test ./internal/app -run 'TestRunWritesTranscriptFile|TestRunIsQuietDuringProviderRun'` — RED (no transcript writer / mid-run leak), GREEN after wiring `openTranscript` + `UI.Mute/Flush` into `App.Run`.
- `go test ./...` — 117 passed across 10 packages.
- `go vet ./...` — clean. `go build ./...` — clean. `gofmt -l` — clean.
- `go test -race ./internal/shell ./internal/app ./internal/ui ./internal/session` — 68 passed, race-clean.
- `GOOS=windows GOARCH=amd64 go build ./...` — clean.
- `golangci-lint` — not installed in this environment.

### Completion Notes List

- Implemented the single-write fan-out (`internal/shell/fanwriter.go`): terminal-authoritative, best-effort taps. A tap (transcript) write error disables that tap and never surfaces as a `Write` error or reduces the terminal byte count, so capture can never degrade the TUI (NFR-2). Deliberately not `io.MultiWriter` (which would fail the whole write on a tap error).
- Wired the fan-out into the Story 3.1 runner via a new `Options.Transcript io.Writer` (nil ⇒ terminal-only, preserving 3.1 behavior). The `outputDone`/`ptmx.Close`/restore choreography is unchanged — the fan-out is a pure `io.Writer`.
- Added a per-run transcript at `.ai-session/transcript.ansi` via a new `session.Paths.Transcript()`; `App.Run` opens it as a stream (`os.Create`, the documented exception to `WriteAtomic`), creating `.ai-session/` lazily without requiring `aictl init`, and closes it on every exit path via `defer`.
- Made quiet supervision structural: added `UI.Mute()`/`UI.Flush()` (ordered buffering across out/err) and bracketed the provider run in `App.Run` so any mid-run aictl output is buffered and flushed only after the provider exits. Pre/post lines ("Launching …", "Provider … exited with code N") are unchanged and concise.
- Resolved the stale `ui.go` comment that deferred the "output-error policy to the Epic 3 fan-out writer" — that policy now lives in `fanwriter.go`.
- Safety: added `transcript.ansi` to the `aictl init` `.gitignore` so raw transcripts (which can contain secrets) are not committed, matching the existing rationale for ignoring `checkpoints/`.
- Updated two existing app tests to `t.Chdir(t.TempDir())` so the new transcript creation does not write `.ai-session/` into the repo during tests.
- Out of scope and not added (per AC6): usage-limit detection (the fan-out leaves a `taps` seam for the Story 4.1 detector), exit-reason classification, pre/post checkpoints and per-Attempt dirs (3.4), attempts log, provider adapters/injection (3.3), and content-level secret scanning of transcript bytes (deferred post-v1 — transcript is raw ANSI by design).
- Note: transcript open failure is fatal *before* launch (after `LookPath`), preserving 3.1's "no side effects when the executable is missing" ordering while guaranteeing AC3's per-run transcript.

### File List

- `internal/shell/fanwriter.go` (new)
- `internal/shell/fanwriter_test.go` (new)
- `internal/shell/runner.go` (modified — `Options.Transcript`, fan-out wiring)
- `internal/shell/runner_test.go` (modified — transcript fidelity test + `emit-ansi` helper case)
- `internal/ui/ui.go` (modified — `Mute`/`Flush` buffering; updated comment)
- `internal/ui/ui_test.go` (modified — mute/flush tests)
- `internal/session/paths.go` (modified — `Transcript()` accessor)
- `internal/app/run.go` (modified — open transcript stream, mute/flush around the run)
- `internal/app/run_test.go` (modified — transcript + quiet tests; `t.Chdir` on two existing tests)
- `internal/app/init.go` (modified — ignore `transcript.ansi`)
- `internal/app/init_test.go` (modified — assert `transcript.ansi` in gitignore; review fix)
- `cmd/aictl/run_test.go` (modified — `t.Chdir` to a temp dir to stop the transcript leak; review fix)

## Change Log

- 2026-06-16: Implemented Story 3.2 — single-write fan-out transcript capture (`internal/shell/fanwriter.go`), `Options.Transcript` wiring, `.ai-session/transcript.ansi` per-run stream, and quiet supervision via a muted/buffered `internal/ui`. Story moved to review.
- 2026-06-16: Addressed code review findings — 5 patches resolved (cmd-test transcript leak fixed via `t.Chdir`; `defer a.UI.Flush()` for panic/error-safe quiet supervision; lazy `.gitignore` write so `run`-without-`init` never leaves an un-ignored transcript; transcript file created `0o600`; init gitignore test updated). 1 decision resolved (transcript-setup failure stays fatal). 3 items deferred to `deferred-work.md`.
