---
baseline_commit: 90af2e41a846087d29c7c17aef4b3b5859c67f08
---

# Story 2.1: Capture faithful git state

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want aictl to capture my branch, status, diff, and recent commits from the real `git` binary,
so that handoffs reflect exactly what I see — with secrets excluded and large diffs bounded.

## Acceptance Criteria

1. **Faithful capture via shell-out:** aictl captures, verbatim from the `git` binary: short status (`git status --short`), the working+staged diff, and recent commits (`git log`, bounded count). Output is what the developer sees — aictl does not reformat it. (FR-13; architecture "git is source of truth")
2. **Diff covers working + staged changes** and works on a repo with no commits yet (unborn HEAD) without erroring.
3. **Bounded diff:** the captured diff is truncated at a caller-supplied `maxDiffChars` with an explicit, unmistakable marker (e.g. `… [truncated N chars]`). Truncation happens **after** redaction. (FR-13, NFR-6)
4. **Secret redaction:** file sections in the diff whose path matches any caller-supplied denylist glob (default `.env*`, `*.pem`, `*.key`, `id_*`) have their contents stripped/replaced with a redaction marker **before** the diff reaches any consumer. (privacy guardrail)
5. **Offline & classified errors:** all capture works with no network. "Not a git repository" is reported as a distinct sentinel (`ErrNotARepo`) so callers can treat it as benign; other git failures surface as wrapped errors.
6. **Bounded execution:** git subprocesses run under a timeout so a hung/prompting git cannot block aictl indefinitely.
7. **No regression:** existing `Branch`/`IsDirty`/`Capture` (used by `aictl start`) keep working unchanged.

## Tasks / Subtasks

- [x] **Task 1 — Expand `internal/git` capture (AC: 1, 2, 7)**
  - [x] Added `Status` → `git status --short --untracked-files=normal`.
  - [x] Added `RecentCommits(ctx, dir, n)` → `git log -n <n> --oneline` (default n=10); unborn repo (no commits) returns `"", nil`.
  - [x] Added `Diff(ctx, dir, denylist, maxChars)` → working+staged diff, redacted then truncated.
  - [x] Preserved `Branch`/`IsDirty`/`Capture`/`run`/`Snapshot`; updated the package doc comment.
  - [x] Unborn-HEAD safe: `Diff` uses `git diff --cached` + `git diff` (not `git diff HEAD`); documented in code.
- [x] **Task 2 — Secret-denylist filter (AC: 4)** — `internal/git/diff.go`
  - [x] `splitDiffSections` parses on `diff --git ` boundaries; `redactDiff` replaces matching file bodies with `[redacted: matches secret denylist]`, preserving the header.
  - [x] Matches the **basename** of the b-path against each glob via `filepath.Match` (documented; recommendation from open-question Q3 adopted).
  - [x] Redaction runs before truncation.
- [x] **Task 3 — Bounded diff (AC: 3)**
  - [x] `truncate` caps at `maxChars` with `… [truncated N chars]`; `maxChars <= 0` = no limit.
- [x] **Task 4 — Timeout + error classification (AC: 5, 6)**
  - [x] `run` wraps each git call in `context.WithTimeout(ctx, 10s)` (honors parent cancellation).
  - [x] `ErrNotARepo` sentinel returned (via `%w`/`errors.Is`) when git reports "not a git repository". `Diff` gates on `rev-parse --is-inside-work-tree` first so it classifies cleanly (raw `git diff` outside a repo emits a `--no-index` usage error, not the standard message — caught by a test).
- [x] **Task 5 — Tests (AC: 1–7)**
  - [x] `capture_test.go` (temp `git init` repos + `mustGit` helper): status with untracked; diff staged on unborn repo; redaction of a tracked `.env`; truncation at maxChars; recent-commits empty on unborn + present after a commit; all four capture fns classify non-repo as `ErrNotARepo`.
  - [x] `diff_test.go` (pure unit): redaction strips denylisted file / preserves others / no-denylist unchanged; `matchesDenylist`; `truncate` boundaries.
  - [x] Existing `Branch`/`IsDirty`/`Capture` tests unchanged & passing (no regression).

