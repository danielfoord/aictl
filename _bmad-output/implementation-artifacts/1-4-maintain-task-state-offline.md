---
baseline_commit: 037749cb025239cc2d4a713c5cb73a74841f0ad5
---

# Story 1.4: Maintain Task State offline (`note` / `done` / `next` / `fail`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to record decisions, completed steps, next steps, and failures without any provider running,
so that my task state never depends on a live provider or network — it's always there for the next handoff.

## Acceptance Criteria

1. **Four credit-free commands:** `aictl note "<decision>"`, `aictl done "<completed step>"`, `aictl next "<next step>"`, and `aictl fail "<known failure>"` each append the message to the correct Task State field and persist immediately. (FR-3)
   - Mapping: `note` → `decisions`, `done` → `completed`, `next` → `nextSteps`, `fail` → `knownFailures`.
2. **Offline & provider-free:** each command completes with no network access and no provider invoked. (FR-3, NFR-1)
3. **Atomic, hand-editable persistence:** updates are written via `session.SaveState` (→ `WriteAtomic`); `state.yaml` remains human-readable and hand-editable. (NFR-3, NFR-4)
4. **Provider-contributed updates are consumed, not clobbered:** because each command loads the current `state.yaml` (which a provider may have edited), then appends and saves, prior content — including provider-written entries — is preserved. A Session with zero provider updates still yields complete Task State from user input alone. (FR-4)
5. **Robust to malformed state:** a malformed/corrupt `state.yaml` produces a clear error and is **not** overwritten with garbage (no corruption). (FR-4)
6. **Requires a session; clear feedback:** if no `.ai-session/` exists, the command fails with a clear message (e.g. "no aictl session here; run `aictl start` first") and non-zero exit. On success it gives concise confirmation.

## Tasks / Subtasks

