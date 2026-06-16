---
baseline_commit: 6514afa
---

# Story 3.1: Run a provider in a pseudo-terminal (`aictl run <provider>`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `aictl run <provider>` to launch a real provider CLI inside a pseudo-terminal,
so that the provider's native terminal UI, keyboard handling, resize behavior, and exit status behave as if I had run the provider directly.

## Acceptance Criteria

1. **PTY launch with native UI passthrough:** given an installed provider CLI, when I run `aictl run <provider>`, `aictl` starts that executable inside a pseudo-terminal and mirrors provider output to the user's terminal without visible wrapper output during the run. (FR-5, FR-7, NFR-2)
2. **Keyboard input reaches the provider:** stdin is passed through to the provider PTY, including raw-mode keys and interactive control sequences needed by TUIs and approval prompts. (FR-5, NFR-2)
3. **Terminal resize propagates:** `SIGWINCH` from the controlling terminal updates the provider PTY size so rich terminal UIs reflow correctly. (FR-5, NFR-2)
4. **Provider exit status is preserved:** if the provider exits with code `N`, `aictl run` surfaces that result and the `aictl` process exits with `N`; if the provider is terminated by signal `S`, surface a conventional `128+S` result. (FR-5)
5. **Terminal mode is restored every time:** normal completion, provider error exit, context cancellation, panic inside runner code, and process signal handling all restore the user's terminal to its prior mode. Restore must be idempotent and tested. (FR-5, NFR-2)
6. **Missing executable fails clearly:** if `<provider>` is not found on `PATH` or is not executable, `aictl run <provider>` fails before raw mode with a clear error and no session/provider side effects.
7. **No later-story scope is implemented:** this story does not add transcript capture, usage-limit detection, prompt/handoff injection, provider adapters, pre/post checkpoints, attempts logging, or fallback chains. Those belong to Stories 3.2-3.4 and Epic 4.

## Tasks / Subtasks

- [x] **Task 1 - Add the PTY runner package (AC: 1, 2, 4, 5)**
  - [x] Create `internal/shell/runner.go` with a small testable API, for example `Run(ctx context.Context, opts Options) (Result, error)`.
  - [x] Keep `internal/shell` as the only package that starts child PTYs or changes terminal modes.
  - [x] Use `exec.CommandContext` for the child process and `github.com/creack/pty` for the pseudo-terminal.
  - [x] Copy keyboard input to the PTY (`io.Copy(ptmx, stdin)`) and PTY output to the terminal writer (`io.Copy(stdout, ptmx)`), with no transcript or detector yet.
  - [x] Convert child process results into a typed `Result{ExitCode int}`. Preserve normal exit codes and map signal exits to `128+signal`.
  - [x] Ensure PTY files are closed on all exit paths so blocked copy goroutines can unwind.
- [x] **Task 2 - Raw-mode lifecycle and terminal restore (AC: 2, 5)**
  - [x] Create `internal/shell/rawmode.go`.
  - [x] Enter raw mode only when stdin is a terminal; non-terminal stdin should still be usable for simple automated tests.
  - [x] Implement an idempotent restore function guarded by `sync.Once` or equivalent.
  - [x] Restore through `defer` from the top of the runner and from signal/cancellation handling before returning or exiting.
  - [x] Add panic-safety inside the runner: if runner code panics after raw mode is enabled, restore the terminal before re-panicking.
  - [x] Carry forward the deferred Story 1.1 item: a second interrupt must not leave the terminal stuck in raw mode. At minimum, restore before propagating a hard termination path; do not build fallback-chain cancellation logic yet.
- [x] **Task 3 - Resize propagation (AC: 3, 5)**
  - [x] Create `internal/shell/resize.go`.
  - [x] Apply the initial terminal size before/just after child start and listen for `syscall.SIGWINCH`.
  - [x] Use `pty.InheritSize(os.Stdin, ptmx)` or an injected wrapper around it; tolerate non-TTY/stdin-size errors without corrupting the run.
  - [x] Stop signal notification and clean up channels/goroutines when the runner returns.
- [x] **Task 4 - Add the `aictl run` app and Cobra surface (AC: 1, 4, 6, 7)**
  - [x] Add `internal/app/run.go` with `App.Run(ctx, provider string, args []string) (shell.Result, error)`.
  - [x] For Story 3.1, treat `<provider>` as an executable name or path. Use `exec.LookPath` for bare command names and return a clear error if missing.
  - [x] Do not require `.ai-session/` yet; handoff/checkpoint/session ordering is introduced in later stories.
  - [x] Print only concise pre-run and post-run lines via `UI` before the PTY starts and after it exits. Do not print `aictl` messages while the provider owns the terminal.
  - [x] Add `cmd/aictl/run.go` with `Use: "run <provider> [-- provider-args...]"`, `cobra.MinimumNArgs(1)`, and thin delegation to `App.Run`.
  - [x] Register `newRunCmd(a)` in `cmd/aictl/root.go`.
  - [x] Preserve provider exit codes in the real process. `main` currently exits `1` for every command error; add a typed exit-code error or equivalent path so `aictl run fake-exits-42` exits `42`.
