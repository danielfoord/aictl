---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - '_bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md'
  - '_bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md'
  - 'BRIEF.md'
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2026-06-15'
project_name: 'aictl'
user_name: 'Daniel'
date: '2026-06-15'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:** 21 FRs across 6 cohesive clusters that map closely to
component boundaries:
- **Session & Task State (FR-1–4):** repo-resident `.ai-session/`, credit-free state
  mutation (`note/done/next/fail`), provider-optional state updates. Local YAML store.
- **Interactive PTY Runner (FR-5–7):** run real provider CLI in a pseudo-terminal with
  full passthrough, faithful transcript capture, and silent supervision. Highest-risk
  component (raw mode, SIGWINCH, output fan-out).
- **Git Checkpointing (FR-8–10):** pre/post-run + on-demand snapshots of git
  status/diff/commits; "files changed" computed from pre/post comparison.
- **Handoff Generation (FR-11–14):** deterministic, LLM-free template fill; size-bounded;
  `recover` last-resort prompt from minimal state.
- **Provider Adapters & Config (FR-15–18):** built-in Trio adapters + config-driven
  generic adapter; per-provider prompt-injection modes; verify commands.
- **Usage Detection & Fallback (FR-19–21):** stream pattern-matching, exit-reason
  classification, `run --fallback` chain, attempts log.

**Non-Functional Requirements (architectural forcing functions):**
- **NFR-3 Pre-run durability ordering** — state + handoff must be persisted to disk
  *before* any provider process starts. Dictates run-loop control flow + atomic writes.
- **NFR-1 Provider-independence** — no network/LLM in the core path; handoff is a pure
  deterministic function of local state.
- **NFR-2 / NFR-5 TUI non-interference + quiet observability** — single pass-through
  output write fanning to terminal/transcript/detector; no TTY logging mid-run.
- **NFR-4 Inspectable plaintext state** — YAML throughout; no binary store/DB.
- **NFR-6 Startup overhead (~1s)** — git diff dominant; bounded by `maxDiffChars`.

**Scale & Complexity:** Medium, concentrated rather than distributed.
- Primary domain: CLI / terminal systems tooling (single-user, single-process, no service)
- Complexity level: Medium (hard parts = PTY concurrency + durability ordering + extensibility)
- Estimated architectural components: ~6–8 (cmd, session, git, providers, handoff,
  runner, config, checkpoint)

### Technical Constraints & Dependencies

- **Language/binary:** Go, single static binary, lean dependency tree.
- **PTY:** `github.com/creack/pty` (Unix). macOS + Linux only; Windows/ConPTY out of scope.
- **Runtime externals (not build deps):** provider CLIs (`claude`, `codex`, `gemini`) and
  the `git` binary must be present on PATH.
- **Hard prohibition:** no network calls or LLM usage in the core path (NFR-1).
- **Distribution:** `go install` + prebuilt macOS/Linux binaries on GitHub releases.

### Cross-Cutting Concerns Identified

- **Child-process lifecycle & cancellation:** spawn/wait/exit-code propagation; Ctrl-C →
  `user_cancel` stops the chain (does not advance).
- **Terminal state management:** raw-mode enable/restore (must survive panics/signals),
  SIGWINCH resize propagation.
- **Atomic persistence:** durable, fsync'd, hand-editable YAML writes (NFR-3/NFR-4).
- **Git access strategy:** shell-out vs library — decision pending.
- **Secret redaction:** git-tracked-only diffs + configurable path denylist; filter
  placement in the diff→handoff path.
- **Config as versioned public contract:** parsing, validation, defaults merge, semver.
- **Output fan-out:** terminal + transcript + usage detector from one write path.
- **Provider extensibility:** adapter interface unifying built-in and config-defined CLIs.

## Starter Template Evaluation

### Primary Technology Domain

CLI / terminal systems tool in **Go**, distributed as a single static binary. No web/scaffold
starter applies; the Go convention is `go mod init` + a command framework + a small set of
vetted libraries. Stack constraints (Go, `creack/pty`, macOS+Linux, lean deps, no network in
core) are inherited from the PRD and were not re-litigated.

### Starter Options Considered

- **No scaffold (chosen approach):** `go mod init` + Cobra. Go has no batteries-included
  generator equivalent to Next.js/T3; idiomatic Go projects assemble a thin set of libraries.