- [x] **Task 1 — State-append use-cases in `internal/app` (AC: 1, 2, 3, 4, 6)**
  - [x] Add `internal/app/state.go` with the four use-cases. Implement them over a single private helper to avoid duplication, e.g. `func (a *App) appendEntry(ctx, field stateField, value string) error` plus thin `Note/Done/Next/Fail` wrappers — OR four small methods sharing a helper. (DRY: don't copy the load→append→save body four times.)
  - [x] Each: resolve cwd → `session.NewPaths` → require `.ai-session/` exists (else a clear sentinel error, e.g. `ErrNoSession`) → `session.LoadState` → append trimmed value to the mapped slice → `session.SaveState`.
  - [x] Reject empty/whitespace values with a clear error (consistent with `Start`'s empty-goal handling).
  - [x] Print concise confirmation (e.g. `noted decision: "<value>"`).
- [x] **Task 2 — Commands in `cmd/aictl/state.go` (AC: 1, 6)**
  - [x] Add thin Cobra subcommands `note`, `done`, `next`, `fail`, each `cobra.ExactArgs(1)`, calling the matching `app` method. No logic in `cmd/`. Group them in one file (`cmd/aictl/state.go`) per the architecture tree.
  - [x] Register all four on the root command in `root.go`.
- [x] **Task 3 — Tests (AC: 1–6)**
  - [x] `internal/app/state_test.go` (`t.Chdir` + `t.TempDir`): after `init` (or `start`), each command appends to the right field and persists; multiple appends accumulate in order; values round-trip through YAML; no network.
  - [x] No-session case: command without `.ai-session/` returns the `ErrNoSession` sentinel.
  - [x] Preservation (FR-4): pre-write a `state.yaml` containing provider-style entries, run a command, assert the pre-existing entries are retained alongside the new one.
  - [x] Empty value rejected.

### Review Findings

- [x] [Review][Decision] Concurrent state updates can lose entries — Resolved by adding a dedicated transient `.state.lock` for Task State mutations, separate from the active session `.lock`. Evidence: `internal/app/state.go`, `internal/session/paths.go`.
- [x] [Review][Patch] Canceled command can still mutate state [`internal/app/state.go:74`]

## Dev Notes

**Depends on Stories 1.1–1.3 (all `done`).** This story is small: it composes existing primitives. **Reuse, don't reinvent:**
- `session.LoadState(path)` / `session.SaveState(path, state)` already exist (Story 1.3) — `SaveState` routes through `WriteAtomic`. Use them; do **not** call `os.WriteFile` or re-marshal by hand.
- `session.NewPaths(root)` for all `.ai-session/` paths. `session.TaskState` already has the four target slices (`Completed`, `Decisions`, `NextSteps`, `KnownFailures`).
- The `App` method + thin Cobra command pattern is established (`internal/app/{init,start}.go` ↔ `cmd/aictl/{init,start}.go`, registered in `root.go`). Mirror it exactly.

### Field mapping (do not deviate)

| Command | TaskState field (yaml) |
|---|---|
| `note`  | `Decisions` (`decisions`) |
| `done`  | `Completed` (`completed`) |
| `next`  | `NextSteps` (`nextSteps`) |
| `fail`  | `KnownFailures` (`knownFailures`) |
[Source: prd.md FR-3; architecture.md (`cmd/aictl/state.go`)]

### Session requirement & the lock

- These commands operate on an existing Session's Task State. **Require `.ai-session/` to exist**; if absent, return a clear `ErrNoSession` and exit non-zero (do **not** auto-init — recording into a non-existent session is a user error; auto-init is `start`'s job). They work after either `aictl init` or `aictl start`.
- **Do NOT acquire the single-session lock** for these. The lock marks an *active session* (held from `start`); recording state is allowed whenever the Session Directory exists and must not be gated on or mutate the lock. [Source: architecture.md#Data Architecture; Story 1.3 lock semantics]

### FR-4 — consume-on-load, no clobber

- The load→append→save flow inherently consumes whatever is in `state.yaml` at the time (including provider-written entries) and preserves it. That *is* the FR-4 mechanism — no extra merge logic needed for v1.
- **Malformed `state.yaml`:** `LoadState` will return an unmarshal error → the command fails with a clear message and writes nothing (no corruption). Full lenient/strict-field handling (reject unknown fields, tolerate partial) is a **deferred** item (see `deferred-work.md`, "strict YAML unmarshal" → routed here/1.3); for this story, "clear error, no overwrite" satisfies AC 5. Do not silently discard a malformed file.

### Conventions (inherited — all stories enforce these)

- Atomic writes via `session.WriteAtomic` only (through `SaveState`); thin `cmd/`; `context.Context` first; wrap errors with `%w`; `cmd` is the only layer that prints errors / sets exit code; camelCase yaml tags (already on `TaskState`). [Source: architecture.md#Implementation Patterns]
- **No network** — depguard + `internal/guard` test enforce NFR-1; these commands import only `os`, `session`, stdlib. [Source: architecture.md NFR-1]

### Learnings carried from Stories 1.1–1.3 (real, not theoretical)

- **gofmt import-block alignment** tripped CI twice — run `gofmt -w` before finishing; CI's gofmt gate is scoped to `git ls-files '*.go'`.
- **errcheck (golangci-lint v2.12.2) is enforced** — never drop an error return; use `_, _ =` only with a deliberate comment (as in `ui.Printf`). Check returns from `LoadState`/`SaveState`.
- **Tests use `t.Chdir(t.TempDir())`** (Go 1.26) and `ui.New(io.Discard, io.Discard)` via the `newTestApp()` helper already in `internal/app/*_test.go` — reuse it.
- **Code-review recurring theme:** partial-failure robustness. These commands are a single load→append→save with no lock interaction, so the wedge risk from 1.2/1.3 doesn't apply — but still keep the write atomic (already handled by `SaveState`).
- Each prior story added its subcommand to `root.go` via `rootCmd.AddCommand(...)`; add the four here the same way.

### Project Structure Notes

- NEW: `internal/app/state.go`, `internal/app/state_test.go`, `cmd/aictl/state.go`. MODIFIED: `cmd/aictl/root.go` (register 4 subcommands). No new dependencies. No changes to `TaskState` shape (fields already exist).

### References

- [Source: epics.md#Epic 1 → Story 1.4]
- [Source: prd.md#4.1 — FR-3, FR-4]
- [Source: architecture.md#Data Architecture; #Implementation Patterns; #Cross-Cutting NFRs (NFR-1/3/4)]
- [Source: implementation-artifacts/1-3-start-session-with-goal.md — LoadState/SaveState, app+cmd pattern, lock semantics]

### Open Questions

1. **No-session behavior** — error (recommended, implemented as `ErrNoSession`) vs auto-init. Recommend **error**: these record into an existing session; `start`/`init` create one.
2. **Confirmation verbosity** — one concise line per command is assumed; confirm tone/format if you have a preference.

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- `go test ./internal/app` red phase failed on missing `App.Note/Done/Next/Fail` and `ErrNoSession`.
- `go test ./cmd/aictl` red phase failed on missing `note` command registration.
- `go test ./internal/app` passed after adding app use-cases.
- `go test ./cmd/aictl` passed after adding Cobra commands and root registration.
- `go build ./...` passed with `GOCACHE=/private/tmp/aictl-gocache`; Go emitted a non-fatal module stat-cache permission warning for `/Users/daniel/go/pkg/mod`.
- `go test -race -shuffle=on -coverprofile=/private/tmp/aictl-coverage.out -covermode=atomic ./...` passed.
- `go vet ./...` passed.
- `gofmt -l $(git ls-files "*.go")` returned no files.
- `golangci-lint run` could not be executed locally because `golangci-lint` is not installed.
- Review patch red phase: `go test ./internal/app` failed on ignored canceled context and lost concurrent appends.
- Review patch verification: `go test ./internal/app` passed after adding context checks and `.state.lock` serialization.
- Review patch full verification: `go build ./...`, `go test -race -shuffle=on -coverprofile=/private/tmp/aictl-coverage.out -covermode=atomic ./...`, `go vet ./...`, and gofmt check passed.

### Implementation Plan

- Add a single app-layer append helper that trims input, requires an existing `.ai-session/`, loads the current `state.yaml`, appends to the mapped `TaskState` slice, and persists via `session.SaveState`.
- Keep command files thin: Cobra validates one argument and delegates directly to `internal/app`.
- Cover field mapping, accumulation, YAML round-trip readability, no-session errors, provider-entry preservation, malformed-state no-clobber behavior, empty input rejection, command registration, and command output.

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.
- Implemented `App.Note`, `App.Done`, `App.Next`, and `App.Fail` over a shared load-append-save helper.
- Added `ErrNoSession` with clear feedback when `.ai-session/` is missing.
- Added thin Cobra commands `note`, `done`, `next`, and `fail`, and registered them on the root command.
- Added app and command tests covering AC1-AC6, including malformed `state.yaml` not being overwritten.
- Verified build, race+shuffle tests, vet, and gofmt locally; `golangci-lint` remains unavailable in this environment.
- Addressed code review findings: canceled contexts now stop before mutation, and Task State append commands serialize through a dedicated transient `.state.lock`.
- `.state.lock` is ignored in newly initialized session directories so transient mutation locks are not committed.

### File List

- `_bmad-output/implementation-artifacts/1-4-maintain-task-state-offline.md` (modified)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified)
- `cmd/aictl/root.go` (modified)
- `cmd/aictl/root_test.go` (modified)
- `cmd/aictl/state.go` (new)
- `internal/app/init.go` (modified)
- `internal/app/init_test.go` (modified)
- `internal/app/state.go` (new)
- `internal/app/state_test.go` (new)
- `internal/session/paths.go` (modified)

### Change Log

- 2026-06-15: Implemented Story 1.4 offline Task State commands (`note`, `done`, `next`, `fail`) with app-layer persistence, Cobra wiring, and tests.
- 2026-06-15: Addressed code review findings with canceled-context handling and dedicated `.state.lock` serialization for Task State mutations.
