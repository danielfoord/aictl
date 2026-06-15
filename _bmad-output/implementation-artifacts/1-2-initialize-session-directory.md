---
baseline_commit: 69232211365d075d012f2ce3653daa09f2116f79
---

# Story 1.2: Initialize a Session Directory (`aictl init`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to run `aictl init` in my repo,
so that aictl has a durable, repo-resident place to keep my task state.

## Acceptance Criteria

1. **Creates the Session Directory:** running `aictl init` in a repo with no `.ai-session/` creates `.ai-session/` containing `config.yaml` (with built-in defaults), `state.yaml` (empty Task State), and a `checkpoints/` directory. (FR-1)
2. **Non-destructive:** running `aictl init` when `.ai-session/` already exists does not destroy or overwrite existing state — it is a no-op or an explicit, clearly-messaged refusal. (FR-1)
3. **Protects secrets / avoids committing transients:** `init` adds ignore entries so transient/sensitive parts of the Session Directory are not accidentally committed (e.g. writes/updates `.ai-session/.gitignore`). (FR-1, privacy guardrail)
4. **Atomic & plaintext writes:** every file written goes through the atomic write helper (temp file in the same dir → fsync → rename), and all artifacts are human-readable plaintext YAML. (NFR-3, NFR-4)
5. **Offline:** `init` performs no network access and invokes no provider. (NFR-1)
6. **Clear feedback:** on success `init` prints a concise confirmation including the created path; on refusal (AC 2) it prints why and exits non-zero.

## Tasks / Subtasks

- [x] **Task 1 — Centralized session paths (AC: 1)**
  - [x] Created `internal/session/paths.go`: `Paths` type with `Dir/Config/State/Checkpoints/GitIgnore/Lock` accessors. Single source of truth for `.ai-session/` paths.
  - [x] Repo root = working directory (resolved via `os.Getwd()` in the use-case).
- [x] **Task 2 — Atomic write store (AC: 4, NFR-3/4)**
  - [x] Created `internal/session/store.go` `WriteAtomic(path, data, perm)`: temp file in same dir → fsync → rename → fsync parent dir. Cleans up temp file on any failure.
  - [x] Single FS-write path for Session artifacts (documented in code).
  - [x] Added unmarshal load helpers (`UnmarshalTaskState`, `config.Unmarshal`) used by the non-destructive check / tests.
- [x] **Task 3 — Task State & Config types (AC: 1)**
  - [x] Created `internal/session/session.go`: `TaskState` (goal/completed/decisions/nextSteps/knownFailures/verifyCommands) with explicit camelCase yaml tags + `Marshal`. `init` writes an empty TaskState.
  - [x] Created `internal/config/config.go`: `Config` (defaultProvider/providers/verify/handoff/denylist) with camelCase yaml tags + `Marshal`/`Unmarshal`.
  - [x] Created `internal/config/defaults.go`: `Default()` with `maxDiffChars: 30000` and denylist (`.env*`, `*.pem`, `*.key`, `id_*`); providers/verify empty (Trio defaults deferred to Story 3.3).
- [x] **Task 4 — `aictl init` command (AC: 1, 2, 3, 5, 6)**
  - [x] Created `cmd/aictl/init.go`: thin Cobra subcommand (`cobra.NoArgs`) calling `app.Init(ctx)`.
  - [x] Implemented `App.Init` in `internal/app/init.go`: creates `.ai-session/` + `checkpoints/`, writes default config, empty state, and `.gitignore` via `WriteAtomic`.
  - [x] Non-destructive guard: refuses with `ErrAlreadyInitialized` + exit 1 if `.ai-session/` exists.
  - [x] Prints success message with created path.
- [x] **Task 5 — Tests (AC: 1–5)**
  - [x] `internal/session/store_test.go`: content correctness, no temp leftovers, atomic overwrite, perm applied.
  - [x] `internal/app/init_test.go`: fresh dir creates all files with valid YAML; existing dir → `ErrAlreadyInitialized` with existing content untouched; gitignore entries; pure-FS (no network). Uses `t.Chdir` + `t.TempDir`.
  - [x] config.yaml / state.yaml round-trip via goccy/go-yaml verified in the tests above.

### Review Findings (code review 2026-06-15)

_3 adversarial layers. Acceptance Auditor: all 6 ACs PASS, all conventions PASS. Hunters found a robustness cluster in `init`'s create path._

**Patch (applied 2026-06-15):**

- [x] [Review][Patch] Made `init` atomic & self-healing — `os.Mkdir` gate (race-free create-or-`EEXIST`; also covers a file at `.ai-session`) + `defer` rollback (`os.RemoveAll` on partial failure, only ever removing a dir this call created) [internal/app/init.go]
- [x] [Review][Patch] Added `.tmp-*` to the Session `.gitignore` [internal/app/init.go] (+ assertion in init_test.go)

**Deferred (tracked in deferred-work.md):**

