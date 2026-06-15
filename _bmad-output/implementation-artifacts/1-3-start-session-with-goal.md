---
baseline_commit: 3f46f8ce6ead0ad59e9155fa9b62e243d77cfd2f
---

# Story 1.3: Start a Session with a Goal (`aictl start`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to run `aictl start "<goal>"`,
so that my task has a recorded, immutable objective and aictl knows my starting repo state.

## Acceptance Criteria

1. **Records an immutable Goal:** `aictl start "Refactor the classifier"` persists the Goal verbatim to Task State; the Goal is immutable for the Session's life (a later `start` does not silently change it). (FR-2)
2. **Captures initial repo context:** starting a Session records the initial branch and clean/dirty status. (FR-2)
3. **Single active Session per working tree:** if a Session is already active in this working tree, a second `aictl start` is refused with a clear message and non-zero exit (lockfile enforced). (FR-2, PRD Glossary)
4. **Deterministic behavior without prior `init`:** running `aictl start` when `.ai-session/` does not exist behaves deterministically and as documented — either auto-initializes the Session Directory or instructs the user to run `aictl init`. (FR-2)
5. **Atomic, offline:** Task State is persisted via `store.WriteAtomic`; `start` performs no network access and invokes no provider. (NFR-1, NFR-3)
6. **Clear feedback:** on success, `start` prints a concise confirmation (goal + branch). On refusal (AC 3), it explains why.

## Tasks / Subtasks

- [ ] **Task 1 — Single-session lock (AC: 3, NFR-3)**
  - [x] Created `internal/session/lock.go`: `AcquireLock` (O_CREATE|O_EXCL — race-free) writing pid+timestamp; `ReleaseLock`; `ErrSessionLocked` sentinel.
  - [x] "Active Session" = lock present; documented in code that the lock persists (not released per-command in v1) until the Session is reset.
- [x] **Task 2 — Minimal git context read (AC: 2)**
  - [x] Created `internal/git/git.go` (shell-out via `os/exec`): `Branch` (`git branch --show-current`), `IsDirty` (`git status --porcelain`), `Capture` → `Snapshot{Branch,Dirty}`. (Used `--show-current` over `rev-parse` so it works on an unborn branch.)
  - [x] Scope fence respected: no diff/commits/denylist (Story 2.1).
  - [x] "Not a git repo" → `Capture` returns an error; `Start` treats git capture as best-effort and continues (records empty branch).
- [x] **Task 3 — Goal + immutability on Task State (AC: 1)**
  - [x] `Start` sets `TaskState.Goal` and persists via `session.SaveState` (→ `WriteAtomic`).
  - [x] Immutability enforced via the lock: a second `start` fails at `AcquireLock` (ErrSessionLocked) before touching the Goal. Added `Branch`/`Dirty` camelCase fields to `TaskState`; added `LoadState`/`SaveState` helpers.
- [x] **Task 4 — `aictl start` command (AC: 1–6)**
  - [x] Created `cmd/aictl/start.go`: thin subcommand (`cobra.ExactArgs(1)`) calling `app.Start(ctx, goal)`.
  - [x] Implemented `App.Start`: getwd → auto-init if missing → acquire lock → load state → set Goal + capture context → save → success message.
  - [x] AC-4: **auto-init if absent, then proceed** (resolved open question).
- [x] **Task 5 — Tests (AC: 1–5)**
  - [x] `internal/session/lock_test.go`: acquire on fresh; second acquire → ErrSessionLocked; release frees; release-missing is no-error.
  - [x] `internal/app/start_test.go`: goal persisted; immutable on re-start (ErrSessionLocked, goal unchanged); branch captured; auto-init w/o prior init; empty goal rejected; works outside a git repo (empty branch).
  - [x] `internal/git/git_test.go`: branch + dirty detection against a temp `git init` repo; non-repo returns error.

### Review Findings (code review 2026-06-15)

_3 adversarial layers. Acceptance Auditor: all 6 ACs PASS, all conventions PASS. Both hunters converged on a real lock-wedge robustness issue._

**Patch (applied 2026-06-15):**

- [x] [Review][Patch] Release the lock on any post-acquire failure in `Start` (`defer` + `committed` flag; lock persists only on success) — failures are now recoverable, not a permanent wedge [internal/app/start.go] (+ `TestStartReleasesLockOnFailure`)
- [x] [Review][Patch] Capture git context BEFORE auto-init so `dirty` is faithful to the user's tree at session start (verified: clean repo now reports `dirty: false`) [internal/app/start.go]
- [x] [Review][Patch] Pinned `git status --porcelain --untracked-files=normal` for deterministic dirty detection [internal/git/git.go]

