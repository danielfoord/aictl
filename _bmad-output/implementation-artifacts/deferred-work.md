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
