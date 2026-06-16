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

Foundation only: the `aictl` binary builds and runs (`aictl --help`, `aictl --version`).
Functional commands (`init`, `start`, `run`, `handoff`, …) land in subsequent stories.

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
