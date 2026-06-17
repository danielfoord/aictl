---
baseline_commit: 54a0b08
context:
  - _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-16.md
  - _bmad-output/implementation-artifacts/3-3-provider-adapters-trio-config-injection.md
  - _bmad-output/implementation-artifacts/3-4-pre-post-run-checkpoints-durability-ordering.md
---

# Story 3.6: Smart-default handoff injection

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want `aictl run` to inject the handoff prompt only when there's a task to continue,
so that I can launch a provider and drive it myself (e.g. a BMad `/dev-story` or
`/code-review` slash command) without the supervisor hijacking the session — while
still getting auto-continue when I'm resuming in-progress work.

## Background

This story comes from the 2026-06-16 Sprint Change Proposal. Story 3.3 made
handoff injection **always on**: `aictl run <provider>` unconditionally delivers
"Read `.ai-session/handoff.md` and continue the in-progress task…" as the
provider's opening prompt. For sessions the developer means to drive themselves
(BMad slash commands; the goal emerges in-session), that injected prompt hijacks
the provider into reading a goal-less handoff instead of waiting for the user's
command. This story makes injection **conditional on there being a task to
continue**. Capturing a goal after a run already works (`run` is goal-optional and
takes no lock; `aictl start` can set it afterward) — only the forced injection is
the blocker.

## Acceptance Criteria

1. **Fresh run launches clean:** when `aictl run <provider>` runs on a session with
   **no Goal and no recorded progress**, the provider is launched with **no injected
   prompt** (no prompt argument, no PTY-written input), so the user drives it. (FR-17)
2. **Resuming a task injects:** when the session has a **Goal** *or* any recorded
   task-state **progress** (completed steps, decisions, next steps, or known
   failures), `aictl run` injects the handoff per the provider's configured mode —
   exactly as Story 3.3 does today. (FR-17)
3. **Durability unchanged:** the inject/skip decision is computed from **Task State
   only**. `handoff.md` is still generated and persisted, and the pre-run and
   post-run checkpoints are still written on **every** run regardless of the
   decision (NFR-3). Only the prompt delivery to the provider is gated. (FR-8, FR-9)
4. **Bare providers still never inject:** an unknown provider name (run as a bare
   executable) continues to receive no injection (Story 3.3 decision, unchanged). (FR-16)
5. **Mode still decides "how":** when injection does happen, the per-provider mode
   (file-ref/arg/stdin/paste) and its text are applied exactly as before; this story
   only changes *whether* to inject, not *how*. (FR-17)
6. **No other run behavior changes:** provider resolution, missing-executable
   pre-flight, transcript path, quiet supervision, exit-code/`ExitError`/`PostRunError`
   handling, and the files-changed verdict are unchanged. (Regression guard)

## Tasks / Subtasks

- [x] **Task 1 — Task-state "in progress" predicate (AC: 1, 2)**
  - [x] Add a method to `session.TaskState`, e.g. `func (s TaskState) InProgress() bool`,
        returning `true` when `Goal` is non-empty **or** any of `Completed`,
        `Decisions`, `NextSteps`, `KnownFailures` is non-empty.
  - [x] Do **not** count `Branch`, `Dirty` (captured at session start) or
        `VerifyCommands` (configuration) as progress.
  - [x] Unit-test the predicate: empty state → false; goal-only → true; each
        progress list alone → true; branch/dirty/verifyCommands-only → false.
- [x] **Task 2 — Gate injection in `App.Run` (AC: 1, 2, 3, 4, 5)**
  - [x] In `internal/app/run.go`, compute `inject := prepared.State.InProgress()`
        and thread it into the launch decision (see Expected Implementation Shape).
  - [x] When `inject` is **false** for a **known** provider, launch with the
        provider's base args + the user's args only — **no** prompt argument and
        **no** `InitialInput`. When `inject` is **true**, behave exactly as today
        (`p.Inject(paths.Handoff(), userArgs)`).
  - [x] Keep `prepareHandoff` + `session.WriteAtomic(paths.Handoff())` +
        `checkpoint.CapturePre`/`CapturePost` exactly as they are — handoff and
        checkpoints are produced on every run. Only `shell.Options.Args` /
        `InitialInput` are affected.
  - [x] Preserve the unknown-name bare path, the empty-command error, and the
        invalid-mode error in `resolveLaunch`.
