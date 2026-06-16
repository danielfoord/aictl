---
baseline_commit: a68190be887ef607711571df3f6bed65362715ab
---

# Story 2.2: Generate a deterministic Handoff (`aictl handoff` / `prepare`)

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want to generate a portable `handoff.md` from my task and git state,
so that I can hand my in-progress task to any tool (by copy-paste or file reference) without re-explaining it — and it works even with no provider available.

## Acceptance Criteria

1. **`aictl handoff` (and alias `aictl prepare`)** generate `.ai-session/handoff.md` from a fixed template, sourcing: Goal, branch, `git status`, the working+staged diff (redacted + bounded via Story 2.1), recent commits, latest verify output (if present), command log (if present), and the plan (next steps). They exit 0 **without invoking any provider** and with no network. (FR-11, FR-12, NFR-1)
2. **Deterministic:** given identical inputs, `handoff.Generate` returns byte-identical output. No timestamps, randomness, map-iteration order, or clock reads in the generator. (FR-11; covered by a golden/determinism test)
3. **Anti-redo instructions present:** the Handoff contains explicit guidance — continue from current state; do not restart from scratch; inspect changed files first; run verification before finishing. (FR-11)
4. **Bounded & redacted diff:** the embedded diff is produced via `git.Diff(ctx, dir, denylist, maxChars)` with the denylist and `maxDiffChars` read from the session Config (defaults `30000` / `.env*,*.pem,*.key,id_*`). Secrets never reach `handoff.md`. (FR-13, privacy)
5. **Atomic write:** `handoff.md` is written via `session.WriteAtomic`. (NFR-3)
6. **Requires a session; graceful with sparse state:** if no `.ai-session/` exists, fail with a clear message + non-zero exit. Optional sources absent (no verify output / no command log / unborn repo / empty task state) produce a valid Handoff with those sections empty or omitted — never an error.

## Tasks / Subtasks

- [x] **Task 1 — Pure generator `internal/handoff` (AC: 1, 2, 3)**
  - [x] `internal/handoff/generator.go`: `Generate(in Input) (string, error)` — pure (no I/O/clock/randomness). `Input` has Goal/Branch/Status/Diff/RecentCommits/VerifyOutput/CommandLog/NextSteps/Decisions/KnownFailures.
  - [x] `internal/handoff/templates.go` + `handoff.tmpl`: `//go:embed` + `text/template`; anti-redo instructions + labeled sections; empty optionals omitted via `{{if}}`/`{{-`.
  - [x] No `time.Now()`/randomness/map-ranging — ranges only slices in fixed order. Determinism verified by golden + determinism tests.
- [x] **Task 2 — Input assembly in `internal/app` (AC: 1, 4, 5, 6)**
  - [x] `App.Handoff(ctx)` in `internal/app/handoff.go`: cwd → require `.ai-session/` (`ErrNoSession`) → `config.Load` + `session.LoadState` → best-effort git `Status`/`Diff(denylist,maxDiffChars)`/`RecentCommits` → read `latest-verify.txt`/`command-log.md` if present → `handoff.Generate` → `WriteAtomic(paths.Handoff())`.
  - [x] Added `config.Load(path)` (missing file → `Default()`); passes `cfg.Denylist` + `cfg.Handoff.MaxDiffChars` to `git.Diff`.
  - [x] Prints the handoff path.
- [x] **Task 3 — Commands `cmd/aictl/handoff.go` (AC: 1)**
  - [x] `aictl handoff` (`cobra.NoArgs`, alias `prepare` via `Aliases`) → `app.Handoff`; registered on root.
- [x] **Task 4 — Tests (AC: 1–6)**
  - [x] `generator_test.go`: determinism, committed golden (`testdata/handoff_golden.md`), anti-redo phrases, sparse-input section omission.
  - [x] `handoff_test.go`: after `start`, `Handoff` writes `handoff.md` with goal + diff; denylisted `.env` secret absent; no session → `ErrNoSession`.

### Review Findings (code review 2026-06-16)

