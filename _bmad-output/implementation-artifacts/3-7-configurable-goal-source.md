---
baseline_commit: 54a0b08
context:
  - _bmad-output/planning-artifacts/bmad-continuity-gap-2026-06-16.md
  - _bmad-output/implementation-artifacts/3-6-smart-default-handoff-injection.md
---

# Story 3.7: Configurable goal source (tool-agnostic task continuity)

Status: done

<!-- Definition of done now includes README user docs (see Task 6). -->


<!-- Backfilled spec: the design was settled in the bmad-continuity-gap note + a
short Q&A, then implemented directly. This file documents it in the standard
format so it can be reviewed like the others. -->

## Story

As a developer,
I want aictl to populate the session goal (and task context) from a configurable,
tool-agnostic source,
so that a handover carries real context without me retyping it — regardless of which
workflow drives the provider (a BMad slash command, a Jira/Linear CLI, a plain
`TASK.md`, etc.).

## Background

From the 2026-06-16 BMad continuity-gap note. In a drive-it-yourself run (Story 3.6:
fresh session ⇒ no injection, you type your own command), aictl's durable task memory
(`state.yaml`: goal/plan) is never captured — so a handover carries only git state,
and the next provider gets no meaningful continuation. The fix must be **tool-agnostic**
(aictl special-cases no tool): a user-configured **source** aictl reads to populate the
goal. The BMad-specific bridge prototyped earlier was rejected for coupling aictl core
to one tool.

## Acceptance Criteria

1. **Config surface:** `config.yaml` gains an optional `goalSource` with `file` and/or
   `command` (camelCase, explicit yaml tags). Empty/unset preserves today's manual
   behavior. (Tool-agnostic config contract)
2. **Read the source:** `aictl sync` reads the source — a `file` (resolved relative to
   the repo root) is read; otherwise a `command` is run via `sh -c` in the repo root and
   its **stdout** used. `file` takes precedence. An unconfigured source is a clean no-op.
3. **Parse goal + structured context:** the source text is parsed as structured YAML
   (`goal`, `nextSteps`, `decisions`, `knownFailures`) when it parses and carries a
   non-empty `goal`; otherwise the whole trimmed text becomes the goal (plain-text mode).
4. **Populate without clobbering:** `sync` sets `state.Goal` (and appends the structured
   `nextSteps`/`decisions`/`knownFailures`) **only when the session has no goal**. An
   existing goal is left untouched (goal stays user-owned). Offline; no provider.
5. **Session required:** `sync` returns `ErrNoSession` if `.ai-session/` is absent and
   writes nothing; a configured-but-empty/absent goal yields a concise no-op message.
6. **No tool coupling; explicit-only for now:** no BMad (or any tool) parsing in aictl
   core — the integration lives entirely in the user's `file`/`command`. The explicit
   `aictl sync` command is shipped; auto-populating in `run` (behind a config flag) is
   **out of scope** for this story.

## Tasks / Subtasks

- [x] **Task 1 — Config surface (AC: 1)**
  - [x] Add `GoalSource{File, Command}` and a `GoalSource` field to `config.Config`
        (camelCase yaml tags). `Default()` leaves it zero (discoverable but unset).
  - [x] Round-trip test in `config_test.go`.
- [x] **Task 2 — `internal/goalsource` reader/parser (AC: 2, 3)**
  - [x] `Read(ctx, dir, file, command) (string, bool, error)` — file (relative to dir) or
        `sh -c` command stdout; unset ⇒ `("", false, nil)`. Mirrors the `internal/verify`
        shell-out pattern.
  - [x] `Parse(text) Context` — structured YAML when it parses with a non-empty goal, else
        plain text → goal. `Context{Goal, NextSteps, Decisions, KnownFailures}`.
  - [x] Unit tests: structured, plain, empty, file, command, file-precedence, unset.
- [x] **Task 3 — `App.Sync` orchestration (AC: 4, 5)**
  - [x] `internal/app/sync.go`: require a session (`ErrNoSession`), `config.Load`, read +
        parse the source, and — only if `state.Goal == ""` — set the goal and append the
        structured context, then `session.SaveState`. Report concisely via `a.UI`.
  - [x] Tests: file source (goal + next steps), command source (plain), requires-session,
        leaves-existing-goal, no-source-configured.
- [x] **Task 4 — Thin Cobra command (AC: 2, 6)**
  - [x] `cmd/aictl/sync.go` (`Use: "sync"`, `NoArgs`, delegates to `App.Sync`); register
        `newSyncCmd(a)` in `cmd/aictl/root.go`.
