# Deferred Work

## Deferred from: code review of 1-1-project-foundation-and-skeleton (2026-06-15)

- **Escalate a second Ctrl-C to a hard exit** so the supervisor stays killable when a child ignores context cancellation. No long-running `RunE` exists yet, so not reachable now. → **Story 3.1 (PTY runner).**
- **Synchronize `internal/ui` writes** (mutex) and handle nil writers / `Fprintf` errors. `ui` is the shared output sink for a tool that will stream concurrent child output; unsynchronized writes will interleave/race once concurrency lands. → **Epic 3 fan-out writer (Story 3.2).**
- **Add `cobra.NoArgs` / unknown-command handling to the root command.** Bare/unknown args are currently accepted without a clear error. Best addressed once subcommands exist. → **Story 1.2+.**
- **Pin exact `creack/pty` and `goccy/go-yaml` versions** when they are first imported (currently pinned only in story/architecture prose, not enforced by `go.mod`). → **Stories 1.2 (go-yaml) / 3.1 (pty).**
