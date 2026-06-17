# aictl

> Portable AI coding sessions across providers.

`aictl` is a provider-agnostic **supervisor** for agentic coding CLIs (Claude Code,
Codex, Gemini, and others). It wraps the real provider CLI in a pseudo-terminal so
you keep its native UI, while `aictl` owns durable, portable session state in your
repo — so a coding task survives provider switches and quota limits.

**The core idea:** the orchestrator is the source of truth, not the AI CLI. `aictl`
generates a portable handoff *before* every risky provider call, so even if a
provider dies the moment it starts (out of credits), the next one picks up from a
clean, fully-informed state — no work lost.

> 🚧 **Under construction.** This is the project foundation. Commands are being
> implemented incrementally (see `_bmad-output/`).

## Status

Working today: session state (`init`, `start`, `note`/`done`/`next`/`fail`),
portable handoffs (`handoff`/`prepare`, `recover`), verification (`verify`),
supervised provider runs in a PTY with conditional handoff injection and
before/after checkpoints (`run`), and on-demand checkpoints (`checkpoint`).

In progress: automatic provider **fallback** on quota limits (`run --fallback`)
and the attempts log — see `_bmad-output/`. Until then, switch providers by
re-running `aictl run <next-provider>`.

## Requirements

- Go 1.26+
- `git` on `PATH`
- The provider CLIs you intend to supervise (e.g. `claude`, `codex`, `gemini`)

Targets macOS and Linux.

## Build

```bash
go build ./cmd/aictl
./aictl --help
```

Or install it onto your `PATH`:

```bash
go install github.com/danielfoord/aictl/cmd/aictl@latest
```

## Usage

### How a session is meant to work

`aictl` treats the AI CLI as a **disposable worker** and treats itself as the
**durable memory** of the task. You don't run `claude` (or `codex`, or `gemini`)
directly — you run it *under* `aictl`, which keeps the goal, the plan, the git
state, and a transcript in your repo so the task survives the provider being
switched, rate-limited, or killed mid-thought. The guiding rule: **the
orchestrator is the source of truth, not the AI.**

A task flows like this:

1. **Frame the task once.** `aictl start "<goal>"` records an immutable goal and
   snapshots your starting git state. As you (or the provider) make decisions and
   discover steps, capture them with `note` / `next` / `done` / `fail`. This is
   the task's memory, and it lives in `.ai-session/state.yaml` — *not* in any
   provider's chat history, which dies with the provider. (If your goal lives in a
   planning tool — a story file, a ticket — configure a `goalSource` and run
   `aictl sync` to pull it in instead of retyping; see Configuration.)

2. **Tell aictl how to check your work.** Put your build/test commands in
   `config.yaml` and run `aictl verify`; the captured output rides along in the
   next handoff so the provider always knows the real build/test state.

3. **Work through a provider.** `aictl run claude` launches Claude Code in its
   normal terminal UI. *Before* it starts, aictl generates a **handoff** — a
   plain-Markdown briefing built from your goal, current `git diff`/status, recent
   commits, latest verify output, and your notes — and checkpoints the repo before
   and after, so even if the provider dies the instant it opens (out of credits),
   nothing is lost. When you're **resuming** a task (a goal or recorded progress
   exists), aictl also tells the provider to read the handoff and continue; on a
   **fresh** session it stays out of the way and lets you drive — type your own
   command (e.g. a slash command like `/dev-story`) and capture the goal afterward.

4. **Switch providers without re-explaining.** When a provider hits its usage
   limit, run `aictl run codex`. aictl regenerates the handoff from your *current*
   git state, so the next provider picks up exactly where the last left off — from
   ground truth, not a stale summary. This is the core promise: **never lose a
   coding session to quota.** (Doing this automatically on a quota hit —
   `run --fallback` — is the next milestone; today you switch by hand.)

5. **Everything but the run is offline.** Capturing state, generating a handoff
   (`aictl handoff`), checkpointing (`aictl checkpoint "<label>"`), and producing a
   last-resort prompt (`aictl recover`) all work with **zero providers and no
   network** — so your task state never depends on a live CLI or your credit
   balance.

