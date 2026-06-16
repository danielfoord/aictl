# Deferred Work

## Deferred from: code review of 1-1-project-foundation-and-skeleton (2026-06-15)

- **Escalate a second Ctrl-C to a hard exit** so the supervisor stays killable when a child ignores context cancellation. No long-running `RunE` exists yet, so not reachable now. → **Story 3.1 (PTY runner).**
- **Synchronize `internal/ui` writes** (mutex) and handle nil writers / `Fprintf` errors. `ui` is the shared output sink for a tool that will stream concurrent child output; unsynchronized writes will interleave/race once concurrency lands. → **Epic 3 fan-out writer (Story 3.2).**
- **Add `cobra.NoArgs` / unknown-command handling to the root command.** Bare/unknown args are currently accepted without a clear error. Best addressed once subcommands exist. → **Story 1.2+.**
- **Pin exact `creack/pty` and `goccy/go-yaml` versions** when they are first imported (currently pinned only in story/architecture prose, not enforced by `go.mod`). → **Stories 1.2 (go-yaml) / 3.1 (pty).** (go-yaml v1.19.2 pinned in Story 1.2.)

## Deferred from: code review of 1-2-initialize-session-directory (2026-06-15)

- **Strict YAML unmarshal** (reject unknown fields) so typos in hand-edited `config.yaml` / `state.yaml` are reported rather than silently dropped (NFR-4 hand-editability). Add at the point each file is loaded-and-acted-on. → **state: Story 1.3 · config: Story 3.3.**
- **Normalize nil `Providers` map** after config unmarshal (avoid a future nil-map assignment panic) and **warn when `init`/commands run outside a git repo** (aictl is git-centric). → **Stories 1.3 / 2.1 / 3.3.**
- **Parent-directory fsync** for the `.ai-session/` directory creation itself (file writes are already fsync'd; the dir-entry creation is not). Low impact — `init` is re-runnable. → **store-hardening pass when warranted.**

## Deferred from: code review of 1-3-start-session-with-goal (2026-06-15)

- **Stale-lock recovery** for abnormal termination (SIGKILL / power loss) that strands `.ai-session/.lock`. The defer-rollback applied in 1.3 releases the lock on in-process failures, but a crash between `AcquireLock` and `SaveState` still leaves a permanent lock with no recovery path. Add pid-liveness detection in `AcquireLock` (reclaim if the recorded pid is dead) and/or an `aictl unlock` / `start --force`. → **follow-up story (lock recovery).**
- **git subprocess timeouts + error classification** — bound git calls with a timeout and distinguish "not a git repository" (expected, silent) from a genuine git failure (surface a warning). Currently all git errors are swallowed as best-effort. → **Story 2.1 (git layer).**
- **Resolve the true repo root** (walk up to `.git` / an existing `.ai-session/`) instead of using raw CWD, so commands run from a subdirectory reuse the project's session; warn when outside a git repo. Consolidate with the 1.2-deferred repo-root check. → **Story 2.1 / shared `session` helper.** (2.1 kept this out of scope; still open.)

## Deferred from: code review of 2-1-capture-faithful-git-state (2026-06-16)

- **Defense-in-depth content secret scanner** — a regex/entropy scan over diff bodies so that even if the path-based redaction parser misses a section, a recognizable secret (API keys, PEM blocks) is still stripped. The PRD already defers "deeper content-scanning redaction (entropy/regex over diff bodies)" to post-v1; this review reinforces it. → **post-v1 redaction hardening.**
- **Apply denylist to `Status` output** — `git status --short` lists untracked file *paths*; a not-yet-gitignored secret filename is disclosed by name when status is embedded in a handoff. Filter/redact denylisted paths from status too. → **Story 2.2 (handoff generation), when status first feeds a handoff.**
- **Validate denylist globs at config load** — `matchesDenylist` does `filepath.Match` and ignores `ErrBadPattern`, so a malformed glob fails open (never matches). Validate patterns when the config is loaded and warn/error. → **Story 3.3 (config loading) or a config-validation pass.**
- **Single whole-operation timeout for `Diff`** — currently each git subprocess gets its own 10s via `run`; `Diff` issues ~3, so the effective budget is ~30s. Give the whole capture one shared deadline. → **minor git-layer follow-up.**

## Deferred from: code review of 2-2-generate-deterministic-handoff (2026-06-16)

- **Content secret scanner for non-diff channels** — the handoff embeds `git status` (paths), `RecentCommits` (commit messages), verify output, and command log. Path-based denylist redaction (now applied to diff + status) cannot catch a secret *value* pasted into a commit message or printed in verify/command output. A regex/entropy content scanner is needed to redact those. This is the PRD's already-deferred "deeper content-scanning redaction" and is now the **gating item before the handoff is automatically fed to providers in Epic 3** (manual use is lower-risk). → **post-v1 redaction hardening / before Epic 3 auto-injection.**
- **Exclude `.ai-session/` from handoff git capture** — `git status`/`diff` report aictl's own session dir (and, on a second run, the tracked `handoff.md` self-references). Exclude `.ai-session/` via a pathspec in the handoff capture. Note this intersects the Story 1.2 decision to keep `handoff.md` tracked "so the session travels with the repo" — revisit whether `handoff.md` should be gitignored instead. → **focused follow-up (touches git layer + gitignore policy).**
- **Markdown-fence hardening** — diff/verify/command-log content containing a ``` run closes the handoff's fenced code block early, corrupting the packet (and a mild prompt-injection vector into the consuming model). Use a dynamically-sized fence (longer than any backtick run in the body). → **handoff-hardening follow-up.**
- **Bound embedded verify/command-log size** — only the diff is capped by `maxDiffChars`; `latest-verify.txt`/`command-log.md` are embedded unbounded. Cap them too. → **Story 2.3 / Epic 3 when those files are populated.**