- [x] **Task 5 — Verification**
  - [x] `go test ./...`, `go vet ./...`, `go build ./...`,
        `go test -race ./internal/app ./internal/goalsource ./internal/config`,
        `gofmt -l`, `task lint` (golangci-lint v2.12.2), `GOOS=windows GOARCH=amd64 go build ./...`.
  - [x] End-to-end smoke: `aictl init` → set `goalSource.command` → `aictl sync` populates
        goal + next steps in `state.yaml`.
- [x] **Task 6 — User documentation (definition of done)**
  - [x] Document `aictl sync` in the README Commands table and `goalSource` in the
        Configuration section (structured + plain examples, tool-agnostic note), and
        reference it from the session-lifecycle narrative. A feature is not done until
        it is documented for users.

## Dev Notes

**Seventh story of Epic 3, from correct-course (the BMad continuity gap).** Narrow,
additive, tool-agnostic. Pairs with Story 3.6 (smart default) and the Epic 4 fallback
(the auto-switch half).

### Scope Boundary

- **In scope:** the `goalSource` config, the `internal/goalsource` reader/parser, `App.Sync`,
  and the `aictl sync` command. Goal + structured context (next-steps/decisions/known-failures).
- **Out of scope:** auto-populating the goal during `run` (deferred behind a future config
  flag); any tool-specific parsing in aictl (BMad/Jira/etc. lives in user config); the
  auto-fallback half (Epic 4); changing `run`/handoff/checkpoint behavior.

### Reuse / Do Not Reinvent

- **Mirror `internal/verify`** for running the source command (`sh -c`, capture output) —
  same trust model and pattern.
- **Reuse `config.Load`, `session.LoadState`/`SaveState`, `ErrNoSession`** as the other
  commands do; `aictl sync` is a thin command over `App.Sync` (logic in `internal/app`).
- **No tool coupling in core** — `internal/goalsource` knows only "a file or a command";
  the BMad/Jira/file specifics are the user's `goalSource` value.
- Do **not** overwrite an existing goal (it stays user-owned; consistent with the
  immutable-goal stance).

### Current Files (this story)

- NEW: `internal/goalsource/goalsource.go` (+ `_test.go`), `internal/app/sync.go`
  (+ `sync_test.go`), `cmd/aictl/sync.go`.
- MODIFIED: `internal/config/config.go` (+ `config_test.go`) — `GoalSource`; `cmd/aictl/root.go`
  — register `sync`.

### Edge Cases

- **No `goalSource` configured:** clean no-op message, no error.
- **Existing goal:** left untouched; report the source goal but do not apply it.
- **Source yields no goal** (empty file/command, or structured-without-goal): no-op message.
- **`file` and `command` both set:** `file` wins (documented).
- **Command fails / file missing:** surfaced as a wrapped error (the source is misconfigured).
- **Markdown/plain file:** falls back to plain-text mode (whole trimmed text → goal).
- **Session required:** `.ai-session/` absent ⇒ `ErrNoSession`, nothing written.

### Testing Guidance

- App tests use `t.Chdir(t.TempDir())`; a real session has `state.yaml`, so the test helper
  seeds an empty one alongside the `goalSource` config.
- Prefer deterministic command sources in tests (`printf`).
- Keep `-race` clean.

### Previous Story Intelligence

- **3.6** added `TaskState.InProgress()` and the smart-default gate; this story's whole point
  is to make `InProgress()` true (with real context) without manual typing, so 3.6 then
  injects "continue" into the next provider.
- Repo conventions: thin `cmd/` → `internal/app` → feature package; `config.Load` pattern;
  `task lint` (golangci-lint v2.12.2) is an enforced gate; every `App` test that writes under
  `.ai-session/` must `t.Chdir(t.TempDir())`.

### References

- [Source: _bmad-output/planning-artifacts/bmad-continuity-gap-2026-06-16.md]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-4 (provider-optional task state) / FR-11 (handoff)]
- [Source: internal/verify/verify.go (shell-out pattern)]
- [Source: internal/app/handoff.go (config-load pattern)]
- [Source: internal/session/session.go (TaskState)]

## Review Findings

_Code review 2026-06-17 (Blind Hunter + Edge Case Hunter + Acceptance Auditor; clean isolated diff vs `54a0b08`). All six ACs audited as satisfied; constraints (no tool coupling, no new deps, no networking, out-of-scope `run` untouched) confirmed._

### Patch

- [x] [Review][Patch] `Parse` corrupts the goal for structured-without-goal sources [internal/goalsource/goalsource.go] — FIXED: the parse is accepted as structured when it unmarshals and **any** field is set (goal or any list), so a goal-less structured source keeps its fields and `Sync` cleanly no-ops on the empty goal instead of jamming raw YAML into `Goal`. Covered by `TestParseStructuredWithoutGoalKeepsFields`.
- [x] [Review][Patch] Command-failure diagnostics drop stderr [internal/goalsource/goalsource.go `Read`] — FIXED: on `*exec.ExitError` the captured stderr is appended to the wrapped error. Covered by `TestReadCommandFailureIncludesStderr`.