### Review Findings (code review 2026-06-16)

_3 adversarial layers. Acceptance Auditor: all 7 ACs PASS. Both hunters converged on a secret-leak cluster in the redaction parser — security-relevant (redacted diffs feed AI providers), so the parser is being hardened._

**Patch (applied 2026-06-16):**

- [x] [Review][Patch] `run` prepends `-c core.quotePath=false` and sets `LC_ALL=C`/`LANG=C` — deterministic path output + English messages for classification [internal/git/git.go]
- [x] [Review][Patch] `Diff` forces `--src-prefix=a/ --dst-prefix=b/` — immune to `diff.noprefix`/`mnemonicPrefix` [internal/git/git.go]
- [x] [Review][Patch] Rewrote header parsing (`sectionPaths`/`scanPath`, quoted+unquoted) and `matchesDenylist` (both a/b paths, full-path **or** basename) — closes quoted-path, rename-a-side, and directory-glob leaks. New tests: `TestRedactDiffQuotedPath`, `TestRedactDiffDirectoryGlob`, `TestRedactDiffRenameMatchesASide` [internal/git/diff.go]
- [x] [Review][Patch] `truncate` backs off to a UTF-8 rune boundary (`TestTruncateRuneBoundary`) [internal/git/diff.go]
- [x] [Review][Patch] `RecentCommits` detects unborn branch via `git rev-parse --verify --quiet HEAD` (locale-independent) [internal/git/git.go]

**Deferred (tracked in deferred-work.md):**

- [x] [Review][Defer] Defense-in-depth content secret scanner (regex/entropy over diff bodies) so a redaction parse-miss still can't leak — PRD already defers "deeper content-scanning redaction" → post-v1
- [x] [Review][Defer] Apply the denylist to `Status` output too (untracked secret *filenames* are disclosed by name in status) → Story 2.2 (when status feeds the handoff)
- [x] [Review][Defer] Validate denylist globs at config-load and warn on `ErrBadPattern` (today `matchesDenylist` fails open on a malformed glob) → config-loading story (3.3)
- [x] [Review][Defer] Single whole-operation timeout budget for `Diff` (today each of the ~3 subprocesses gets its own 10s) → minor git-layer follow-up

**Dismissed:** `run` TrimSpace "fidelity" (git diffs carry no significant edge whitespace; trimming the trailing newline is harmless); injected `diff --git ` line in a file body splitting a section (real diff content lines are prefixed with space/`+`/`-`, so an unprefixed `diff --git ` only occurs as a real header); bare-repo `--is-inside-work-tree=false` (aictl operates in work trees); large `n` to `RecentCommits` (git handles it, output bounded by the timeout).

## Dev Notes

**First story of Epic 2 (Portable Handoffs & Recovery). Depends on Epic 1 (all done).** This story matures the `internal/git` package created minimally in Story 1.3 — it is an **UPDATE**, not a greenfield package.

### Current state of `internal/git/git.go` (READ before editing)

Exists today (Story 1.3): `Snapshot{Branch, Dirty}`, the `run(ctx, dir, args...)` shell-out helper (captures stdout, wraps stderr), `Branch` (`git branch --show-current`), `IsDirty` (`git status --porcelain --untracked-files=normal`), `Capture` (Branch+Dirty). **`aictl start` (internal/app/start.go) calls `git.Capture` best-effort** — do not change these signatures or semantics; only add to them. The package doc comment explicitly says diff/commits/denylist arrive "in Story 2.1" — update it.

### Layering (important)