- [x] **Task 3 — Tests (AC: 1-6)**
  - [x] `internal/session` test for `TaskState.InProgress()` (Task 1).
  - [x] `internal/app` tests asserting the `shell.Options` handed to the stubbed
        `a.runProvider`:
    - fresh session (no goal/progress) + a **known** provider (e.g. a fake `claude`
      on PATH) ⇒ `got.Args` has no injected prompt and `got.InitialInput` is empty;
    - session with a Goal (via `Start`) ⇒ the handoff prompt is injected (final arg
      contains `handoff.md`);
    - session with only progress (e.g. `Next`/`Note`) and no goal ⇒ injected;
    - in **both** inject and skip cases, `handoff.md` exists and the
      `NNNN-before-*`/`NNNN-after-*` checkpoint dirs are written (durability AC3).
  - [x] Use `t.Chdir(t.TempDir())` for every test that calls `App.Run` (transcript +
        checkpoints write under `.ai-session/`).
  - [x] Run `go test ./...`, `go vet ./...`, `go build ./...`,
        `go test -race ./internal/app ./internal/session ./internal/checkpoint`,
        `gofmt -l`, `task lint` (golangci-lint v2.12.2), and
        `GOOS=windows GOARCH=amd64 go build ./...`.
- [x] **Task 4 — Documentation clarifications (from the Sprint Change Proposal)**
  - [x] PRD `FR-17`: add the consequence that injection happens only when the
        Session has a task to continue (a fresh run launches clean). *(Wording from
        the proposal §4.2.)*
  - [x] `architecture.md` (API & Communication / Provider adapter contract): note
        that *whether* to inject is decided by the run loop from Task State; the mode
        decides *how*; fallback advances (Epic 4) reuse the same predicate.
  - [x] `README.md` (Usage → "How a session is meant to work", step 3): adjust the
        "read the handoff and continue" line to say it applies when resuming
        in-progress work; a fresh run lets you drive the provider.

## Dev Notes

**Sixth story of Epic 3, added by correct-course.** Narrow behavioral refinement of
Story 3.3's injection. No new packages, no new dependencies.

> **Baseline:** `54a0b08`. If Story 3.5's working-tree changes are not yet
> committed, commit them first so this story's diff is clean.

### Scope Boundary

- **In scope:** a `TaskState` "in progress" predicate; gating the injection portion
  of `App.Run` on it; tests; the three doc clarifications above.
- **Out of scope:** an explicit per-run override flag (`--inject`/`--no-inject`) or a
  `none` injection mode — Daniel chose the implicit smart default (Option C); an
  explicit escape hatch can be a later story if needed. Also out: any change to
  handoff generation, checkpoints, transcript, provider resolution, fallback
  (Epic 4), or `aictl start`/goal semantics.

### Reuse / Do Not Reinvent

- **Decide from Task State only.** Do **not** use "does `handoff.md` exist" as the
  signal — `App.Run` rewrites `handoff.md` on every run (`session.WriteAtomic`), so
  it is always present after the first run and would wrongly force injection forever.
  The Task-State predicate only becomes true when the user/provider records a goal or
  progress, which is the stable, correct signal.
- **Reuse the existing injection machinery.** `providers.Provider.Inject` already
  builds the injected args/`InitialInput`; for the skip case just launch with
  `append(p.Args, userArgs...)` and no prompt — do not add a parallel injection path.
- **Keep the predicate in `internal/session`** (on `TaskState`) so Epic 4's fallback
  loop can reuse it — a fallback advance is a "continue" run and should inject via the
  *same* predicate (no special-casing).
- **Keep `cmd/aictl` untouched.** This is an `internal/app` + `internal/session`
  change; the Cobra `run` command needs no edits.

### Expected Implementation Shape