- [x] **Task 5 - Test coverage and verification (AC: 1-7)**
  - [x] Add `internal/shell` unit tests for exit-code mapping, restore idempotence, restore-on-error, and restore-on-panic using injectable terminal-mode functions.
  - [x] Add a PTY integration test using a fake CLI/test helper that echoes stdin bytes and exits with a controlled code. Skip only when the platform/environment has no usable PTY/TTY support.
  - [x] Add a resize test using an injected `inheritSize` function or signal channel hook so `SIGWINCH` behavior is covered without relying on CI terminal geometry.
  - [x] Add app tests for missing provider executable and successful delegation to the runner.
  - [x] Add command/root tests that `run` is registered, validates args, passes arguments after `--`, and preserves typed provider exit code semantics.
  - [x] Run `go test ./...`, `go vet ./...`, and `go build ./...`. Run `golangci-lint run` if installed.

## Dev Notes

**First story of Epic 3. Depends on Epics 1 and 2 being implemented, but intentionally does not consume their handoff/checkpoint artifacts yet.** This story creates the risky terminal/process foundation that later Epic 3 stories will extend with transcript teeing, provider adapters, prompt injection, pre/post checkpoints, and quiet observed output.

### Scope Boundary

- In scope: direct executable launch via `aictl run <provider>`, PTY lifecycle, raw-mode keyboard passthrough, terminal resize, terminal restoration, provider exit-code preservation.
- Out of scope: `internal/providers`, built-in `claude|codex|gemini` adapters, config-driven adapters, prompt injection, `.ai-session/handoff.md` preparation, transcripts, usage-limit detection, checkpoints, attempts log, fallback chains.
- Do not make `aictl` an alternate UI. The provider's own TUI remains the UX; `aictl` must stay silent while the provider owns the screen.

### Reuse / Do Not Reinvent

- Reuse `internal/ui.UI` for pre/post messages. Do not write directly to stdout/stderr from app/cmd code during the run.
- Follow the existing command layering: `cmd/aictl/*.go` parses args only; `internal/app` orchestrates; feature package logic lives in `internal/shell`.
- Use `context.Context` for subprocess lifetime, matching prior verify/recover review standards.
- Preserve the no-network property. `os/exec` and PTY process I/O are allowed; do not add network imports.

### Expected Implementation Shape

Recommended package files:

- `internal/shell/runner.go`: command start, PTY copy loop, wait/result mapping, cancellation cleanup.
- `internal/shell/rawmode.go`: terminal detection, raw-mode enter, idempotent restore.
- `internal/shell/resize.go`: initial size and `SIGWINCH` handling.
- `internal/app/run.go`: path lookup, pre/post UI lines, runner invocation, result-to-error handling.
- `cmd/aictl/run.go`: thin Cobra command.

Suggested runner contracts can evolve during implementation, but keep them testable:

```go
type Options struct {
    Command string
    Args    []string
    Dir     string
    Env     []string
    Stdin   *os.File
    Stdout  io.Writer
    Stderr  io.Writer
}

type Result struct {
    ExitCode int
}
```

If this shape is awkward for tests, prefer dependency injection over global mutable state.

### Current Files to Modify

- `cmd/aictl/root.go`: currently registers `init`, `start`, `note`, `done`, `next`, `fail`, `handoff`, `verify`, `recover`; add `run`.
- `cmd/aictl/main.go`: currently maps all command errors to `os.Exit(1)`. Update this carefully so provider exit codes survive without changing normal CLI validation errors.
- `cmd/aictl/root_test.go`: extend root command coverage for registration/argument behavior.
- `internal/app/app.go`: no new dependency field is required unless you choose to inject a runner for tests. Keep `App` small.
- `internal/ui/ui.go`: use as-is for pre/post messages. Do not solve the deferred synchronized writer/fan-out concern here; Story 3.2 owns the fan-out writer.
- `go.mod`: add `github.com/creack/pty v1.1.24` when first imported. Add `golang.org/x/term` only if needed for raw-mode support; document why in completion notes because the architecture emphasized a lean dependency tree.

### Edge Cases That Must Not Be Missed

