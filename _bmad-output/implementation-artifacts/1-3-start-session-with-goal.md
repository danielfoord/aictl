# Story 1.3: Start a Session with a Goal (`aictl start`)

Status: ready-for-dev

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
  - [ ] Create `internal/session/lock.go`: acquire/release a lockfile at `Paths.Lock` (`.ai-session/.lock`). Acquisition fails clearly if a live lock exists (detect/handle stale locks pragmatically — e.g. record pid/timestamp).
  - [ ] "Active Session" for v1 = lock present; document the chosen staleness policy.
- [ ] **Task 2 — Minimal git context read (AC: 2)**
  - [ ] Create `internal/git/git.go` (shell-out wrapper around the `git` binary via `os/exec`) **and only the minimal reads this story needs**: current branch and clean/dirty status (`git rev-parse --abbrev-ref HEAD`, `git status --porcelain`).
  - [ ] **Scope fence:** do NOT implement diff, recent-commits, `maxDiffChars` bounding, or the secret denylist here — those are Story 2.1. Create just the slice 1.3 requires so there is no forward dependency on 2.1.
  - [ ] Handle "not a git repo" gracefully (clear error or recorded "no branch" — document choice).
- [ ] **Task 3 — Goal + immutability on Task State (AC: 1)**
  - [ ] Set `TaskState.goal` (field exists from Story 1.2) on `start`; persist via `WriteAtomic`.
  - [ ] Enforce immutability: if a Goal is already set, `start` does not overwrite it — refuse (AC 3 path) or no-op with a clear message. Capture initial branch/dirty into Task State (add fields if needed, with `yaml:"camelCase"` tags).
- [ ] **Task 4 — `aictl start` command (AC: 1–6)**
  - [ ] Create `cmd/aictl/start.go`: thin Cobra subcommand taking the goal as a required positional arg; calls `app.Start(ctx, goal)`. No logic in `cmd/`.
  - [ ] Implement the `Start` use-case in `internal/app`: acquire lock → resolve/handle missing `.ai-session/` (AC 4) → capture git context → set immutable Goal → persist → success message.
  - [ ] Decide and implement the AC-4 behavior (recommended: auto-init if absent, then proceed — see Open Questions).
- [ ] **Task 5 — Tests (AC: 1–5)**
  - [ ] `lock_test.go`: acquire succeeds on fresh tree; second acquire fails; release frees it; stale-lock policy behaves as documented.
  - [ ] `start` use-case test (table-driven, `t.TempDir()` + a temp git repo): goal persisted + immutable on re-`start`; branch/dirty captured; second concurrent start refused; no-init behavior per AC 4; no network.
  - [ ] `git` minimal-read test against a temp repo (branch + dirty detection); non-repo handled.

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

_(to be filled by dev agent)_

### Debug Log References

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.

### File List

_(to be filled by dev agent)_