Today (`internal/app/run.go`, ~line 94 and ~127):

```go
command, injection, err := resolveLaunch(providers.Resolve(prepared.Config), name, paths.Handoff(), userArgs)
// ...
res, err := a.runProvider(ctx, shell.Options{
    Command: resolvedPath, Args: injection.Args, /* ... */ InitialInput: injection.InitialInput,
})
```

Target — thread the decision through `resolveLaunch`:

```go
inject := prepared.State.InProgress()
command, injection, err := resolveLaunch(providers.Resolve(prepared.Config), name, paths.Handoff(), userArgs, inject)
```

```go
// resolveLaunch: unknown name → bare (no injection, unchanged); known provider →
// inject only when `inject` is true.
func resolveLaunch(resolved map[string]providers.Provider, name, handoffPath string, userArgs []string, inject bool) (string, providers.Injection, error) {
    p, ok := providers.Lookup(resolved, name)
    if !ok {
        return name, providers.Injection{Args: userArgs}, nil
    }
    if p.Command == "" {
        return "", providers.Injection{}, fmt.Errorf("provider %q has no command configured", name)
    }
    if !p.Mode.Valid() {
        return "", providers.Injection{}, fmt.Errorf("provider %q has unknown injection mode %q ...", name, p.Mode)
    }
    if !inject {
        return p.Command, providers.Injection{Args: append(append([]string(nil), p.Args...), userArgs...)}, nil
    }
    return p.Command, p.Inject(handoffPath, userArgs), nil
}
```

```go
// internal/session/session.go
func (s TaskState) InProgress() bool {
    return s.Goal != "" || len(s.Completed) > 0 || len(s.Decisions) > 0 ||
        len(s.NextSteps) > 0 || len(s.KnownFailures) > 0
}
```

> Note the invalid-mode check still runs even when skipping injection, so a
> misconfigured provider fails fast regardless. (Keep it before the `!inject` branch.)

### Current Files to Modify (read in full before editing)

- `internal/app/run.go` — `App.Run` and `resolveLaunch`. **What changes:** add the
  `inject` parameter + the skip branch; compute `inject` from `prepared.State`.
  **Preserve:** `prepareHandoff` (handoff.md generation), `CaptureGit`/`CapturePre`/
  `CapturePost`, the `LookPath`-before-side-effects ordering, transcript wiring,
  `Mute`/`defer Flush`, `ExitError`/`PostRunError`, and the files-changed line.
- `internal/session/session.go` — `TaskState`. **What changes:** add `InProgress()`.
  **Preserve:** all fields and (un)marshal behavior.
- `internal/app/run_test.go` — extend with inject/skip cases (Task 3).
- Docs: `prd.md` (FR-17), `architecture.md`, `README.md` (Task 4).

### Existing Behavior That Must Not Regress

- A run with a goal/progress still injects exactly as Story 3.3 (file-ref default,
  configurable per provider).
- Unknown provider names still run as bare executables with no injection.
- `handoff.md` and `NNNN-before-*`/`NNNN-after-*` checkpoints are written on every
  run (durability NFR-3), inject or not.
- Missing executable still fails before raw mode / side effects; provider exit codes
  still propagate via `ExitError`; post-run checkpoint failure still via `PostRunError`.
- Quiet supervision and transcript capture unchanged.

### Edge Cases That Must Not Be Missed

- **First run of a fresh session** (no `init`/`start`): `ensureRunSession` writes an
  empty `TaskState` → `InProgress()` is false → no injection. Correct (you drive it).
- **`start` then `run`:** `Start` sets `Goal` (and Branch/Dirty) → `InProgress()` true
  → injects. Correct (resume).
- **Progress without a goal:** a session where only `next`/`note`/`done`/`fail` were
  used (no `start`) → injects. Correct.
- **Branch/Dirty only:** these are set by `Start` *together with* Goal, so they never
  appear without a Goal in practice — but the predicate must still ignore them so a
  hypothetical state with only branch/dirty does not falsely inject.