_3 adversarial layers. Acceptance Auditor: all 6 ACs PASS (literal AC4 — diff redaction — holds). Both hunters surfaced a design gap the ACs didn't cover: the handoff embeds secret-bearing channels beyond the diff, and a config foot-gun can disable redaction._

**Patch (applied 2026-06-16):**

- [x] [Review][Patch] `config.Load` normalizes guardrails: empty `denylist` → `DefaultDenylist`, `maxDiffChars <= 0` → `DefaultMaxDiffChars` (a present-but-empty config can no longer silently disable redaction/bounding). Tests in `config_test.go` [internal/config/config.go]
- [x] [Review][Patch] `git.RedactStatusPaths` redacts denylisted file paths (incl. rename `old -> new`) from `git status`, wired into `App.Handoff`. Test `TestRedactStatusPaths` [internal/git/diff.go, internal/app/handoff.go]
- [x] [Review][Patch] `App.Handoff` surfaces real git failures as warnings (only `git.ErrNotARepo` is treated as a benign empty section) — no more misleading "clean tree" handoff [internal/app/handoff.go]
- [x] [Review][Patch] `init` gitignore now includes `latest-verify.txt` and `command-log.md` [internal/app/init.go]

**Deferred (tracked in deferred-work.md):**

- [x] [Review][Defer] **Content secret scanner** (regex/entropy) applied to the channels path-matching can't cover — **commit messages** (`RecentCommits`), **verify output**, **command log** — so a secret in those is stripped even though it isn't a denylisted file. Reinforces the PRD's deferred "deeper content-scanning redaction" → post-v1, and is now the gating item before the handoff is auto-fed to providers (Epic 3)
- [x] [Review][Defer] Exclude `.ai-session/` from the handoff's git capture so the handoff doesn't report aictl's own dir / self-reference `handoff.md` on a second run (touches the 2.1 git layer + the 1.2 "handoff.md tracked" decision) → focused follow-up
- [x] [Review][Defer] Markdown-fence hardening: content containing a ``` run closes the fenced block early (mis-parse / minor injection into the consuming model) — use a dynamic fence longer than any backtick run → handoff-hardening follow-up
- [x] [Review][Defer] Bound the size of embedded `verify`/`command-log` like the diff is bounded → Story 2.3 / Epic 3 when those channels are populated

**Dismissed:** malformed `config.yaml` aborts handoff (failing **closed** is correct — better than generating with a broken/empty denylist); `handoff.md` mode `0o644` (it's meant to be shared/handed off; redaction is the protection); `RecentCommits` count hardcoded to 10 (spec doesn't require config); generator/template determinism concerns (verified: `git.run` forces `LC_ALL=C`, template ranges only slices, no clock/rand/maps — both determinism + golden tests pass).

## Dev Notes

**Second story of Epic 2. Depends on 2.1 (git capture) + Epic 1 (session/state/config).** This is the **heart of the tool** — the deterministic, LLM-free handoff. Compose existing primitives; the generator itself must stay pure.

### Reuse (don't reinvent)

- **Story 2.1 git layer:** `git.Status(ctx, dir)`, `git.Diff(ctx, dir, denylist, maxChars)` (already redacts secrets + truncates), `git.RecentCommits(ctx, dir, n)`. `Diff`/`Status`/`RecentCommits` return `git.ErrNotARepo` outside a repo — treat that (and unborn repos) as empty sections, mirroring `start`'s best-effort capture.
- **Config:** `config.Unmarshal([]byte)` exists; defaults `config.DefaultMaxDiffChars` (30000) and `config.DefaultDenylist`. There is **no `LoadConfig(path)` helper yet** — add a tiny one (read file → `config.Unmarshal`; missing file → `config.Default()`), in `internal/config` or inline in app. Keep it small.
- **Session:** `session.LoadState` / `session.NewPaths` / `session.WriteAtomic`. `paths.Handoff()` — check it exists; if not, add it to `session/paths.go` (`.ai-session/handoff.md`) the same way `Config()`/`State()` are defined. `ErrNoSession` from Story 1.4 (in `internal/app`).
- **App + thin-cmd pattern:** mirror `internal/app/{init,start,state}.go` ↔ `cmd/aictl/*.go` registered in `root.go`.

### Determinism (the load-bearing requirement — FR-11)

- The generator is a **pure function of its `Input`**. Forbidden inside it: `time.Now()`, `rand`, environment reads, ranging a `map`, goroutine-order dependence. This is what makes the handoff byte-stable and golden-testable, and is exactly the architecture's named anti-pattern (`time.Now() inside handoff.Generate`).
- Any inherently variable data (e.g. a real timestamp) belongs in the *caller's* logs/attempts, **not** in the deterministic handoff body. The handoff describes repo+task state, which is already captured deterministically.
- Use `text/template` with `//go:embed`; the embedded template is the single source of the wording.

### Handoff content (template sections)

Source-of-truth sections (label clearly): **Goal**; **Current Repository State** (branch, `git status`, diff); **Recent Commits**; **Latest Verification Output** (if present); **Recent Commands** (from `command-log.md` if present — this file is only populated from Story 2.3/Epic 3; include-if-present); **Plan / Next Steps** (+ Decisions, Known Failures from Task State); **Instructions** (the AC-3 anti-redo block). [Source: prd.md FR-11; addendum.md "Deterministic handoff template"; architecture.md#Handoff generation]

### Conventions (inherited — CI-enforced)

- Pure generator + thin `cmd/`; `context.Context` first on `App.Handoff`; `%w` error wrapping; atomic writes via `WriteAtomic`; camelCase yaml (state/config already tagged); **no network** (depguard + `internal/guard`). gofmt import-block alignment (the recurring CI nit). errcheck is on — handle every return. [Source: architecture.md#Implementation Patterns; Epic-1/2.1 review history]

### Learnings carried from 2.1 / Epic 1 (real)

- 2.1's review hammered the **redaction guardrail**; you get that for free by routing the diff through `git.Diff` — **do not** re-capture the diff yourself or bypass redaction. The handoff test should still assert a denylisted secret never appears (defense check).
- Tests use `t.Chdir(t.TempDir())` + a temp `git init` repo (helper pattern in `internal/git/capture_test.go`, `internal/app/start_test.go`); `newTestApp()` exists in `internal/app/*_test.go` (UI → `io.Discard`).
- Library-only vs wired: this story DOES wire a command (handoff/prepare). The `run --fallback` pre-run handoff regeneration (FR-12 "run regenerates before each provider") is **Epic 3** — out of scope here; just provide the command + generator.
- gofmt the embedded `.tmpl`? No — gofmt only touches `.go`. But keep the template file tidy; CI gofmt won't check it.

### Project Structure Notes

- NEW: `internal/handoff/{generator.go,templates.go,handoff.tmpl,generator_test.go}`, `internal/app/{handoff.go,handoff_test.go}`, `cmd/aictl/handoff.go`, `internal/handoff/testdata/<golden>`. MODIFIED: `cmd/aictl/root.go` (register handoff/prepare), possibly `internal/session/paths.go` (add `Handoff()` if absent), and a small config-load helper. No new external dependencies (`text/template`, `embed` are stdlib).

### References

- [Source: epics.md#Epic 2 → Story 2.2]
- [Source: prd.md FR-11 (deterministic handoff), FR-12 (handoff/prepare), FR-13 (bounded diff), NFR-1/NFR-3]
- [Source: architecture.md#Handoff generation (deterministic); #Format Patterns ("Handoff is generated by a pure function … no I/O, clock, randomness")]
- [Source: addendum.md — deterministic handoff template sections]
- [Source: implementation-artifacts/2-1-capture-faithful-git-state.md — git.Status/Diff/RecentCommits, redaction]

### Open Questions

1. **Command-log / verify sections now or later?** Both files are first populated in Story 2.3 (verify) / Epic 3 (command log). Recommend rendering them **include-if-present** now (empty/omitted until those stories), so 2.2 needs no rework. Confirm.
2. **`prepare` as alias vs distinct command** — recommend a Cobra alias on `handoff` (same behavior) unless you want different semantics later.
3. **Template wording** — the spec fixes the required *sections* and anti-redo *intent*; exact prose is the dev's to draft in `handoff.tmpl` (keep it provider-agnostic and imperative).

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (BMad dev-story workflow)

### Debug Log References

- `go mod tidy` clean; `go build ./...` ok; `go vet ./...` no issues; `gofmt` clean
- Golden seeded via `go test ./internal/handoff -run TestGenerateGolden -update`
- `go test ./...` → **62 passed** across 8 packages (new `internal/handoff` pkg)
- Smoke (temp git repo): `start` → stage `main.go` + `.env` → `note`/`next` → `aictl prepare` → handoff.md rendered with goal, status, diff (`.env` **`[redacted]`**, `main.go` intact), Decisions, Next Steps, Instructions; `grep topsecret` → no leak; output has no timestamps (deterministic).

### Completion Notes List

- **Deterministic by construction:** `handoff.Generate` is a pure function over `Input`; the embedded `handoff.tmpl` ranges only slices and reads only fields — no clock/rand/maps. Determinism is covered by both a byte-equality test and a committed golden file.
- **Redaction reused, not reimplemented:** the diff is sourced solely via `git.Diff(denylist, maxDiffChars)` (Story 2.1), so secret-stripping + truncation come for free; the app test asserts a `.env` secret never reaches `handoff.md` (defense check), and the live smoke confirmed it.
- **Added supporting helpers:** `config.Load(path)` (missing → `Default()`); `session.Paths.Handoff()`/`LatestVerify()`/`CommandLog()`. `ErrNoSession` reused from Story 1.4 (same package).
- **Include-if-present** for `latest-verify.txt` (Story 2.3) and `command-log.md` (later) — both render empty/omitted today, so no rework when those stories land (resolves open-question Q1).
- `prepare` is a Cobra **alias** on `handoff` (Q2). Template wording (Q3) drafted in `handoff.tmpl` — provider-agnostic, imperative.
- **Scope fence held:** the `run`-time pre-provider handoff regeneration (FR-12 "before each provider") remains Epic 3; this story ships only the command + generator.
- Conventions: pure generator, thin `cmd/`, context-first, `%w` wrapping, atomic write, no network (guard test green), gofmt clean.
- Known cosmetic (not blocking): `git status` lists untracked `.ai-session/` in the handoff — tied to the deferred "status-path redaction / .ai-session root-gitignore" item (Story 2.3).

### File List

- `internal/handoff/generator.go` (new)
- `internal/handoff/templates.go` (new)
- `internal/handoff/handoff.tmpl` (new)
- `internal/handoff/generator_test.go` (new)
- `internal/handoff/testdata/handoff_golden.md` (new — golden)
- `internal/app/handoff.go` (new)
- `internal/app/handoff_test.go` (new)
- `cmd/aictl/handoff.go` (new)
- `internal/session/paths.go` (modified — Handoff/LatestVerify/CommandLog accessors)
- `internal/config/config.go` (modified — `Load(path)` helper)
- `cmd/aictl/root.go` (modified — register handoff/prepare)

### Change Log

- 2026-06-16: Implemented Story 2.2 — deterministic handoff generation (FR-11, FR-12). Added the pure `internal/handoff` generator (embedded `text/template`, byte-stable, golden-tested), `App.Handoff` assembling task + redacted git state, and `aictl handoff`/`prepare`. Added `config.Load` + `paths.Handoff/LatestVerify/CommandLog`. 62 tests passing.
- 2026-06-16: Addressed code review — 4 patches hardening the handoff's secret surface: config guardrail normalization (empty denylist / maxDiffChars≤0 → defaults), `git status` path redaction, surfacing real git-capture failures, and gitignoring verify/command-log artifacts. 4 findings deferred (content secret-scanner for commit-msg/verify/command-log — gating Epic-3 auto-injection; exclude `.ai-session/` from capture; markdown-fence hardening; bound verify/log size). 66 tests passing.