- [x] [Review][Defer] Strict YAML unmarshal (reject unknown fields) to catch typos in hand-edited config/state — deferred to the load-and-act stories (state load → 1.3, config load → 3.3)
- [x] [Review][Defer] Normalize nil `Providers` map on load + warn when `init` runs outside a git repo — deferred to git/config-loading stories (1.3 / 2.1 / 3.3)
- [x] [Review][Defer] Parent-dir fsync for `.ai-session/` directory creation durability — low; revisit during a store-hardening pass

**Dismissed (noise / false positive):** empty slices "serialize as null" (false — goccy emits `[]`, confirmed in smoke test); `os.Rename` Windows semantics (Unix-only per PRD); `state.yaml` `0o644` (task state is non-secret plaintext); fsyncDir-after-rename "conflation" (failing closed on non-durable is intentional); `ctx` unused in `Init` (intentional for a short FS op, convention established); relative `repoRoot` (caller passes absolute `os.Getwd()`).

## Dev Notes

**Depends on Story 1.1** (module, Cobra root, `internal/app`, `internal/ui`, CI/lint). 1.1 may not be merged yet — assume its scaffolding and conventions exist; do not re-create them.

This story introduces three foundational pieces used by nearly every later story: **centralized paths**, the **atomic write store**, and the **config/state types + defaults**. Get these right — they are high-leverage.

### What this story creates (NEW files)