- **Fallback (Epic 4) later:** the same predicate governs each provider in a chain;
  no fallback-specific override is added here.

### Testing Guidance

- App tests assert the `shell.Options` captured by the `a.runProvider` stub (Args /
  InitialInput), not real provider launches. Reuse `writeExecutable` + PATH and
  `t.Chdir(t.TempDir())` from the existing `internal/app/run_test.go`.
- For the "resume injects" case, seed a goal with `a.Start(ctx, "<goal>")` (or write
  progress with `a.Next`/`a.Note`) before `a.Run`.
- Assert durability in both branches: `handoff.md` exists and a
  `NNNN-before-<provider>/` dir exists after the run.
- Keep `-race` clean (the run path spawns goroutines); add no unsynchronized shared
  state in tests.

### Previous Story Intelligence (3.3, 3.4, 3.5)

- **3.3** built `providers.Resolve`/`Lookup`/`Inject` and `resolveLaunch` (unknown →
  bare; invalid mode → error). This story adds one more decision around `Inject`.
- **3.4** made `App.Run` generate the handoff and capture pre/post checkpoints with
  the live transcript under the after-checkpoint dir, and added `PostRunError`. None
  of that changes — only the prompt delivery is gated.
- **3.5** review lessons that apply here: every `App.Run`/checkpoint test must
  `t.Chdir(t.TempDir())` (a cmd-level test once leaked a real `.ai-session/`), and the
  CI **lint** job (golangci-lint v2.12.2 via `task lint`) is now enforced — run it
  before marking done (a `staticcheck` De-Morgan finding slipped through previously).

### Latest Technical Information

- No new dependencies. Go stays `go 1.26.0`. No network imports (NFR-1; the depguard
  rule in `.golangci.yml` will fail the build if any are added).
- Local gate: `task ci` (fmt:check + vet + lint + race tests) reproduces CI.

### Project Structure Notes

- MODIFIED: `internal/app/run.go`, `internal/session/session.go`,
  `internal/app/run_test.go`, `internal/session/session_test.go` (or wherever
  `TaskState` is tested), and the three docs (`prd.md`, `architecture.md`,
  `README.md`).
- No new files; no changes to `internal/providers`, `internal/shell`, `cmd/aictl`.

### References