**Deferred (tracked in deferred-work.md):**

- [x] [Review][Defer] Stale-lock recovery for crash/SIGKILL (pid-liveness check or an `aictl unlock`/`--force`) — the defer-rollback above covers in-process failures, but an abnormal termination still strands the lock; no recovery command exists in v1 → follow-up story
- [x] [Review][Defer] git subprocess timeouts + error classification (distinguish "not a git repo" from a real git failure and surface the latter) → Story 2.1 (git-layer maturation)
- [x] [Review][Defer] Resolve the true repo root (walk up to `.git` / existing `.ai-session`) + warn when run outside a git repo — consolidate with the 1.2-deferred repo-root item → Stories 2.1 / shared helper

**Dismissed:** "Goal immutable but overwritten" (false positive — lock blocks the second `start` before the goal line; immutability tested); detached-HEAD label conflation (`--show-current` yields names for unborn branches, so empty ≈ detached in a real repo — label is accurate); lock fsync / discarded `Fprintf` error (lock *existence* is the contract via O_EXCL; contents are best-effort diagnostics); `LoadState` strictness (kept strict for future consumers; the lock-release patch makes failures recoverable).

## Dev Notes

**Depends on Stories 1.1 (foundation) and 1.2 (`session.Paths`, `store.WriteAtomic`, `Config`/`TaskState` types, `aictl init`).** Reuse them — do not recreate. The `TaskState.goal` field and the Session Directory already exist from 1.2; this story sets the Goal and adds the lock + minimal git read.

### What this story creates / touches