- **`cobra-cli` generator:** can scaffold the command tree, but produces boilerplate we'd
  immediately restructure to the addendum's package layout. Use selectively, not as the base.
- **Command framework fork:** Cobra vs urfave/cli vs stdlib `flag` — decided in favor of
  **Cobra** for subcommand ergonomics, POSIX flags, and shell completions on a public tool.

### Selected Foundation

**No third-party project scaffold.** Foundation = Go module + Cobra command layer + core libs,
organized per the addendum's package layout.

**Initialization Command (first implementation story):**

```bash
go mod init github.com/<owner>/aictl       # module path TBD by owner
go get github.com/spf13/cobra@v1.10.2      # CLI framework (pflag pulled transitively)
go get github.com/creack/pty@v1.1.24       # Unix pseudo-terminal
go get github.com/goccy/go-yaml@latest     # maintained YAML (NOT archived gopkg.in/yaml.v3)
# Go toolchain: 1.26.x (1.26.4 current stable)
```

### Verified Versions (web-checked, June 2026)

| Component | Version | Role |
|---|---|---|
| Go toolchain | 1.26.4 | language / single-binary build |
| `github.com/spf13/cobra` | v1.10.2 | command tree, flags, completions |
| `github.com/creack/pty` | v1.1.24 | Unix PTY (the one essential native dep) |
| YAML library | `goccy/go-yaml` (alt: `go.yaml.in/yaml/v3`) | config + state (de)serialization |

> ⚠️ `gopkg.in/yaml.v3` is archived/unmaintained as of 2026 — do not use it.

### Architectural Decisions Provided by This Foundation

**Language & Runtime:** Go 1.26.x, single static binary, cross-compiled for darwin/linux
(amd64+arm64). No runtime deps beyond the external provider CLIs and `git` on PATH.

**CLI Layer:** Cobra root command `aictl` with one subcommand per PRD verb; pflag for POSIX
flags (`run --fallback`, `run <provider>`); `aictl completion <shell>` free; per-command
arg validators and help.

**Config & State Serialization:** `goccy/go-yaml` for `config.yaml` (public contract) and
`state.yaml` (Task State) — human-readable, hand-editable (NFR-4), with friendly validation
errors for a user-facing config.

**Code Organization:** addendum package layout — `cmd/aictl` (Cobra wiring) + `internal/`
packages: `session`, `git`, `providers`, `handoff`, `shell` (PTY runner), plus `config` and
`checkpoint`. Business logic stays out of `cmd/` so commands are thin.

**Build/Dev Tooling:** standard `go build`/`go test`; release via GoReleaser-style cross-builds
attached to GitHub releases (per PRD distribution decision) — tool choice finalized later.

**Testing:** Go's stdlib `testing`; PTY runner gets integration tests against a fake
interactive child; handoff generator is pure-function and unit-tested for determinism.