- [Source: _bmad-output/planning-artifacts/sprint-change-proposal-2026-06-16.md]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-17]
- [Source: _bmad-output/planning-artifacts/architecture.md#API & Communication — Child process I/O (PTY runner)]
- [Source: _bmad-output/implementation-artifacts/3-3-provider-adapters-trio-config-injection.md]
- [Source: internal/app/run.go]
- [Source: internal/session/session.go]
- [Source: internal/providers/injection.go]

## Review Findings

_Code review 2026-06-16 (Blind Hunter + Edge Case Hunter + Acceptance Auditor; clean diff vs `54a0b08`). All six ACs audited as satisfied. The core gate logic was verified correct by the code-access layers — several Blind-Hunter "High" items were disproved: `prepared.State` loads persisted state; the decision never consults handoff.md (a second fresh run stays fresh); `Start` sets Goal+Branch together (so excluding Branch is safe); `CapturePost` writes no task state (idempotent)._

### Patch

- [x] [Review][Patch] Stale Status summary wording [README.md:22] — FIXED: "auto-injected handoff" → "conditional handoff injection".
- [x] [Review][Patch] Fresh-skip test doesn't assert the post-run checkpoint [internal/app/run_test.go] — FIXED: `TestRunFreshSessionSkipsInjection` now asserts `0001-after-claude/` as well, closing AC3 on the skip branch.

### Deferred

- [x] [Review][Defer] Corrupt/unreadable `state.yaml` hard-fails `aictl run` [internal/app/handoff.go:64-67] — `prepareHandoff` returns the `LoadState` error fatally, while git-capture errors degrade to warnings. A malformed state file blocks launching the provider rather than degrading to a clean run. **Pre-existing** (Story 3.4's `prepareHandoff`); not introduced by 3.6. Deferred as a resilience follow-up.

_Dismissed (verified): `prepared.State` provenance, second-fresh-run flips, Branch-without-Goal, `CapturePost` idempotency (all confirmed correct by the Edge layer); duplicated argv "drift" (the skip base `append(p.Args, userArgs...)` equals `Inject`'s base — no drift); invalid-mode aborts a fresh run (intentional per the story — fail fast on a config typo); empty-string/whitespace predicate edges (start trims; theoretical); unknown-name bare path aliasing + silent no-injection (pre-existing 3.3, by design)._

## Dev Agent Record

### Agent Model Used

Claude Opus 4.8 (claude-opus-4-8)

### Debug Log References

- `go test ./internal/session -run TestTaskStateInProgress` — RED (`InProgress` undefined) → GREEN after adding the method (9 sub-cases).
- `go test ./internal/app -run TestRunFreshSessionSkipsInjection` — RED (injection always on) → GREEN after gating in `App.Run`/`resolveLaunch`.
- `go test ./...` — 177 passed (12 packages). `go vet`, `go build`, `gofmt -l` — clean.
- `go test -race ./internal/app ./internal/session ./internal/checkpoint` — 95 passed, race-clean.
- `task lint` (golangci-lint v2.12.2) — 0 issues. `GOOS=windows GOARCH=amd64 go build ./...` — clean.

### Completion Notes List

- Added `session.TaskState.InProgress()` — true when a Goal or any task-state progress (Completed/Decisions/NextSteps/KnownFailures) exists; Branch/Dirty/VerifyCommands deliberately excluded. Lives in `internal/session` so Epic 4 fallback can reuse it.
- Gated injection in `App.Run`: `inject := prepared.State.InProgress()` threaded into `resolveLaunch(..., inject)`. When false for a known provider, the launch uses base args + user args only (no prompt, no `InitialInput`); when true, behaves exactly as Story 3.3. The invalid-mode and empty-command guards run before the inject branch, so a misconfigured provider still fails fast.
- Durability unchanged: `handoff.md` is still generated/persisted and pre/post checkpoints still written on every run, inject or not (asserted in `TestRunFreshSessionSkipsInjection`). The inject/skip decision uses Task State only — never "does handoff.md exist" (which `run` rewrites every time).
- Unknown (bare-executable) providers continue to receive no injection (unchanged).
- Updated the three existing injection tests to seed a goal (they now assert the *resume* case); added `TestRunFreshSessionSkipsInjection` (skip + durability) and `TestRunProgressWithoutGoalInjects` (progress-only injects).
- Docs (Task 4): clarified PRD FR-17 (when-to-inject), architecture provider-adapter note (whether vs how; fallback reuses the predicate), and README step 3 (fresh = drive it; resume = inject).
- Out of scope (Daniel's Option C): no explicit `--inject`/`--no-inject` flag or `none` mode.

### File List

- `internal/session/session.go` (modified — `TaskState.InProgress()`)
- `internal/session/session_test.go` (new — predicate table test)
- `internal/app/run.go` (modified — `inject` gate in `App.Run` + `resolveLaunch`)
- `internal/app/run_test.go` (modified — fresh-skip + progress-only tests, `seedGoal` helper, resume tests seeded with a goal)
- `_bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md` (modified — FR-17 clarification)
- `_bmad-output/planning-artifacts/architecture.md` (modified — provider-injection note)
- `README.md` (modified — Usage step 3 wording)

## Change Log

- 2026-06-16: Implemented Story 3.6 — smart-default handoff injection. `aictl run` now injects the handoff only when the session has a goal/progress to continue (`TaskState.InProgress()`); a fresh run launches the provider clean so the user drives it. Handoff generation + pre/post checkpoints unchanged. PRD FR-17 / architecture / README clarified. Story moved to review.
- 2026-06-16: Code review (3 layers). All 6 ACs satisfied; core logic verified correct (loud "High" findings disproved by code-access layers). 2 low patches applied (README Status wording; post-checkpoint assertion on the fresh-skip test). 1 pre-existing item deferred (corrupt state.yaml hard-fails run). Status set to done.