Because the handoff is regenerated from git before every risky call, it can't
drift or lie about what's been done — and because all state is plaintext in your
repo, the whole session is inspectable and travels with the code.

### Quickstart

```bash
# 1. Initialize aictl in your repo (creates .ai-session/)
aictl init

# 2. Record the task's immutable goal
aictl start "Refactor the exit-reason classifier"

# 3. Capture context as you work — offline, no provider, no network
aictl next "extract classify() into its own function"
aictl note "keep snake_case string values for exit reasons"

# 4. Run a provider CLI through aictl. Its native TUI is untouched; aictl
#    prepares a handoff, checkpoints the repo before & after, records a raw
#    transcript, and preserves the provider's exit code.
aictl run claude

# 5. Snapshot a labeled recovery point any time
aictl checkpoint "before risky refactor"
```

Everything except `run` is fully offline and never invokes a provider, so your
session state never depends on a live CLI or network.

### Commands

| Command | What it does |
|---|---|
| `aictl init` | Create the `.ai-session/` Session Directory (non-destructive if it exists). |
| `aictl start <goal>` | Begin a session with an immutable goal; capture initial repo state. |
| `aictl note <decision>` | Append a decision to the task state. |
| `aictl done <step>` | Append a completed step. |
| `aictl next <step>` | Append a next step. |
| `aictl fail <failure>` | Append a known failure. |
| `aictl handoff` (alias `prepare`) | Generate the deterministic, portable `handoff.md` from task + git state. |
| `aictl verify` | Run your configured verification commands and capture their output for the next handoff. |
| `aictl recover` | Produce a minimal last-resort continuation prompt from git + goal alone. |
| `aictl run <provider> [-- args...]` | Launch a provider CLI in a pseudo-terminal with the handoff injected and pre/post checkpoints. |
| `aictl checkpoint <label>` | Write a labeled, provider-free snapshot of repo + task context. |
| `aictl sync` | Populate the session goal (and next-steps/decisions) from the configured `goalSource`, when the session has no goal yet. |

`aictl run` resolves `<provider>` through the registry: the built-in **Trio**
(`claude`, `codex`, `gemini`) and any providers you define in config. Arguments
after `--` are passed straight through to the provider. A name that isn't a known
provider is run as a plain executable (no handoff injection), so you can supervise
any CLI. The provider's exit code becomes `aictl`'s exit code.

### Configuration

Config lives at `.ai-session/config.yaml` (created by `init`) and is a plain,
hand-editable YAML contract:

```yaml
# Providers overlay the built-in claude/codex/gemini adapters. A new name adds a
# provider; an existing name overrides only the fields you set.
providers:
  claude:
    command: claude            # executable on PATH (default: the provider name)
    args: []                   # extra args prepended before injection / your run args
    promptInjection:
      mode: file-ref           # file-ref (default) | arg | stdin | paste
      text: ""                 # custom prompt; empty = "read .ai-session/handoff.md and continue"
    usageLimitPatterns: ["usage limit", "try again"]

# Commands run by `aictl verify`; their combined output feeds the next handoff.
verify:
  - go test ./...
  - go vet ./...

handoff:
  maxDiffChars: 30000          # truncate the git diff embedded in a handoff

# File globs whose contents are stripped from diffs/handoffs/checkpoints.
denylist: [".env*", "*.pem", "*.key", "id_*"]

# Optional, tool-agnostic source `aictl sync` reads to set the goal when the
# session has none. Use a file OR a command (file wins if both are set).
goalSource:
  file: TASK.md                          # read the goal from a file, OR
  command: "your-tool print-current-goal"  # run a command; use its stdout
```

Injection modes: `file-ref` (default) injects a short "read `.ai-session/handoff.md`
and continue" instruction as an argument; `arg` passes your `text` as an argument;
`stdin`/`paste` type the prompt into the provider's terminal after it starts (for
CLIs that can't take a prompt argument).

