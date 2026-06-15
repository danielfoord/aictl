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

## License

[MIT](LICENSE)
