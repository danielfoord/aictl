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

## Deferred from: code review of 2-3-verify-commands-feed-handoff (2026-06-16)

- **Bound the verify channels (capture + storage + embed)** — `verify.Run` uses `CombinedOutput` (buffers everything in memory); `latest-verify.txt` and `command-log.md` are written/grown unbounded and embedded whole into the handoff. Add a capture-side limited writer, truncate `latest-verify.txt`, bound the handoff embed, and rotate/cap the append-only `command-log.md`. The verify analogue of the diff's `maxDiffChars`. → **handoff/verify hardening.**
- **Configurable verify timeout** — verify commands run under the inherited signal-aware ctx (Ctrl-C interrupts after the 2.3 ctx-handling patch), but there is no per-run deadline for unattended/CI use; a hung interactive command blocks until canceled. Add a generous configurable timeout. → **config + verify follow-up.**
- **Content secret-scanner over verify output / command log** — path-based denylist can't catch a token printed in test output or a command log line. Reinforces the 2.2-deferred content scanner; gating item before Epic-3 auto-injection. (`latest-verify.txt`/`command-log.md` are gitignored, limiting commit exposure, but the handoff embed is unredacted.) → **post-v1 / Epic-3 gate.**

## Deferred from: code review of 2-4-last-resort-recovery (2026-06-16)

- **Resolve true repo root before session lookup** — `App.Recover` follows the current command pattern of using raw CWD. Running from a repo subdirectory misses the repo-root `.ai-session/`; this is the same pre-existing repo-root resolution gap already deferred from Story 1.3. → **shared session/repo-root helper follow-up.**
- **Decide whether untracked files belong in recovery state** — `git.Diff` captures staged + unstaged tracked diff but not untracked file contents. A recovery prompt can show `(no uncommitted changes)` when only untracked files exist. This traces to the existing git-diff granularity/open-question rather than this story alone. → **git capture semantics follow-up.**

## Deferred from: code review of 3-1-run-provider-in-pty (2026-06-16)

- **stdin→ptmx copy goroutine leak / keystroke steal** — `io.Copy(ptmx, stdin)` (internal/shell/runner.go:80-82) stays blocked on `os.Stdin.Read` after the child exits and `ptmx` is closed; the goroutine leaks and the pending read can swallow the user's next keystroke. Inherent to PTY wrappers; clean interruption of a blocking stdin read is non-trivial. The spec's requirement (unblock the child/output path) is satisfied. → **PTY stdin-copy lifecycle follow-up.**
- **No `cmd.WaitDelay` on the provider process** — internal/shell/runner.go:84-86. If a grandchild keeps the PTY slave open after the child exits, `<-outputDone` could hang with terminal restore still deferred. Low likelihood; add `WaitDelay` hardening in a later story. → **process shutdown hardening follow-up.**
- **AC1 quiet/no-interleave + ANSI passthrough not tested** — internal/shell/runner_test.go. No test asserts aictl stays silent while the provider owns the screen or that ANSI/control output is mirrored verbatim. Structural guarantee holds in code (only pre/post UI lines). → **PTY passthrough test follow-up.**

## Deferred from: code review of 3-2-faithful-transcript-and-quiet-supervision (2026-06-16)

- **Concurrent `aictl run` corrupts the shared transcript** — `openTranscript` (internal/app/run.go) uses `os.Create` (truncate) on a single `.ai-session/transcript.ansi` with no run lock. Two concurrent runs in the same worktree truncate/interleave one file. Resolved by Story 3.4's per-Attempt transcript directories (`checkpoints/NNNN-after-<provider>/transcript.ansi`) + run-time locking. → **per-Attempt transcript + run lock follow-up.**
- **No negative-path test for transcript close on error/signal** — `closeTranscript` is deferred and correct by construction, but no test exercises the provider-error or signal close path. The 3.2 UI-flush panic test partially covers panic unwind. → **transcript close-path test follow-up.**
- **Tap short-write / Close error swallowed** — the fan-out tap (transcript) ignores short writes and the transcript `Close` error is discarded; an `ENOSPC` partial write could make the transcript non-byte-identical without surfacing. Best-effort by design (capture must not degrade the screen). → **transcript fidelity-on-disk-error follow-up.**

## Deferred from: code review of 3-3-provider-adapters-trio-config-injection (2026-06-16)

- **paste/stdin injection robustness** — `shell.Options.InitialInput` is written 250ms after launch (`internal/shell/runner.go`) with no TUI-readiness handshake, a hardcoded `"\r"` submit byte, and possible interleaving with early keystrokes (both the injection and the stdin copy write the same ptmx). Inherent to the paste approach and a known PRD open question; `file-ref` (no PTY write) is the robust default for the Trio. → **paste-injection reliability follow-up.**
- **Provider-specific positional-arg placement** — `internal/providers/injection.go` appends the prompt as the final positional arg; a provider that needs the prompt behind a flag (e.g. `-p`) or a `--` separator relies on config `args` for now. → **per-provider arg-shape follow-up.**
