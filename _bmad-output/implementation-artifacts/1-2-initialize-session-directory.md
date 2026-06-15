# Story 1.2: Initialize a Session Directory (`aictl init`)

Status: ready-for-dev

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

- [ ] **Task 1 — Centralized session paths (AC: 1)**
  - [ ] Create `internal/session/paths.go`: a `Paths` type/helpers resolving `.ai-session/` and its children (`config.yaml`, `state.yaml`, `checkpoints/`, `.gitignore`, `.lock`, etc.) from a repo root. **All `.ai-session/` paths must come from here** — no string-concatenated paths elsewhere (later stories depend on this).
  - [ ] Resolve the repo root (the working directory for v1; the directory where `.ai-session/` lives).
- [ ] **Task 2 — Atomic write store (AC: 4, NFR-3/4)**
  - [ ] Create `internal/session/store.go` with `WriteAtomic(path string, data []byte) error`: write to a temp file in the **same directory**, `fsync` the file, `rename` over the target (atomic on POSIX); also fsync the parent dir for durability.
  - [ ] This is the **single FS-write path** for all Session artifacts — every later story must use it (do not call `os.WriteFile` directly for Session files).
  - [ ] Add load helpers (`ReadFile`/unmarshal) as needed for the non-destructive check.
- [ ] **Task 3 — Task State & Config types (AC: 1)**
  - [ ] Create `internal/session/session.go`: `TaskState` struct (fields: `goal`, `completed`, `decisions`, `nextSteps`, `knownFailures`, `verifyCommands`) with explicit `yaml:"camelCase"` tags. `init` writes an **empty** TaskState (no goal yet — Goal is set in Story 1.3).
  - [ ] Create `internal/config/config.go`: `Config` struct with explicit `yaml:"camelCase"` tags — `defaultProvider`, `providers` (map), `verify` (commands), `handoff` options (incl. `maxDiffChars`), and the secret `denylist`.
  - [ ] Create `internal/config/defaults.go`: built-in defaults — `maxDiffChars` (e.g. 30000 per brief), secret denylist defaults (`.env*`, `*.pem`, `*.key`, `id_*`), and the Trio provider entries are NOT required here (full provider defaults land in Story 3.3) — include only what a freshly-initialized config needs to be valid and self-documenting.
- [ ] **Task 4 — `aictl init` command (AC: 1, 2, 3, 5, 6)**
  - [ ] Create `cmd/aictl/init.go`: thin Cobra subcommand calling an `app` method (e.g. `app.Init(ctx)`); no logic in `cmd/`.
  - [ ] Implement the init use-case in `internal/app` (or `internal/session`, called via `app`): create `.ai-session/` + `checkpoints/`, write default `config.yaml` and empty `state.yaml` via `WriteAtomic`, write `.ai-session/.gitignore`.
  - [ ] Non-destructive guard: if `.ai-session/` (or its key files) already exists, do not overwrite — refuse with a clear message and non-zero exit (AC 2).
  - [ ] Print concise success message with the created path (AC 6).
- [ ] **Task 5 — Tests (AC: 1–5)**
  - [ ] `store_test.go`: `WriteAtomic` writes correct content, leaves no temp files on success, and overwrites atomically; (if feasible) does not corrupt the target on a simulated mid-write failure.
  - [ ] `init` use-case test (table-driven): fresh repo → dir + files created with valid YAML; existing `.ai-session/` → no-op/refusal, existing content untouched; no network (pure FS).
  - [ ] Confirm generated `config.yaml`/`state.yaml` round-trip through goccy/go-yaml unmarshal.

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

_(to be filled by dev agent)_

### Debug Log References

### Completion Notes List

- Ultimate context engine analysis completed — comprehensive developer guide created.

### File List

_(to be filled by dev agent)_