**`goalSource` and `aictl sync`** (task continuity): aictl is tool-agnostic — it
ships no knowledge of any planning tool. `aictl sync` reads your configured `file`
or `command` and uses the output as the goal; it does **not** overwrite a goal you
already set. The output may be **structured YAML** —

```yaml
goal: Refactor the exit-reason classifier
nextSteps:
  - extract classify() into its own function
decisions:
  - keep snake_case exit-reason values
```

— or **plain text** (the whole output becomes the goal). Point `command` at a tiny
script for *any* workflow (a BMad story, a Jira/Linear CLI, a `cat TASK.md`); the
integration lives in your config, never in aictl. Once a goal is set, `aictl run`
injects the handoff so the next provider continues the task (see step 3 above).

### Session state (`.ai-session/`)

All state is plaintext and travels with your repo:

| Path | Contents |
|---|---|
| `state.yaml` | Task state: goal, decisions, completed/next steps, known failures. |
| `config.yaml` | The config above. |
| `handoff.md` | The most recently generated handoff packet. |
| `checkpoints/NNNN-{before,after}-<provider>/` | Pre/post-run snapshots (git status/diff/commits, summary, and the run's `transcript.ansi`). |
| `checkpoints/NNNN-checkpoint-<label>/` | On-demand `aictl checkpoint` snapshots. |

Transient/secret-bearing artifacts (checkpoints, transcripts, the lockfile) are
gitignored; `state.yaml`, `config.yaml`, and `handoff.md` stay tracked so the
session is portable.

## Debugging

### Tests first

Most issues are fastest to reproduce and debug through the test suite (logic lives
in `internal/`, with the `cmd/` layer kept thin):

```bash
go test ./...                       # whole suite
go test ./internal/app -run TestRun # one package / one test (regex)
go test ./internal/... -v           # verbose: see which cases run
go test -race ./internal/app ./internal/shell   # the PTY/UI paths — run with the race detector
go vet ./...                        # static checks
```

The fake provider CLI used by the PTY tests is a self-exec test helper
(`TestHelperProcess` in `internal/shell`), so you don't need a real provider
installed to debug `run` behavior.

### Stepping with Delve

```bash
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a command end-to-end (offline commands are easiest):
dlv debug ./cmd/aictl -- start "refactor the classifier"

# Debug a specific test:
dlv test ./internal/checkpoint -- -test.run TestCaptureManualWritesExpectedArtifacts
```

VS Code (`.vscode/launch.json`):

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "aictl start",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/aictl",
      "args": ["start", "debug session"]
    }
  ]
}
```

### Debugging `aictl run` (the PTY path)

`aictl run <provider>` puts your terminal into **raw mode** and launches the
provider inside a pseudo-terminal, which conflicts with a debugger that also
drives the terminal. Prefer debugging the runner through `internal/shell` tests,
or attach Delve in headless mode and connect from another terminal:

```bash
dlv debug --headless --listen=:2345 --api-version=2 ./cmd/aictl -- run claude
# then, in a second terminal:  dlv connect :2345
```

If a crashed or interrupted debug session leaves your terminal garbled (stuck in
raw mode), restore it with:

```bash
stty sane    # or: reset
```

### Inspecting session state

`aictl` keeps all state as plaintext under `.ai-session/`, so the on-disk
artifacts are themselves a debugging tool — read them directly:

```bash
cat .ai-session/state.yaml          # task state (goal, next steps, decisions, failures)
cat .ai-session/config.yaml         # resolved config (providers, denylist, maxDiffChars)
cat .ai-session/handoff.md          # last generated handoff
ls  .ai-session/checkpoints/        # per-run before/after + on-demand checkpoints
cat .ai-session/checkpoints/0001-after-*/transcript.ansi   # raw provider transcript
```

These files are gitignored where they may hold raw output; the transcript is
written `0600` because it can contain secrets the provider printed.

## License

[MIT](LICENSE)
