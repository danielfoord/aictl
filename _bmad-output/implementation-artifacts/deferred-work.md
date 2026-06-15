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
- **Resolve the true repo root** (walk up to `.git` / an existing `.ai-session/`) instead of using raw CWD, so commands run from a subdirectory reuse the project's session; warn when outside a git repo. Consolidate with the 1.2-deferred repo-root check. → **Story 2.1 / shared `session` helper.**