**Note:** Project initialization using the commands above should be the first implementation story.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (block implementation):**
- Git access strategy → **shell out to the `git` binary** (faithful porcelain output, zero added deps).
- Checkpoint invasiveness → **non-invasive patch files** under `.ai-session/`; never mutate the user's branch/index/stash.
- Usage-limit detection confidence → **confirm-on-exit** (pattern match remembered, classified only when the provider process exits).
- State durability ordering → **atomic write-before-exec** (state + handoff fsync'd before any provider starts).
- PTY runner concurrency model → fan-out writer + dedicated I/O goroutines + signal-safe raw-mode restore.

**Important Decisions (shape architecture):**
- Provider adapter model → interface + built-in registry overlaid by config.
- Session Directory layout → split, plaintext, hand-editable files.
- Secret redaction placement → filter in the git-diff → artifact pipeline.
- Exit-reason classification → exit code + detector state.

**Deferred Decisions (post-MVP, out of v1 per PRD):**
- LLM summarization/compaction; cost-aware provider strategies; Windows/ConPTY; live in-run file watcher; release-tooling specifics (GoReleaser vs manual) finalized at packaging time.

### Data Architecture — Session state & on-disk layout

- **Store:** plaintext YAML/Markdown files in the repo-resident `.ai-session/` (no DB, no binary format) — satisfies NFR-4 (inspectable/hand-editable) and "state travels with the repo."
- **Layout:** `state.yaml` (Task State) · `config.yaml` (Config) · `handoff.md` (current Handoff) · `attempts.log` · `command-log.md` · `latest-verify.txt` · `checkpoints/NNNN-{before|after}-{provider}/`.
- **Atomicity & durability (NFR-3):** every mutation writes to a temp file in the same directory, `fsync`s, then `rename`s over the target (atomic on POSIX). The run loop **persists state + handoff and fsyncs before `exec`-ing any provider**, so an instant provider death leaves a complete, resumable pre-run state.
- **Serialization:** `goccy/go-yaml`; Task State is append-oriented (decisions/completed/nextSteps/knownFailures lists) so concurrent-free single-writer mutation is simple.
- **Concurrency guard:** one active Session per working tree (PRD Glossary) enforced via a lockfile in `.ai-session/`.

### Authentication & Security — Safety, secrets, redaction

- **No auth surface:** aictl makes no network/LLM calls (NFR-1); there are no credentials of its own.
- **Never destroy user work (PRD Safety):** checkpoints are pure captures (patch + text); aictl never runs destructive git (`reset`, `checkout --`, `stash drop`) and never writes to the working tree.
- **Secret redaction:** diffs are sourced from `git` over **tracked changes only** (gitignored `.env`/keys are naturally excluded), then passed through a **configurable path denylist** (default e.g. `.env*`, `*.pem`, `*.key`, `id_*`) that strips matching file sections before any Handoff/Checkpoint/transcript is written. Filter lives at the single git-diff → artifact boundary so every consumer is covered.
- **Local-only:** no telemetry; all artifacts stay on disk.

### API & Communication — Provider boundary, child I/O, git

- **Git access:** **shell out** to the `git` binary, capturing porcelain (`git status --short`, `git diff` working+staged, `git log -n N`). Faithful to what the developer sees; diff bounded by `maxDiffChars` (FR-13) at capture time. Git is treated as the primary source of truth (FR-8/9).
- **Provider adapter contract:** a `Provider` interface (`Name`, `Command`, `Args(mode, handoffPath)`, `UsageLimitPatterns`, injection mode). **Built-in registry** (Claude/Codex/Gemini) is the default set; **Config overlays it** — a config entry can override a built-in or define a brand-new provider, unifying built-in and generic adapters (FR-15/16). Default prompt injection is **file-ref** (FR-17): inject "read `.ai-session/handoff.md` and continue," with arg/stdin/paste modes available per adapter. **Whether** to inject is decided by the run loop from Task State — inject only when there is a task to continue (a Goal or recorded progress); a fresh session launches the provider clean so the user drives it. The per-provider **mode** decides *how*. Fallback advances (FR-20) are "continue" runs and reuse the same predicate.
- **Child process I/O (PTY runner):** provider launched via `creack/pty`. Concurrency model:
  - goroutine A: `io.Copy(ptmx, os.Stdin)` — keyboard passthrough;
  - goroutine B: `io.Copy(fanWriter, ptmx)` where `fanWriter` writes once and fans to **{controlling terminal, transcript file, usage detector}** (single pass-through, no extra buffering → NFR-2/NFR-6);
  - SIGWINCH handler: re-`InheritSize` on resize;
  - terminal set to raw mode with **signal-safe + deferred restore** (restore on normal exit, panic, and signal) so the user's terminal is never left broken.
- **Usage detection:** the detector scans the fan-out stream with a small overlap buffer (patterns can straddle reads), case-insensitive, against the provider's configured patterns. A match is **remembered, not acted on mid-run**; classification happens at process exit (confirm-on-exit) to avoid false positives from help text or the handoff echoing a pattern.
- **Exit-reason classification:** combine child exit code + detector state → `success` (0, no match) · `usage_limit` (match + nonzero) · `user_cancel` (SIGINT/130, no match) · `auth_failure`/`crash`/`unknown` (nonzero, pattern-or-code heuristic). Drives the fallback loop (FR-19/20): only `usage_limit` advances; `user_cancel` and other failures stop.

### Frontend Architecture

Not applicable — aictl renders no UI of its own. The provider's native TUI is passed through untouched (PRD Non-Goal: "not a replacement UI"). The PTY runner above is the nearest analog and is covered under API & Communication.

### Infrastructure & Deployment

- **Build:** `go build` to a single static binary; cross-compile darwin/linux × amd64/arm64.
- **Distribution:** `go install` + prebuilt binaries on GitHub releases (PRD decision); release automation tool (e.g. GoReleaser) chosen at packaging time.
- **Runtime prerequisites:** `git` and the chosen provider CLIs present on PATH; aictl probes availability (provider `IsAvailable`) and fails with a clear message.
- **Observability:** structured `attempts.log` (provider, exit reason, checkpoint refs) + per-attempt transcript; aictl prints to the controlling TTY **only before/after** a run (NFR-2/NFR-5), buffering any mid-run notices until the provider exits.

### Decision Impact Analysis

**Implementation sequence (architecture-driven):**
1. Module + Cobra skeleton + `.ai-session/` scaffolding & atomic YAML store (foundation for everything).
2. Git capture (shell-out) + deterministic handoff generator (pure function; testable in isolation).
3. PTY runner (raw mode, fan-out, resize, exit codes) — highest-risk; validate "feels native" early.
4. Usage detector + exit classification layered onto the runner.
5. Provider adapters (built-in registry + config overlay + injection modes).
6. Fallback loop + attempts log + checkpoints (pre/post) wiring it together.
7. `recover`, verify commands, secret denylist polish.

**Cross-component dependencies:**
- Durability ordering couples the **runner** to the **state store** and **handoff generator** (must persist+fsync before exec).
- The **fan-out writer** couples the **runner** to the **usage detector** and **transcript**.
- **Exit classification** couples the **detector** to the **fallback loop**.
- **Config overlay** couples **config** to the **provider registry** and **secret denylist**.
- **Git shell-out** underpins **checkpoints**, **handoff**, and the **files-changed** verdict.

## Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**Critical Conflict Points Identified:** ~10 areas where independent agents could diverge —
package boundaries, error handling, YAML key casing, atomic persistence, command wiring,
context propagation, logging, the PTY cleanup contract, provider registration, and testing.

### Naming Patterns

**Package & file naming:**
- Packages: short, lowercase, no underscores (`session`, `handoff`, `providers`, `git`, `shell`, `config`, `checkpoint`). Package name matches directory.
- Files: lowercase with underscores allowed for grouping (`runner.go`, `usage_detector.go`, `exit_reason.go`). Test files co-located as `*_test.go` (Go standard — **never** a separate `tests/` dir).
- One Cobra command per file under `cmd/aictl/` named after the verb (`run.go`, `start.go`, `checkpoint.go`).

**Go identifier naming:** standard Go — `MixedCaps` exported, `mixedCaps` unexported; initialisms uppercase (`ID`, `PTY`, `URL`); no `Get` prefix on getters. Interfaces named for behavior (`Provider`, not `IProvider`).

**Error variables & types:** sentinels `ErrNoSession`, `ErrSessionLocked`; error types `XxxError`. Exit-reason constants stay `snake_case` *string values* (`"usage_limit"`) but `MixedCaps` Go consts (`ExitUsageLimit`).

**YAML keys (config.yaml & state.yaml):** **`camelCase`** (matches the brief's example: `defaultProvider`, `usageLimitPatterns`, `maxDiffChars`, `includeDiff`). Every struct field carries an explicit `yaml:"..."` tag — never rely on default casing. *[Decision: camelCase; switch to snake_case here if preferred.]*

### Structure Patterns

- **`cmd/aictl/` is thin:** commands parse flags, build a context, and call `internal/` packages. **No business logic in `cmd/`.**
- **`internal/` holds all logic**, one concern per package (per the addendum layout). Cross-package types that everyone needs (e.g. `Session`, `ExitReason`) live in their owning package and are imported, not duplicated.
- **No global mutable state.** Dependencies (config, session, clock, output writer) are passed explicitly so the runner and generators are unit-testable.
- **`.ai-session/` paths are centralized** in one place (e.g. `session.Paths`) — no string-concatenated paths scattered across packages.

### Format Patterns

- **All on-disk writes go through one atomic helper** (`store.WriteAtomic`: temp + fsync + rename). No package calls `os.WriteFile` directly for Session artifacts. This is the single enforcement point for NFR-3 durability.
- **Handoff is generated by a pure function** `handoff.Generate(state, gitState, verify) (string, error)` — no I/O, no clock, no randomness, so output is byte-deterministic and golden-file testable (FR-11).
- **Timestamps:** RFC 3339 / ISO-8601 UTC everywhere (`attempts.log`, checkpoint dir prefixes use zero-padded sequence `0001`, not timestamps, for stable ordering).
- **Git output captured verbatim** (porcelain) — aictl does not reformat diffs; it only truncates at `maxDiffChars` with an explicit `… [truncated N chars]` marker.

### Communication Patterns

- **Context propagation:** every operation that spawns a process or could block takes `ctx context.Context` as its first parameter; cancellation flows from the root command's signal-aware context.
- **Provider registration:** built-in adapters register into a registry; `config` overlays it at load time producing a single resolved provider map. Lookups go through the resolved map only — agents never branch on "is this built-in or config."
- **Exit classification is one function** `classify(exitCode int, detector DetectorState) ExitReason` — the *only* place exit reasons are decided, so the fallback loop and tests share identical logic.

### Process Patterns

- **Error handling:** return wrapped errors with `fmt.Errorf("...: %w", err)`; never `panic` in `internal/` except the runner's terminal-restore path. Top-level `cmd` is the only layer that prints errors and sets process exit code.
- **Terminal cleanup contract (critical):** any code that enters raw mode **must** guarantee restore via `defer` *and* a signal handler — restore is idempotent and runs on normal return, panic, and SIGINT/SIGTERM. A broken terminal is a release blocker.
- **Quiet-supervision logging:** all aictl user-facing output goes through one `ui` writer that is **suppressed between provider start and exit** (buffered, flushed after). No package writes to `os.Stdout/Stderr` directly during a run (NFR-2/NFR-5).
- **Fail-closed before a run:** if pre-run persistence (state/handoff/checkpoint) fails, the provider is **not** launched — durability ordering is never silently skipped.

### Enforcement Guidelines

**All AI Agents MUST:**
- Route every Session-artifact write through `store.WriteAtomic`.
- Keep `handoff.Generate` pure (no I/O/clock/network) and cover it with golden-file tests.
- Take `context.Context` first; never introduce package-level mutable state.
- Resolve providers via the overlaid registry; never special-case built-in vs config.
- Guarantee raw-mode restore on every exit path; never log to the TTY during a provider run.
- Use explicit `yaml:` tags (camelCase) on all serialized structs.

**Pattern enforcement:** `gofmt` + `go vet` + `golangci-lint` in CI; a determinism test for the handoff generator; an integration test asserting the terminal is restored after an interrupted run. Violations are fixed before merge; pattern changes are recorded in this document.

### Pattern Examples

**Good:**
```go
// thin command delegating to internal
func newRunCmd(app *App) *cobra.Command { /* parse flags -> app.Run(ctx, opts) */ }

// atomic, durable write
if err := store.WriteAtomic(paths.State, data); err != nil { return fmt.Errorf("persist state: %w", err) }

// pure, deterministic handoff
md, err := handoff.Generate(state, gitState, verifyOut)
```

**Anti-patterns:**
```go
os.WriteFile(".ai-session/state.yaml", b, 0644) // ✗ bypasses atomic+fsync (NFR-3)
fmt.Println("[aictl] scanning...")              // ✗ writes to TTY during a provider run
if p.builtin { ... } else { ... }               // ✗ special-casing built-in vs config providers
time.Now() inside handoff.Generate              // ✗ breaks determinism (FR-11)
```

## Project Structure & Boundaries

### Complete Project Directory Structure

```
aictl/
├── README.md
├── LICENSE                          # OSS license (e.g. MIT/Apache-2.0 — owner's call)
├── go.mod                           # module github.com/<owner>/aictl; go 1.26
├── go.sum
├── .gitignore
├── .golangci.yml                    # lint config (enforces patterns)
├── .goreleaser.yaml                 # cross-build darwin/linux × amd64/arm64 (packaging step)
├── .github/
│   └── workflows/
│       ├── ci.yml                   # gofmt, go vet, golangci-lint, go test
│       └── release.yml              # tag → build → attach binaries to GitHub release
├── cmd/
│   └── aictl/
│       ├── main.go                  # builds root cmd, signal-aware context, executes
│       ├── root.go                  # cobra root: `aictl`, global flags, App wiring
│       ├── init.go                  # FR-1   `aictl init`
│       ├── start.go                 # FR-2   `aictl start "<goal>"`
│       ├── state.go                 # FR-3   `note` / `done` / `next` / `fail`
│       ├── run.go                   # FR-5, FR-12, FR-20  `aictl run [provider] [--fallback ...]`
│       ├── checkpoint.go            # FR-10  `aictl checkpoint "<label>"`
│       ├── handoff.go               # FR-12  `aictl handoff` / `aictl prepare`
│       ├── verify.go                # FR-18  `aictl verify`
│       └── recover.go               # FR-14  `aictl recover`
├── internal/
│   ├── app/
│   │   └── app.go                   # App struct: deps (config, session, ui, clock); orchestrates use-cases
│   ├── session/
│   │   ├── session.go               # Session, TaskState types; lifecycle (FR-1..4)
│   │   ├── store.go                 # WriteAtomic (temp+fsync+rename); load/save state (NFR-3/4)
│   │   ├── paths.go                 # centralized .ai-session/ path resolution
│   │   └── lock.go                  # one-Session-per-worktree lockfile
│   ├── config/
│   │   ├── config.go                # Config types + yaml tags (camelCase)
│   │   ├── defaults.go              # built-in defaults (denylist, maxDiffChars, Trio)
│   │   └── load.go                  # parse + validate + overlay onto defaults
│   ├── git/
│   │   ├── git.go                   # shell-out wrapper (exec git)
│   │   ├── status.go                # `git status --short`
│   │   ├── diff.go                  # working+staged diff, maxDiffChars bound, denylist filter
│   │   └── commits.go               # `git log -n N`
│   ├── handoff/
│   │   ├── generator.go             # Generate(state, gitState, verify) → pure, deterministic (FR-11)
│   │   ├── templates.go             # embedded handoff template (//go:embed)
│   │   └── recover.go               # minimal-state continuation prompt (FR-14)
│   ├── providers/
│   │   ├── provider.go              # Provider interface, PromptMode, registry
│   │   ├── registry.go              # built-in registry + config overlay → resolved map (FR-15/16)
│   │   ├── claude.go                # built-in adapter
│   │   ├── codex.go                 # built-in adapter
│   │   ├── gemini.go                # built-in adapter
│   │   └── injection.go             # file-ref/arg/stdin/paste prompt injection (FR-17)
│   ├── shell/
│   │   ├── runner.go                # PTY launch, raw mode, goroutines, exit-code (FR-5)
│   │   ├── fanwriter.go             # single-write fan-out → terminal+transcript+detector (FR-6, NFR-2)
│   │   ├── rawmode.go               # raw-mode enable + signal-safe/idempotent restore
│   │   ├── resize.go                # SIGWINCH → InheritSize
│   │   ├── detector.go              # usage-limit stream scan w/ overlap buffer (FR-19)
│   │   └── exit.go                  # classify(exitCode, detectorState) → ExitReason (FR-19)
│   ├── checkpoint/
│   │   └── checkpoint.go            # pre/post non-invasive patch capture (FR-8/9/10)
│   ├── fallback/
│   │   └── chain.go                 # run-with-fallback loop, attempts log (FR-20/21)
│   └── ui/
│       └── ui.go                    # buffered output; silent during a run (NFR-2/5)
├── testdata/
│   ├── handoff/                     # golden files for deterministic handoff tests
│   └── fakecli/                     # tiny fake interactive child for PTY integration tests
└── docs/
    ├── config-reference.md          # config.yaml public contract (versioned surface)
    └── session-layout.md            # .ai-session/ layout (secondary public surface)
```

**Runtime artifact (created in the user's repo, not in this tree):**

```
<user-repo>/.ai-session/
├── state.yaml            # Task State
├── config.yaml           # Config (public contract)
├── handoff.md            # current Handoff
├── attempts.log          # one line per Attempt
├── command-log.md        # aictl-invoked commands (verify, etc.)
├── latest-verify.txt
├── .lock                 # single-session guard
└── checkpoints/
    ├── 0001-before-claude/  {handoff.md, git-status.txt, git-diff.patch, recent-commits.txt}
    └── 0001-after-claude/   {exit-code.txt, git-status.txt, git-diff.patch, transcript.ansi}
```

### Architectural Boundaries

- **`cmd/` ↔ `internal/app`:** commands are thin adapters; all use-cases enter through `app.App`. No logic in `cmd/`.
- **`internal/app` ↔ feature packages:** `app` orchestrates; it does not implement git/PTY/handoff details. One direction of dependency: `app → {session, git, handoff, providers, shell, checkpoint, fallback, config, ui}`. Feature packages do **not** import `app`.
- **Persistence boundary:** only `session.store` touches the filesystem for Session artifacts (via `WriteAtomic`). Other packages return data; `store` persists it.
- **Process boundary:** only `shell` spawns/owns the child PTY; only `shell` manipulates terminal modes. Nothing else reads `os.Stdin` or sets raw mode.
- **Git boundary:** only `internal/git` shells out to `git`. The secret denylist filter lives here so every downstream consumer (handoff, checkpoint) is covered.
- **Output boundary:** only `internal/ui` writes user-facing text, and it is muted between provider start/exit.
- **No network boundary at all** — there is no package permitted to make network/LLM calls (NFR-1); CI can assert no such imports.

### Requirements to Structure Mapping

| FR cluster | Primary location |
|---|---|
| FR-1–4 Session & Task State | `internal/session`, `cmd/aictl/{init,start,state}.go` |
| FR-5–7 PTY runner | `internal/shell/{runner,fanwriter,rawmode,resize}.go`, `cmd/aictl/run.go` |
| FR-8–10 Git checkpointing | `internal/checkpoint`, `internal/git`, `cmd/aictl/checkpoint.go` |
| FR-11–14 Handoff + recover | `internal/handoff`, `cmd/aictl/{handoff,recover}.go` |
| FR-15–18 Adapters & config | `internal/providers`, `internal/config`, `cmd/aictl/verify.go` |
| FR-19–21 Detection & fallback | `internal/shell/{detector,exit}.go`, `internal/fallback`, `cmd/aictl/run.go` |

**Cross-cutting concerns:**
- Atomic durability → `internal/session/store.go` (used everywhere).
- Secret redaction → `internal/git/diff.go` (single filter point).
- Quiet supervision → `internal/ui` + `internal/shell`.
- Config-as-contract → `internal/config` + `docs/config-reference.md`.

### Integration Points

- **Internal data flow (a `run`):** `cmd/run` → `app.Run` → `git.Capture` + `session.Load` → `handoff.Generate` → `store.WriteAtomic` (+fsync) → `checkpoint` (pre) → `shell.Run` (PTY) → `detector` → `exit.classify` → `checkpoint` (post) → `fallback` decides next → `session` update + `attempts.log`.
- **External integrations:** the `git` binary and provider CLIs (`claude`/`codex`/`gemini` or config-defined) on PATH — invoked as child processes only. No SDKs, no APIs.
- **Data flow guarantee:** state + handoff are persisted and fsync'd **before** `shell.Run` is ever called (NFR-3), so the engineering state survives instant provider death.

### Development Workflow Integration

- **Dev:** `go run ./cmd/aictl …`; unit tests co-located; `handoff` golden tests + `shell` PTY integration test against `testdata/fakecli`.
- **Build:** `go build ./cmd/aictl`; release via GoReleaser cross-compiling darwin/linux × amd64/arm64.
- **Deploy/distribute:** `go install github.com/<owner>/aictl/cmd/aictl@latest` + prebuilt binaries on GitHub releases.

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:** All stack choices interoperate without conflict — Go 1.26.4, Cobra
v1.10.2 (+pflag), creack/pty v1.1.24, goccy/go-yaml, shell-out git. No contradictions: the
"no network in core" decision is consistent with deterministic (LLM-free) handoff; shell-out
git is consistent with "faithful diff" and "git is source of truth"; non-invasive checkpoints
are consistent with the safety guardrail.

**Pattern Consistency:** Patterns reinforce the decisions — single atomic-write helper enforces
NFR-3; pure handoff generator enforces FR-11 determinism; one-`classify`-function enforces
consistent exit handling; camelCase YAML tags match the config contract; the raw-mode restore
contract protects NFR-2.

**Structure Alignment:** The package tree realizes every boundary — `session.store` is the only
FS writer, `shell` the only PTY/terminal owner, `git` the only git caller (and the single secret
filter point), `ui` the only output writer. Dependency direction is acyclic (`app →` features;
features never import `app`).

### Requirements Coverage Validation ✅

**Functional Requirements Coverage:** All 21 FRs map to concrete locations (see Requirements-to-
Structure table). Spot checks: FR-11 → pure `handoff.Generate` + golden tests; FR-17 → per-adapter
`injection.go`; FR-20 → `fallback/chain.go` with handoff regeneration on file change; FR-21 →
`attempts.log` with evidence-only rule (no asserted progress without git/file evidence).

**Non-Functional Requirements Coverage:**
- NFR-1 (no network) → enforced by absence of any networking package; CI import-check.
- NFR-2 (TUI non-interference) → `fanwriter` single pass-through + muted `ui`.
- NFR-3 (pre-run durability) → `store.WriteAtomic` + fail-closed ordering before `shell.Run`.
- NFR-4 (inspectable plaintext) → YAML/Markdown only.
- NFR-5 (observability w/o noise) → `attempts.log` + buffered `ui`.
- NFR-6 (~1s startup) → git diff bounded by `maxDiffChars`; single-write I/O path.

### Implementation Readiness Validation ✅

**Decision Completeness:** All critical decisions documented with verified versions and rationale.
**Structure Completeness:** Complete, specific file tree; runtime `.ai-session/` layout specified.
**Pattern Completeness:** All ~10 conflict points have rules + good/anti-pattern examples + CI enforcement.

### Gap Analysis Results

**Critical Gaps:** None.

**Important Gaps:** None blocking. Note: FR-4 (provider-contributed state updates) relies on the
provider writing `state.yaml` per the injected instruction; aictl re-loads and validates on the
next operation, treating malformed/absent updates as a no-op — correctness depends on `config`/
`session` load-time validation being robust (covered by the validation pattern).

**Nice-to-Have / Inherited (from PRD Open Questions — research, not architecture):**
- Per-provider prompt-injection reliability (validate file-ref vs paste per adapter during build).
- Usage-limit pattern maintenance + false-positive bounds (mitigated by confirm-on-exit).
- "Files changed" granularity (git tracked-only vs untracked) — current design uses git diff;
  revisit if untracked-file edits prove material.
- Owner-level choices: OSS license, module path, GoReleaser vs manual release tooling.

### Validation Issues Addressed

No critical or important issues required resolution. The three inherited PRD open questions are
carried forward as build-time research items, not architectural blockers; the mechanisms that
would implement either resolution already exist in the design (configurable injection modes,
configurable usage patterns, git-based change detection).

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed
- [x] Technical constraints identified
- [x] Cross-cutting concerns mapped

**Architectural Decisions**
- [x] Critical decisions documented with versions
- [x] Technology stack fully specified
- [x] Integration patterns defined
- [x] Performance considerations addressed

**Implementation Patterns**
- [x] Naming conventions established
- [x] Structure patterns defined
- [x] Communication patterns specified
- [x] Process patterns documented

**Project Structure**
- [x] Complete directory structure defined
- [x] Component boundaries established
- [x] Integration points mapped
- [x] Requirements to structure mapping complete

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION (all 16 checklist items confirmed; no critical gaps)

**Confidence Level:** High — focused single-process scope, pre-staged design material, every
requirement traced to a location, NFRs enforced structurally rather than by convention.

**Key Strengths:**
- NFRs enforced by architecture (single write/git/PTY/output owners), not just by guidelines.
- Durability ordering and determinism are structural, making the core promise testable.
- Clean extensibility seam (built-in registry overlaid by config) with no special-casing.
- Highest-risk component (PTY runner) isolated and independently testable via a fake child.

**Areas for Future Enhancement:**
- LLM summarization/compaction, cost-aware strategies, Windows/ConPTY, live file watcher (all
  post-v1 per PRD); release-tooling automation finalized at packaging time.

### Implementation Handoff

**AI Agent Guidelines:**
- Follow the architectural decisions and patterns exactly; route writes through `store.WriteAtomic`,
  keep `handoff.Generate` pure, respect package boundaries, never log to the TTY during a run.
- Refer to this document for all architectural questions; respect the dependency direction.

**First Implementation Priority:**
Initialize the module and foundation (architecture-driven sequence step 1):
```bash
go mod init github.com/<owner>/aictl
go get github.com/spf13/cobra@v1.10.2 github.com/creack/pty@v1.1.24 github.com/goccy/go-yaml@latest
```
Then build the `.ai-session/` atomic store + Cobra skeleton before any provider/PTY work.