- `internal/git` must **not** import `internal/config`. The denylist (`[]string`) and `maxChars` (`int`) are **parameters** to `Diff`, supplied by the caller from `config.Config` (`Handoff.MaxDiffChars`, `Denylist`). This keeps git config-agnostic and matches the architecture's single-filter-point boundary (the filter lives in `internal/git`, the values come from config). [Source: architecture.md#Architectural Boundaries: "only internal/git shells out … the secret denylist filter lives here"]
- Defaults already exist: `config.DefaultMaxDiffChars = 30000`, `config.DefaultDenylist = {".env*","*.pem","*.key","id_*"}`. The handoff/app layer (Story 2.2) will read these from the loaded Config and pass them in. This story just needs the git functions to accept and honor them; tests pass values directly.

### Diff capture approach (resolve the unborn-HEAD gotcha)

Story 1.3 deliberately used `git branch --show-current` over `rev-parse HEAD` to avoid unborn-HEAD failures — same class of issue here. `git diff HEAD` errors on a repo with no commits. Recommended: capture `git diff` + `git diff --cached` and concatenate (label staged vs unstaged if useful), both safe pre-first-commit. Whatever you choose, AC 2 (unborn repo) must pass.

### Redaction details

- A unified diff is a sequence of per-file blocks starting with `diff --git a/<p> b/<p>`. Parse on these boundaries (don't try to be clever with hunk internals — redact the whole file block body, preserving the `diff --git` header so the redaction is visible).
- Match denylist globs with `path/filepath.Match`. Note `filepath.Match` does not cross `/` with `*`, so `.env*` matches `.env`, `.env.local` at a given dir level; decide whether to match the basename, the full path, or both, and **document it** (recommend: match the basename against each glob, since denylist entries like `*.pem` are basename patterns). Renames/copies (`rename from`/`rename to`) — match either path.
- This is the only secret-stripping point; everything downstream (handoff, checkpoints) consumes the already-redacted diff.

### Conventions (inherited — enforced by CI)

- Shell out to real `git` via `os/exec` (no git library). `context.Context` first arg; wrap errors `%w`. **No network** (depguard + `internal/guard` test). gofmt import-block alignment (CI gate scoped to `git ls-files '*.go'`); **errcheck** is on (golangci-lint v2.12.2) — handle every error return; capture `cmd.Run()` errors (already done in `run`). [Source: architecture.md#Implementation Patterns; Stories 1.1–1.4 review history]
- Tests: stdlib `testing`, table-driven, `t.TempDir()`; for commits set `git -C dir config user.email/user.name` locally (no global config needed for init/status/diff). Pin git output expectations loosely (don't assert exact hashes).

### Learnings carried from Epic 1 (real)

- The recurring review theme has been **partial-failure / robustness and faithful semantics** (atomic writes, lock rollback, deterministic dirty). Here the analogues are: redaction-before-truncation ordering (AC 3/4), unborn-HEAD safety (AC 2), and not-a-repo classification (AC 5) — all already baked into the ACs; don't cut them.
- Story 1.3's `git.run` swallows nothing — it wraps stderr into the error. Keep that; build `ErrNotARepo` detection on top (inspect stderr for git's "not a git repository" message, or check exit code 128 + message).
- Keep new exported surface minimal and documented; the package doc should now describe the full capture set.

### Project Structure Notes

- MODIFIED: `internal/git/git.go` (add Status/RecentCommits/Diff + timeout + ErrNotARepo; preserve existing). NEW: `internal/git/diff.go` (redaction+truncation), `internal/git/diff_test.go`, additions to `internal/git/git_test.go`. No changes outside `internal/git`. No new dependencies. No `cmd/` or `app/` wiring in this story — these are library functions consumed by Story 2.2 (handoff generation).

### References

- [Source: epics.md#Epic 2 → Story 2.1]
- [Source: prd.md FR-13 (bounded diff); #Constraints/Privacy (secret denylist); NFR-1, NFR-6]
- [Source: architecture.md#API & Communication (git shell-out); #Authentication & Security (denylist single filter point); #Architectural Boundaries]
- [Source: implementation-artifacts/1-3-start-session-with-goal.md — existing git layer + unborn-HEAD learning]
- [Source: deferred-work.md — "git subprocess timeouts + error classification" routed to Story 2.1]

### Open Questions

1. **Repo-root resolution** (walk up to `.git`/existing `.ai-session`) was also routed here from earlier reviews, but it's a cross-cutting concern about *where* commands operate, not git capture per se. Recommend keeping it **out** of 2.1 (capture takes an explicit `dir`) and addressing it as a small shared `session`/`app` helper in a dedicated follow-up. Confirm.
2. **Recent-commits format/count:** `--oneline` with N=10 assumed. Confirm if handoffs want a richer format (author/date) or different N.
3. **Denylist match scope:** basename vs full-path matching (recommend basename). Confirm.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (BMad dev-story workflow)

### Debug Log References

- `go mod tidy` clean; `go build ./...` ok; `go vet ./...` no issues; `gofmt` clean
- `go test ./...` → **52 passed** across 7 packages
- A test (`TestCaptureFunctionsClassifyNotARepo`) caught that `git diff --cached` *outside* a repo emits a `--no-index` usage error rather than "not a git repository"; fixed by gating `Diff` on `git rev-parse --is-inside-work-tree`, which classifies cleanly as `ErrNotARepo`.

### Completion Notes List

- **Library-only story** — no `cmd/`/`app/` wiring (Story 2.2 will consume these). `internal/git` matured: added `Status`, `RecentCommits`, `Diff`; preserved `Branch`/`IsDirty`/`Capture`/`Snapshot` (verified `aictl start` unaffected — its tests still pass).
- **Layering held:** `internal/git` does not import `internal/config`; `Diff` takes `denylist []string` + `maxChars int` as params (caller supplies from `config`). Filter lives in `internal/git` per the architecture's single-filter-point boundary.
- **Redaction before truncation** (so a secret can't survive via the truncation boundary); redaction matches the **basename** against denylist globs and preserves the `diff --git` header so a redacted file is visibly present.
- **Unborn-HEAD safe** via `git diff --cached` + `git diff` (not `git diff HEAD`); `RecentCommits` returns empty (not an error) when there are no commits yet.
- **Resolved deferred items:** git subprocess timeout (10s) + `ErrNotARepo` classification (both routed here from earlier reviews).
- **Open-question dispositions:** Q1 repo-root resolution kept OUT of 2.1 (capture takes explicit `dir`) — remains a follow-up; Q2 recent-commits `--oneline`, default n=10; Q3 denylist matches basename. Known limitation (documented in `diff.go`): file paths containing spaces (git-quoted) aren't parsed for redaction — keep secret files space-free / gitignored.
- Conventions: context-first, `%w` wrapping, no network (guard test green), gofmt clean.

### File List

- `internal/git/git.go` (modified — Status/RecentCommits/Diff, timeout, ErrNotARepo, doc comment; preserved existing)
- `internal/git/diff.go` (new — redactDiff/splitDiffSections/sectionPath/matchesDenylist/truncate)
- `internal/git/diff_test.go` (new — pure-function tests)
- `internal/git/capture_test.go` (new — git-integration tests)

### Change Log

- 2026-06-16: Implemented Story 2.1 — faithful git capture (FR-13). Expanded `internal/git` with `Status`, `RecentCommits`, and a redacted+bounded `Diff` (working+staged, secret-denylist stripping, `maxChars` truncation), plus a 10s subprocess timeout and `ErrNotARepo` classification. Library-only; preserved Epic-1 git functions. 52 tests passing.
- 2026-06-16: Addressed code review — 5 patches hardening the secret-redaction guardrail: deterministic git env (`core.quotePath=false`, `LC_ALL=C`), forced `--src-prefix/--dst-prefix`, robust quoted/unquoted path parsing matching both a/b paths on full-path-or-basename (closes quoted-path / rename / directory-glob leaks), rune-safe truncation, and locale-independent unborn-branch detection. 4 findings deferred (content secret-scanner, status-path redaction, denylist glob validation, shared Diff timeout). 56 tests passing.