- Provider exits non-zero: `aictl` must return the same code, not collapse to `1`.
- Provider is killed by signal: return/report `128+signal`.
- Missing provider executable: fail before raw mode and before spawning any process.
- Context cancellation or SIGTERM while provider is running: restore terminal before terminating/returning.
- Panic after raw mode enabled: restore terminal before re-panicking.
- Non-interactive stdin/stdout in tests: avoid hard failure where a simpler passthrough run can still be tested.
- Copy goroutine blocked on stdin: closing the PTY on runner shutdown should unblock the child/output path; do not leak long-lived signal handlers.

### Testing Guidance

- Keep PTY tests platform-aware: v1 targets macOS/Linux; Windows/ConPTY is explicitly out of scope.
- Prefer Go test helper processes or `testdata/fakecli` over depending on real provider CLIs.
- A fake CLI should cover: echo input, render ANSI/control output without `aictl` interleaving, exit `0`, exit `42`, and optionally trap/report resize.
- Raw-mode restore tests should use injected `makeRaw`/`restore` functions so they do not depend on the developer's actual terminal.
- Root/main exit-code behavior may need an executable integration test because `main` is where `os.Exit(code)` happens.

### Previous Story Intelligence

- Story 1.1 deferred `github.com/creack/pty` until first import. This is that story; pin it in `go.mod` rather than leaving only prose.
- Story 1.1 also deferred "second Ctrl-C hard exit" to the PTY runner. Address the terminal-restore safety part here; fallback-chain behavior still belongs to Epic 4.
- Story 2.3 review established that subprocesses must take `context.Context`; follow that standard for provider processes.
- Story 2.4 completed recovery and left repo-root/untracked-file semantics deferred. Do not pull those into this story.
- Story 2.2/2.3 deferred content secret scanning and bounded command-log/verify output. This story must not create new transcript/handoff surfaces that would need those protections.

### Latest Technical Information

- Web check on 2026-06-16: `github.com/creack/pty` v1.1.24 is the current v1 package version on pkg.go.dev, with documented `Start` and `InheritSize` APIs. pkg.go.dev also notes a higher major `v2`; do not jump to v2 without an architecture decision because the project has pinned v1.1.24. Source: https://pkg.go.dev/github.com/creack/pty
- The `creack/pty` shell example uses `golang.org/x/term` for `MakeRaw`/`Restore`; `golang.org/x/term` v0.44.0 is current on pkg.go.dev as of 2026-06-16. Source: https://pkg.go.dev/golang.org/x/term
- Go's current 1.26 minor is 1.26.4 as of 2026-06-16; the repo currently declares `go 1.26.0`. Do not change the Go directive unless implementation/build requires it. Source: https://go.dev/doc/devel/release
- `github.com/spf13/cobra` v1.10.2 is already in `go.mod` and remains appropriate for adding `run`. Source: https://pkg.go.dev/github.com/spf13/cobra

### Project Structure Notes