**NEW:**
```
cmd/aictl/start.go            # thin `aictl start "<goal>"` subcommand
internal/session/lock.go      # single-session lockfile (.ai-session/.lock)
internal/git/git.go           # shell-out git wrapper — MINIMAL slice only (branch + dirty)
+ tests co-located
```
**UPDATE (extend, preserve existing behavior):**
```
internal/session/session.go   # set Goal (immutable) + initial branch/dirty fields on TaskState
internal/app/...              # add Start use-case alongside Init
internal/session/paths.go     # add Lock path if not already present
```
[Source: architecture.md#Project Structure & Boundaries]

### Files being extended — current state & what to preserve

- **`internal/session/session.go`** (from 1.2): defines `TaskState` with empty-state creation. *This story:* add Goal-setting with immutability + initial-context fields. *Preserve:* the empty-state init path used by `aictl init` (1.2) must still produce a valid empty TaskState. Add fields with `yaml:` tags; don't rename existing ones.
- **`internal/app`** (from 1.2): has the `Init` use-case. *This story:* add `Start`; *preserve* `Init` behavior and the thin-cmd wiring.
- **`internal/session/paths.go`** (from 1.2): centralized paths. *This story:* ensure `.ai-session/.lock` is resolved here (add if missing); *preserve* existing path resolution.

> ⚠️ The dev agent owns leaving the system working end-to-end: after this story `aictl init` (1.2) must still work, and `start` must compose cleanly with it.

### Critical conventions (architecture — already in force from 1.1/1.2)

- **Git access = shell-out** to the real `git` binary via `os/exec` (the project-wide decision). `os/exec` is permitted by the no-network depguard rule; do not add networking imports. [Source: architecture.md#API & Communication ("Git access: shell out"), #Decision Priority Analysis]
- **Atomic writes** through `session.store.WriteAtomic`; no direct `os.WriteFile` for Session files. [Source: architecture.md#Format Patterns]
- **Centralized paths**, **thin `cmd/`**, **no global state**, **`context.Context` first**, **error wrapping `%w`**, **`cmd` prints errors / sets exit code**. [Source: architecture.md#Implementation Patterns]
- **YAML `camelCase`** with explicit tags on any new serialized field. [Source: architecture.md#Format Patterns]
- **One active Session per working tree** is a PRD-level invariant; the lockfile is its mechanism. [Source: prd.md#Glossary (Session); architecture.md#Data Architecture ("Concurrency guard … lockfile")]

### Goal immutability (FR-2)

The Goal is "the original task description … immutable for the Session's life." [Source: prd.md#Glossary (Goal)] Enforce in `session`, not just `cmd`: once `TaskState.goal` is non-empty, `start` must not overwrite it.

### Library notes

- No new dependencies. `os/exec` (stdlib) for git; goccy/go-yaml (from 1.2) for persistence. Pin nothing new.
- Git invocation: prefer explicit args, capture stdout/stderr, wrap errors with context. Trim trailing newlines from `git` output.

### Project Structure Notes

- Introducing `internal/git` here (a story earlier than its "home" Epic 2) is deliberate and scoped: only the branch/dirty slice. Story 2.1 expands the same package with diff/commits/denylist. Documented so 2.1's create-story knows `internal/git/git.go` already exists.
- Carries forward the **module path / license** open item from 1.1.

### Testing standards

- Go stdlib `testing`, table-driven; `t.TempDir()` and a throwaway git repo (`git init` in the temp dir) for context/lock tests. Assert immutability, lock refusal, branch/dirty capture, AC-4 behavior, and no network. [Source: architecture.md#Starter Template Evaluation → Testing]
- CI must stay green.

### References

- [Source: epics.md#Epic 1 → Story 1.3]
- [Source: prd.md#4.1 Session lifecycle & task state — FR-2; #Glossary (Session, Goal)]
- [Source: architecture.md#Data Architecture (lockfile, atomic writes); #API & Communication (git shell-out); #Implementation Patterns]
- [Source: architecture.md#Cross-Cutting NFRs — NFR-1, NFR-3]

### Open Questions

1. **No-`init` behavior (AC 4):** auto-init then proceed, or instruct the user to run `init` first? Recommend **auto-init then proceed** (smoother first-run); confirm with maintainer.
2. **Stale lock policy:** how to treat a `.lock` left by a crashed process (pid liveness check? age-based override? `--force` flag?). Recommend recording pid+timestamp and offering a clear override path; confirm.
3. **Module path / license** — still pending from Story 1.1.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (BMad dev-story workflow)

### Debug Log References

- `go mod tidy` clean; `go build ./...` ok; `go vet ./...` no issues; `gofmt` clean (after `gofmt -w` on session.go)
- `go test ./...` → 29 passed across 7 packages
- Smoke (temp git repo): `aictl start "Refactor the classifier"` → auto-inits, `Started session`, `branch: main (dirty: true)`, exit 0; `state.yaml` has goal+branch+dirty; `.lock` holds `pid` + `acquiredAt`; second `start` → "a session is already active …" exit 1.

### Completion Notes List

- **Open questions resolved:** (1) no-`init` behavior → **auto-init then proceed**; (2) lock policy → lock = **persistent active-Session marker**, created with `O_EXCL` (race-free), recording pid+timestamp; not released per-command in v1 (released when the Session is reset by a future command). Goal immutability falls out of this: a second `start` is refused at lock acquisition before the Goal is touched.
- **Minimal `internal/git` slice introduced** (its "home" is Epic 2): `Branch`/`IsDirty`/`Capture` only — diff/commits/denylist remain Story 2.1. Used `git branch --show-current` (works on unborn branches). Git capture in `Start` is **best-effort**: starting outside a git repo still succeeds (records empty branch), so `start` isn't blocked pre-`git init`.
- **Documented convention exception:** the lockfile bypasses `session.WriteAtomic` because it needs atomic exclusive *create* (`O_EXCL`), which temp-file+rename can't provide. All durable state artifacts still go through `WriteAtomic`.
- Added `Branch`/`Dirty` fields to `TaskState` (camelCase tags) + `LoadState`/`SaveState` helpers. Registered `start` subcommand on root.
- Conventions honored: thin `cmd/`, context-first, `%w` wrapping, no network (guard test green), camelCase yaml.
- Note on smoke output: `dirty: true` is correct — the untracked `.ai-session/` makes the working tree dirty. In real use `.ai-session/`'s transient parts are gitignored within the dir; whether to ignore the whole dir at repo root is a user choice.

### File List

- `internal/session/lock.go` (new)
- `internal/session/lock_test.go` (new)
- `internal/git/git.go` (new)
- `internal/git/git_test.go` (new)
- `internal/app/start.go` (new)
- `internal/app/start_test.go` (new)
- `cmd/aictl/start.go` (new)
- `internal/session/session.go` (modified — Branch/Dirty fields, LoadState/SaveState)
- `cmd/aictl/root.go` (modified — register start subcommand)
- `go.mod` / `go.sum` (unchanged deps; tidy)

### Change Log

- 2026-06-15: Implemented Story 1.3 — `aictl start "<goal>"` (FR-2). Added single-session lockfile (`session.AcquireLock`, race-free O_EXCL), a minimal shell-out `internal/git` slice (branch + dirty), `TaskState` Branch/Dirty + Load/Save, and the `App.Start` use-case + command. Auto-inits if needed; immutable Goal enforced via the lock. 29 tests passing.
- 2026-06-15: Addressed code review — 3 patches applied (lock released on post-acquire failure; git captured before auto-init; pinned `--untracked-files=normal`). 3 findings deferred (stale-lock crash recovery, git timeouts/error-classification, repo-root resolution). 30 tests passing.
