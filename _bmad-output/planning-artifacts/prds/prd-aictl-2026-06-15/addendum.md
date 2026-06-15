# aictl — PRD Addendum

Implementation-level depth from `BRIEF.md` that does **not** belong in the capability-level PRD but is valuable input to a downstream architecture / solution-design pass. Nothing here is a requirement; it is the maintainer's design exploration, preserved verbatim in intent.

## Recommended language & key library
- **Go**, single static binary. Maintainer preference; fits CLI tooling.
- PTY via **`github.com/creack/pty`** (Unix). The one essential native concern.

## Proposed package layout (from brief)
```
ai-session-manager/        # repo
  cmd/aictl/main.go
  internal/
    session/   session.go store.go
    git/       status.go diff.go commits.go
    providers/ provider.go codex.go claude.go gemini.go
    handoff/   generator.go templates.go
    shell/     runner.go
  .ai-session/             # runtime, per-repo
```

## Session Directory contents (target)
```
.ai-session/
  state.yaml            # Task State (durable, aictl-owned)
  config.yaml           # Config (public contract)
  handoff.md            # current Handoff
  attempts.log          # one line per Attempt
  command-log.md
  latest-verify.txt
  transcript.md / .ansi # per-attempt transcripts
  checkpoints/
    0001-before-codex/  handoff.md git-status.txt git-diff.patch recent-commits.txt command-log.md summary.md
    0001-after-codex/   stdout.log stderr.log exit-code.txt git-status.txt git-diff.patch
```

## Three state layers (mental model)
1. **Durable project state** — git diff/status, changed files, test/build output, branch. Survives provider failure (primary source of truth).
2. **Durable task state** — goal, plan, completed/pending steps, decisions, constraints, known failures. aictl-owned; cannot live inside a provider.
3. **Provider transcript** — stdout/stderr/terminal output. Nice-to-have, never the only state.

## Provider abstraction (interface sketch)
The right abstraction is `InteractiveProcessProvider`, not `ModelProvider`. Sketch:
```go
type Provider interface {
    Name() string
    Command() string
    Args(promptMode PromptMode, handoffPath string) []string
    UsageLimitPatterns() []string
}
type PromptMode string // "arg" | "stdin" | "pty" | "file_ref"
```
Default for Claude-Code-style tools: **`file_ref`** — inject only "Read `.ai-session/handoff.md` and continue the task."

### Exit reason enum
```go
ExitSuccess | ExitUsageLimit | ExitAuthFailure | ExitCrash | ExitUserCancel | ExitUnknown
```

## PTY runner shape (from brief)
- `pty.Start(cmd)`; `pty.InheritSize(os.Stdin, ptmx)`; enable raw mode + restore on exit.
- `go io.Copy(ptmx, os.Stdin)` for keyboard passthrough.
- Output path is a **tee + scan**, not a plain copy:
  ```
  PTY output ──┬── user terminal
               ├── transcript log
               └── usage-limit detector
  ```
  Implemented via a custom `MultiWriter`/`ObservedWriter` (Observe → log → terminal) or `io.TeeReader`.
- Handle **SIGWINCH** → `pty.InheritSize` on resize, else rich TUIs break.

## Prompt-injection strategies
1. **Arg** — `claude "$(cat .ai-session/handoff.md)"` (for CLIs that take a prompt arg).
2. **Interactive paste** — start in PTY, wait, write handoff text + Enter; consider **bracketed paste** (`ESC[200~ … ESC[201~`) for multi-line.
3. **File-ref (preferred default)** — small prompt: "Continue the in-progress task. Read `.ai-session/handoff.md` first. Do not restart from scratch." Avoids mangling the TUI with large pastes; keeps handoff inspectable.

## Execution loop (fallback) — pseudocode
```
while providers remain:
    generate handoff from current state
    create pre-run checkpoint
    run provider with handoff (in PTY)
    capture stdout/stderr + post-run git diff
    if success:        run verification; update session; stop
    if usage_limit:    if repo changed → regenerate handoff; try next provider
    if unexpected:     stop and show diagnostic
```
Go sketch `RunWithFallback(...)` and `RunInteractiveProvider(...)` provided in brief (lines ~927–1439).

## Deterministic handoff template (no LLM)
Sections: Goal · Current Repository State (branch, git status, diff) · Recent Commands · Latest Verification Output · Known Plan · Instructions (continue from current state; do not restart; inspect modified files; prefer small safe changes; run verification before finishing).

## Example Config (from brief)
```yaml
project: extractly
defaultProvider: codex
providers:
  codex:  { command: codex,  args: [], promptInjection: { mode: file-ref, text: "Read .ai-session/handoff.md and continue the task." }, usageLimitPatterns: ["usage limit","quota exceeded","try again in","billing"] }
  claude: { command: claude, args: [], promptInjection: { mode: file-ref, text: "Read .ai-session/handoff.md and continue the task." }, usageLimitPatterns: ["usage limit","try again"] }
  gemini: { command: gemini, args: [], promptInjection: { mode: file-ref, text: "Read .ai-session/handoff.md and continue the task." }, usageLimitPatterns: ["quota","rate limit"] }
verify: [ "dotnet test", "npm run build" ]
handoff: { includeDiff: true, includeRecentCommits: true, includeCommandLog: true, includeTestOutput: true, maxDiffChars: 30000 }
```

## Tricky bits flagged in the brief
- **Interactive CLIs need a TTY** — plain stdin/stdout pipes break rendering, keybindings, raw mode, resize, approval flows.
- **Tool-specific flags** — each provider accepts prompts / resumes sessions differently → per-adapter knowledge.
- **Context size** — can't include whole repo/huge diffs; need smart truncation (`maxDiffChars`).
- **Bad handoffs** — vague summaries cause redo; handoff must be strict and file-specific.
- **Secrets** — never dump `.env`/keys/credentials into handoff.
- **Uncommitted work** — preserve via patches/checkpoints before switching.
- **Evidence rule** — handoff must not claim "Provider X did Y" without git/file/verify evidence.

## Brief's recommended build order (milestones)
1. Session folder + config → 2. Git status/diff capture → 3. Handoff markdown generation → 4. Basic provider shell-out → 5. Manual provider switch → 6. Command/test log capture → 7. Auto-detect quota errors → 8. Auto-fallback chain → 9. PTY for interactive CLIs → 10. Context compression/summarization.
*(Note: PTY is listed at step 9 in the linear order but is foundational for the interactive-UI goal; brief's later "MVP implementation path" reprioritizes PTY runner to Milestone 1. Architecture should resolve sequencing.)*

## Naming alternatives considered (rejected)
`agentmux` ("tmux for AI coding agents"), `relay`, `codemux`, `agent-pty`, `handoff`. **Chosen: `aictl`.**

## Taglines (marketing, for later)
- "Portable AI coding sessions across providers."
- "Never lose an agentic coding workflow to quota limits again."