- NEW: `internal/shell/runner.go`, `internal/shell/rawmode.go`, `internal/shell/resize.go`, corresponding tests, `internal/app/run.go`, `cmd/aictl/run.go`.
- MODIFIED: `go.mod`, `go.sum`, `cmd/aictl/root.go`, `cmd/aictl/main.go`, tests under `cmd/aictl` and `internal/app` as needed.
- No `internal/providers` package in this story. That package starts in Story 3.3.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.1: Run a provider in PTY]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-5: Run a Provider in a pseudo-terminal with full passthrough]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-7: Keep the supervisor quiet while the Provider owns the screen]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md#PTY runner shape]
- [Source: _bmad-output/planning-artifacts/architecture.md#PTY Runner]
- [Source: _bmad-output/planning-artifacts/architecture.md#Architectural Boundaries]
- [Source: _bmad-output/implementation-artifacts/deferred-work.md#Deferred from: code review of 1-1-project-foundation-and-skeleton]
- [Source: cmd/aictl/main.go]
- [Source: cmd/aictl/root.go]
- [Source: internal/app/app.go]
- [Source: internal/ui/ui.go]

## Review Findings

_Code review 2026-06-16 (Blind Hunter + Edge Case Hunter + Acceptance Auditor). All three layers completed; none failed._

### Decision Needed (resolved 2026-06-16)

- Resolved → Patch: `Options.Stderr` dead with a PTY — Daniel chose **keep + document** (matches spec's suggested `Options` shape; may serve future non-PTY paths). See Patch list.
- Resolved → Patch: Explicit hard-termination / second-interrupt restore — Daniel chose **add explicit now** (signal-driven restore in the runner + test). See Patch list.

### Patch

- [x] [Review][Patch] `exitCode` swallows the real wait error, collapsing to generic `1` [internal/shell/runner_unix.go:30-31] — FIXED: returns `fmt.Errorf("wait for provider: %w", waitErr)` when no process state is available.
- [x] [Review][Patch] No test for AC5 restore-on-context-cancellation [internal/shell/runner_test.go] — FIXED: added `TestRunRestoresRawModeOnContextCancellation`, which cancels ctx mid-run and asserts restore called once + non-zero (killed) exit code.
- [x] [Review][Patch] Document `Options.Stderr` as currently unused under a PTY [internal/shell/runner.go:21] — FIXED: added doc comment explaining the PTY merges stdout+stderr and the field is reserved.
- [x] [Review][Patch] Add explicit signal-driven terminal restore + second-interrupt guard in the runner with a test [internal/shell/rawmode.go, runner.go] — FIXED: added `watchTerminalRestore` (proactively restores on SIGINT/SIGTERM, idempotent), wired into `Run`, platform `interruptSignals`, and `TestWatchTerminalRestoreRestoresOnSignal`.

### Patch (bonus — surfaced during verification)

- [x] [Review][Patch] Data race in `TestStartResizeWatcherInheritsInitialAndSignalSize` [internal/shell/runner_test.go] — FIXED: `go test -race` flagged an unsynchronized `calls` counter written by the watcher goroutine; switched to `atomic.Int32`. (Not caught by plain `go test`.)

### Deferred

- [x] [Review][Defer] stdin→ptmx copy goroutine leaks / can steal next keystroke [internal/shell/runner.go:80-82] — deferred, inherent PTY-wrapper tradeoff. `io.Copy(ptmx, stdin)` stays parked on `os.Stdin.Read` after the child exits; `ptmx.Close()` only unblocks the output path. Clean interruption of a blocking `os.Stdin` read is non-trivial; the spec's stated requirement (unblock the child/output path) is met.
- [x] [Review][Defer] No `cmd.WaitDelay`; `<-outputDone` could hang if a grandchild holds the PTY slave open [internal/shell/runner.go:84-86] — deferred, low-likelihood hardening for v1.
- [x] [Review][Defer] AC1 has no test asserting aictl stays quiet / no interleave + ANSI passthrough during the run [internal/shell/runner_test.go] — deferred, coverage gap; the structural guarantee (only pre/post UI lines, output mirrored verbatim) holds in code.

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- `go test ./internal/shell` - RED failed before shell package/dependencies existed; GREEN passed after adding PTY runner, raw-mode lifecycle, resize watcher, and tests.
- `go test ./internal/shell ./internal/app ./cmd/aictl` - passed after adding app and Cobra run command.
- `go test ./...` - passed across all packages.
- `go vet ./...` - passed.
- `go build ./...` - passed.
- `golangci-lint run` - not run; `golangci-lint` is not installed in this environment.

### Completion Notes List

- Implemented `internal/shell` PTY runner using `github.com/creack/pty`, with stdin passthrough, terminal output mirroring, resize handling, and provider exit-code mapping including signal exits.
- Implemented raw terminal mode handling using `golang.org/x/term`, with idempotent restore and panic-safety tests.
- Added `App.Run` and `aictl run <provider> [-- provider-args...]`, including `exec.LookPath` validation, concise pre/post UI messages, and typed provider exit-code propagation through `main`.
- Added unit and integration coverage for shell runner behavior, terminal restore, resize hooks, app delegation/missing-provider errors, command argument forwarding, and exit-code preservation.
- Kept later-story scope out: no transcript capture, usage-limit detection, provider adapters, handoff injection, checkpoints, attempts, or fallback chain.

### File List

- `_bmad-output/implementation-artifacts/3-1-run-provider-in-pty.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `cmd/aictl/main.go`
- `cmd/aictl/root.go`
- `cmd/aictl/run.go`
- `cmd/aictl/run_test.go`
- `go.mod`
- `go.sum`
- `internal/app/app.go`
- `internal/app/run.go`
- `internal/app/run_test.go`
- `internal/shell/rawmode.go`
- `internal/shell/resize.go`
- `internal/shell/runner.go`
- `internal/shell/runner_unix.go`
- `internal/shell/runner_windows.go`
- `internal/shell/runner_test.go`
- `internal/shell/signals_unix_test.go`
- `internal/shell/signals_windows_test.go`
- `internal/shell/test_helpers_test.go`

### Change Log

- 2026-06-16: Implemented Story 3.1 - `aictl run <provider>` PTY runner foundation with raw-mode restore, resize propagation, exit-code preservation, command wiring, dependencies, and regression coverage. Story moved to review.