```
cmd/aictl/init.go                 # thin `aictl init` subcommand
internal/session/paths.go         # centralized .ai-session/ path resolution  ← used by ALL later stories
internal/session/store.go         # WriteAtomic (temp+fsync+rename)            ← single FS-write path
internal/session/session.go       # TaskState type (empty state written here)
internal/config/config.go         # Config type (yaml camelCase tags)
internal/config/defaults.go       # built-in defaults (maxDiffChars, secret denylist)
+ tests co-located (*_test.go)
```
Wire the init use-case through `internal/app` (thin `cmd/`). [Source: architecture.md#Project Structure & Boundaries, #Structure Patterns]

### Session Directory layout (target — this story creates the base)

```
<repo>/.ai-session/
├── config.yaml           # created now (defaults)
├── state.yaml            # created now (empty TaskState)
├── .gitignore            # created now (ignore transient/sensitive parts)
└── checkpoints/          # created now (empty dir; populated in Epic 3)
```
Deferred (created by later stories, do NOT create now): `handoff.md` (2.2), `attempts.log` (4.3), `command-log.md`/`latest-verify.txt` (2.3), `.lock` (1.3). [Source: architecture.md#Project Structure & Boundaries → runtime artifact]

### Critical conventions (architecture — enforce here, inherited by all later stories)

- **Atomic durability (NFR-3):** all Session-artifact writes go through `session.store.WriteAtomic` — temp + fsync + rename. No direct `os.WriteFile` for Session files. This story is where the helper is born; later stories (handoff, checkpoints, attempts) reuse it. [Source: architecture.md#Data Architecture, #Format Patterns ("All on-disk writes go through one atomic helper")]
- **Inspectable plaintext (NFR-4):** YAML only, human-readable, hand-editable. [Source: architecture.md#Cross-Cutting NFRs NFR-4]
- **YAML keys = `camelCase`** with explicit `yaml:"..."` tags on every serialized field — never rely on default casing (config.yaml is a public contract). [Source: architecture.md#Format Patterns → YAML keys]
- **Centralized paths:** one `session.Paths` source of truth; no scattered path strings. [Source: architecture.md#Architectural Boundaries ("`.ai-session/` paths are centralized")]
- **Thin `cmd/`, no global state, `context.Context` first, wrap errors with `%w`, `cmd` prints errors / sets exit code.** [Source: architecture.md#Implementation Patterns]
- **Offline (NFR-1):** no network imports (the depguard rule from 1.1 enforces this). [Source: architecture.md#Cross-Cutting NFRs NFR-1]

### Library notes — goccy/go-yaml

- Marshal/unmarshal via `yaml.Marshal(v)` / `yaml.Unmarshal(data, &v)`. Use struct tags `yaml:"defaultProvider"` etc. Pin the exact version resolved in Story 1.1's `go.mod`. Do not introduce `gopkg.in/yaml.v3`. [Source: architecture.md#Starter Template Evaluation]
- Prefer a stable field order / clear default file so the generated `config.yaml` reads as self-documenting (it is the user's primary contract).

### Config defaults to seed (from brief/PRD)

- `maxDiffChars`: 30000 (handoff diff bound; first used Story 2.1/2.2). [Source: prd.md FR-13; addendum.md example config]
- Secret denylist defaults: `.env*`, `*.pem`, `*.key`, `id_*` (used by the git diff filter in Story 2.1). [Source: prd.md#Constraints/Privacy; architecture.md#Authentication & Security]
- A `config.yaml` produced by `init` must be valid and round-trippable even though providers/verify are empty until later stories.

### Project Structure Notes

- Matches `architecture.md#Project Structure & Boundaries`. The only judgment call: where the init use-case lives — recommend a method on `internal/app` delegating to `internal/session`, keeping `cmd/aictl/init.go` thin.
- Carries forward the unresolved **module path** from Story 1.1 (import paths). No new structural conflicts.

### Testing standards

- Go stdlib `testing`, table-driven. Use `t.TempDir()` for filesystem tests (no real repo needed). Assert: files exist, YAML is valid and round-trips, non-destructive path leaves existing content byte-identical, and no network. [Source: architecture.md#Starter Template Evaluation → Testing]
- CI (from 1.1) must stay green.

### References

- [Source: epics.md#Epic 1 → Story 1.2]
- [Source: prd.md#4.1 Session lifecycle & task state — FR-1]
- [Source: prd.md#Constraints & Guardrails → Privacy/Secrets]
- [Source: architecture.md#Data Architecture — Session state & on-disk layout]
- [Source: architecture.md#Format Patterns; #Structure Patterns; #Architectural Boundaries]
- [Source: architecture.md#Cross-Cutting NFRs — NFR-3 (durability), NFR-4 (plaintext)]
- [Source: addendum.md — example config.yaml, Session Directory contents]

### Open Questions

1. **Module path / license** — still pending from Story 1.1; blocks `go mod init` import paths but not the design of this story.
2. **`init` on existing directory** — confirm preferred behavior: silent no-op vs explicit refusal with non-zero exit. (Story drafts AC 2 as "no-op OR explicit refusal"; recommend **explicit refusal** so users aren't surprised, but defer to maintainer.)

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (BMad dev-story workflow)

### Debug Log References

- `go get github.com/goccy/go-yaml@latest` → v1.19.2 (first import; now a direct dependency, resolving the deferral from Story 1.1)
- `go mod tidy` → clean; `go build ./...` → success; `go vet ./...` → no issues
- `go test ./...` → 16 passed across 6 packages
- `gofmt -l cmd internal` → clean
- Smoke test in a temp dir: `aictl init` → creates `.ai-session/{config.yaml,state.yaml,checkpoints/,.gitignore}`, exit 0; second `aictl init` → "already initialized" + exit 1; config has `maxDiffChars: 30000` + denylist; state is empty TaskState; offline.

### Completion Notes List

- **Open question resolved:** `init` on an existing directory → **explicit refusal** (`ErrAlreadyInitialized`, exit 1), per the story's recommendation. Verified non-destructive (existing `state.yaml` left byte-identical).
- **goccy/go-yaml v1.19.2** added as a direct dependency (the deferred-from-1.1 item for go-yaml is now done; `creack/pty` remains deferred to Story 3.1).
- **Wired the app seam into cmd** (predicted in Story 1.1's review): `newRootCmd` now takes `*app.App` and registers `init`; `main.go` constructs the App; `root_test.go` updated. This resolves 1.1's "internal/app not imported by cmd" note.
- **`.gitignore` choice:** ignores `.lock` (machine-local) and `checkpoints/` (may hold raw transcripts with secrets, and is regenerable). `config.yaml`/`state.yaml` stay tracked so the portable session travels with the repo.
- **Conventions honored:** all Session writes go through `session.WriteAtomic`; camelCase yaml tags with explicit tags; thin `cmd/`; `context.Context` first; errors wrapped with `%w`; no network imports (guard test green).
- Did **not** address Story 1.1 deferred items (root `NoArgs` is now naturally satisfied since `init` adds a subcommand and root uses `cobra.NoArgs` on subcommands; broader arg-handling still tracked).

### File List

- `internal/session/paths.go` (new)
- `internal/session/store.go` (new)
- `internal/session/store_test.go` (new)
- `internal/session/session.go` (new)
- `internal/config/config.go` (new)
- `internal/config/defaults.go` (new)
- `internal/app/init.go` (new)
- `internal/app/init_test.go` (new)
- `cmd/aictl/init.go` (new)
- `cmd/aictl/main.go` (modified — construct App)
- `cmd/aictl/root.go` (modified — accept *app.App, register init subcommand)
- `cmd/aictl/root_test.go` (modified — new newRootCmd signature)
- `go.mod` / `go.sum` (modified — add goccy/go-yaml v1.19.2)

### Change Log

- 2026-06-15: Implemented Story 1.2 — `aictl init` (FR-1). Added `session` (paths, atomic store, TaskState) and `config` (types + defaults) packages, the `App.Init` use-case, and the `init` command; wired the app seam into cmd. Non-destructive (explicit refusal on existing). 16 tests passing.
- 2026-06-15: Addressed code review — 2 patches applied (atomic/self-healing `init` via `os.Mkdir` gate + rollback; `.tmp-*` added to Session `.gitignore`). 3 findings deferred (strict YAML, nil-map/git-repo checks, parent-dir fsync). 16 tests still passing.