### Deferred

- [x] [Review][Defer] A plain-text source with no `goal:` makes the whole (possibly multi-line) file the goal [internal/goalsource/goalsource.go `Parse`] — documented plain-text behavior (the user controls their source); a length cap / first-line extraction is a future refinement. Deferred.
- [x] [Review][Defer] No timeout on the `goalSource` command [internal/goalsource/goalsource.go] — cancelable via ctx (SIGINT) but no deadline; **consistent with `internal/verify`**, so a bounded-timeout policy is a project-wide follow-up, not a 3.7 regression. Deferred.
- [x] [Review][Defer] `.ai-session/` present but `state.yaml` missing yields a raw `LoadState` error instead of `ErrNoSession` [internal/app/sync.go] — only reachable on a hand-damaged session (`init`/`run` always write `state.yaml`); low robustness gap. Deferred.

_Dismissed (verified): the `%q`/arg "mismatch" (false positive — the real format string has two `%q`; `go vet` clean); `file`+`command` both-set precedence (documented file-wins; configuring both is unusual); append-without-dedup (gated on empty goal; gold-plating); the `!ok` "no output" branch (harmless defensive guard; the no-source case is already guarded earlier); `goalSource` serializing as an empty block (intentional discoverability, no `omitempty`); success message not counting decisions/known-failures (cosmetic)._

## Dev Agent Record

### Agent Model Used

Claude Opus 4.8 (claude-opus-4-8)

### Debug Log References

- `go test ./internal/config -run TestGoalSourceRoundTrips` — GREEN (config field).
- `go test ./internal/goalsource` — GREEN (7 tests: structured/plain/empty parse; file/command/precedence/unset read).
- `go test ./internal/app -run TestSync` — GREEN (5 tests: file goal+context, command plain, requires-session, leaves-existing-goal, no-source).
- `go test ./...` — GREEN. `go vet`, `go build`, `gofmt -l` — clean.
- `go test -race ./internal/app ./internal/goalsource ./internal/config` — race-clean.
- `task lint` (golangci-lint v2.12.2) — 0 issues. `GOOS=windows GOARCH=amd64 go build ./...` — clean.
- Smoke: `aictl init` → `goalSource.command` printing structured YAML → `aictl sync` ⇒ `state.yaml` goal + 2 next steps.

### Completion Notes List

- `config.GoalSource{File, Command}` added (optional; `Default()` leaves it unset, serialized as an empty block for discoverability).
- `internal/goalsource`: tool-agnostic `Read` (file or `sh -c` command stdout, file-precedence) + `Parse` (structured YAML with a non-empty goal, else plain text → goal).
- `App.Sync`: requires a session, reads+parses the configured source, and populates goal + structured context only when the goal is empty (never overwrites); concise UI report.
- `aictl sync` thin command registered in root.
- Deferred (per the agreed "both" trigger): auto-populate the goal in `run` when empty, behind a config flag.
- Documented `aictl sync` + `goalSource` in the README (Commands, Configuration with structured/plain examples + the tool-agnostic note, and the lifecycle narrative) — a feature is not done until it's documented.

### File List

- `internal/config/config.go` (modified — `GoalSource` type + field)
- `internal/config/config_test.go` (modified — round-trip test)
- `internal/goalsource/goalsource.go` (new — `Read` + `Parse` + `Context`)
- `internal/goalsource/goalsource_test.go` (new)
- `internal/app/sync.go` (new — `App.Sync`)
- `internal/app/sync_test.go` (new)
- `cmd/aictl/sync.go` (new — `aictl sync`)
- `cmd/aictl/root.go` (modified — register `sync`)
- `README.md` (modified — `aictl sync` + `goalSource` user docs)

## Change Log

- 2026-06-17: Implemented Story 3.7 — tool-agnostic configurable goal source (`goalSource` config, `internal/goalsource`, `App.Sync`, `aictl sync`). Explicit command only; auto-in-run deferred. Spec backfilled to match the other stories. Status: review.
- 2026-06-17: Code review (3 layers). All ACs satisfied; the loudest finding (a `%q`/arg mismatch) was a false positive from the review-prompt truncation. 2 patches applied (structured-without-goal no longer corrupts the goal; command-stderr surfaced in errors) + 2 tests; 3 items deferred.
- 2026-06-17: Added README user docs for `aictl sync` + `goalSource` (a story is not done until documented). Status: done.
