---
stepsCompleted: [1, 2, 3, 4]
status: 'complete'
completedAt: '2026-06-15'
inputDocuments:
  - '_bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md'
  - '_bmad-output/planning-artifacts/architecture.md'
  - '_bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md'
---

# aictl - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for aictl, decomposing the requirements from the PRD and Architecture into implementable stories. (No UX Design document exists — aictl is a terminal supervisor that passes provider TUIs through untouched.)

## Requirements Inventory

### Functional Requirements

FR-1: A user can run `aictl init` to create the `.ai-session/` Session Directory with default Config and empty Task State (non-destructive if it already exists).
FR-2: A user can run `aictl start "<goal>"` to record an immutable Goal and initialize Task State, capturing initial repo context.
FR-3: A user can update Task State without any Provider running via `aictl note/done/next/fail`, with no network and no Provider invoked.
FR-4: A Provider may update Task State when it has credits (optimization only); a Session with zero Provider updates still produces a complete Handoff.
FR-5: A user can run `aictl run <provider>` and interact with the Provider's native TUI in a pseudo-terminal (keyboard, resize, exit code preserved).
FR-6: aictl captures a faithful per-Attempt transcript (raw ANSI) without altering on-screen output.
FR-7: aictl prints status only before/after a run and stays silent while the Provider owns the screen.
FR-8: aictl captures a complete pre-run Checkpoint (branch, status, diff, recent commits, command log, summary) before any Provider starts.
FR-9: aictl captures a post-run Checkpoint (exit code, transcript ref, status, diff) and computes a "files changed" verdict from pre/post comparison.
FR-10: A user can run `aictl checkpoint "<label>"` to snapshot repo + task context on demand, with no Provider and no network.
FR-11: aictl generates a deterministic, LLM-free Handoff before each Provider run (byte-stable, anti-redo instructions, works with zero Providers available).
FR-12: A user can run `aictl handoff`/`aictl prepare` to (re)generate the Handoff; `run` regenerates it before each Provider in a chain.
FR-13: aictl bounds Handoff size by truncating large inputs (notably git diff) at a configurable `maxDiffChars` with a clear marker.
FR-14: A user can run `aictl recover` to produce a clean continuation prompt from minimal state (git diff + branch + Goal), with no Provider available.
FR-15: aictl ships working built-in adapters for the Trio (Claude Code, Codex, Gemini) with sensible defaults and usage-limit patterns.
FR-16: A user can define a new Provider entirely in Config (command, args, injection mode + text, usage-limit patterns) and run it via `aictl run <name>`.
FR-17: aictl defaults to file-reference prompt injection ("read handoff.md and continue"); injection mode is configurable per Provider (file-ref/arg/stdin/paste).
FR-18: A user can configure Verify Commands; `aictl verify` runs them, captures output, and subsequent Handoffs embed it.
FR-19: aictl classifies each run's Exit Reason (success/usage_limit/auth_failure/crash/user_cancel/unknown) from exit code + configurable output-pattern matching.
FR-20: A user can run `aictl run --fallback p1,p2,p3`; on usage_limit aictl advances to the next Provider (reusing or regenerating the Handoff); other failures and user_cancel stop the chain.
FR-21: aictl records every Attempt (Provider, Exit Reason, pre/post Checkpoint refs) and never asserts unverified Provider progress (evidence-only rule).

### NonFunctional Requirements

NFR-1: Provider-independence — every core capability functions with zero authenticated/available Providers and no network/LLM in the core path.
NFR-2: TUI non-interference — supervision must not visibly degrade any Provider's interactive UI (render, raw-mode keys, resize).
NFR-3: Pre-run durability ordering — state + Handoff must be persisted (atomic, fsync'd) to disk before any Provider process starts.
NFR-4: Deterministic, inspectable plaintext state — all Session artifacts are human-readable, hand-editable, git-friendly (YAML/Markdown).
NFR-5: Observability without noise — aictl actions are recoverable via attempts.log / Session Directory without printing during a run.
NFR-6: Startup overhead — pre-run prep (Checkpoint + Handoff) adds launch latency well under ~1s on a typical repo; git diff bounded by maxDiffChars.

### Additional Requirements

(Derived from Architecture — technical/implementation requirements that shape epics & stories.)

- **Greenfield foundation (Epic 1 Story 1):** `go mod init github.com/<owner>/aictl`; `go get` cobra v1.10.2, creack/pty v1.1.24, goccy/go-yaml; Go 1.26.x. Cobra root command skeleton + thin `cmd/aictl` layer + `internal/` package scaffolding per the architecture tree.
- **Atomic persistence primitive:** `session/store.go` `WriteAtomic` (temp + fsync + rename) — the single FS-write path for all Session artifacts (enforces NFR-3/NFR-4); foundational for FR-1..4, FR-8..14, FR-21.
- **Centralized path + single-session lock:** `session/paths.go` + `.ai-session/.lock` enforcing one active Session per working tree.
- **Shell-out git layer:** `internal/git` invoking the real `git` binary for status/diff/commits; the single secret-denylist filter point lives here (privacy guardrail).
- **Provider registry + config overlay:** built-in registry overlaid by Config into one resolved provider map (no special-casing built-in vs config).
- **Single exit-classifier function:** `classify(exitCode, detectorState)` shared by runner, fallback loop, and tests.
- **Output fan-out:** single-write `fanwriter` → {terminal, transcript, usage detector}; raw-mode restore contract (idempotent, signal-safe).
- **Quality gates / CI:** gofmt + go vet + golangci-lint + go test; import-check asserting no networking packages (enforces NFR-1); handoff determinism (golden) test; terminal-restore integration test.
- **Distribution:** `go install` + prebuilt darwin/linux × amd64/arm64 binaries on GitHub releases (GoReleaser).
- **Public-surface docs:** `docs/config-reference.md` (config.yaml contract) and `docs/session-layout.md` (.ai-session/ layout).

### UX Design Requirements

None — no UX Design document exists for aictl (CLI supervisor; provider TUIs are passed through untouched per PRD Non-Goal "not a replacement UI").

### FR Coverage Map

FR-1: Epic 1 — `aictl init` Session Directory
FR-2: Epic 1 — `aictl start` Goal + Task State
FR-3: Epic 1 — credit-free state commands (note/done/next/fail)
FR-4: Epic 1 — provider-optional Task State updates
FR-5: Epic 3 — PTY run with native TUI passthrough
FR-6: Epic 3 — faithful transcript capture
FR-7: Epic 3 — quiet supervision during run
FR-8: Epic 3 — pre-run Checkpoint
FR-9: Epic 3 — post-run Checkpoint + files-changed verdict
FR-10: Epic 3 — on-demand Checkpoint
FR-11: Epic 2 — deterministic Handoff generation
FR-12: Epic 2 — handoff/prepare command
FR-13: Epic 2 — bounded Handoff size (maxDiffChars)
FR-14: Epic 2 — `aictl recover` last-resort prompt
FR-15: Epic 3 — built-in Trio adapters
FR-16: Epic 3 — config-driven generic adapter
FR-17: Epic 3 — prompt injection (file-ref default, configurable)
FR-18: Epic 2 — verify commands feed Handoff
FR-19: Epic 4 — Exit Reason classification
FR-20: Epic 4 — auto-fallback chain
FR-21: Epic 4 — Attempt recording (evidence-only)

## Epic List

### Epic 1: Durable Task Sessions
A developer can initialize aictl in a repo and keep a durable, portable record of a coding task — goal, decisions, completed steps, next steps — that survives restarts and travels with the repo, entirely offline.
**FRs covered:** FR-1, FR-2, FR-3, FR-4
**Carries:** greenfield foundation (module + Cobra skeleton, `internal/` scaffolding), atomic write store, centralized paths + single-session lock, CI quality gates (incl. no-network import check). Establishes NFR-1, NFR-4.

### Epic 2: Portable Handoffs & Recovery
A developer can generate a deterministic, portable handoff — and a last-resort `recover` prompt — from current git + task state, usable by hand with any tool, working even with zero providers available.
**FRs covered:** FR-11, FR-12, FR-13, FR-14, FR-18
**Carries:** shell-out git layer (status/diff/commits), secret-denylist filter, verify commands. Establishes NFR-6; reinforces NFR-1.

### Epic 3: Supervised Provider Runs
A developer can run a real provider CLI (Trio or config-defined) through aictl, keeping its native TUI intact, with the handoff auto-injected and the run checkpointed before and after.
**FRs covered:** FR-5, FR-6, FR-7, FR-8, FR-9, FR-10, FR-15, FR-16, FR-17
**Carries:** PTY runner, single-write fan-out, raw-mode/resize restore contract, provider registry + config overlay, pre/post/on-demand checkpoints. Establishes NFR-2, NFR-3 (pre-run ordering).

### Epic 4: Automatic Provider Fallback
A developer can run a provider chain that automatically switches to the next provider on a usage limit, with every attempt recorded — the "never lose a session to quota" promise — and ship v1.0.
**FRs covered:** FR-19, FR-20, FR-21
**Carries:** usage detection, single exit-classifier, fallback loop, attempts log. Establishes NFR-5. Plus distribution (`go install` + GitHub release binaries) and public-surface docs (`config-reference.md`, `session-layout.md`) as the "ship it" stories.

## Epic 1: Durable Task Sessions

A developer can initialize aictl in a repo and keep a durable, portable record of a coding task that survives restarts and travels with the repo, entirely offline.

### Story 1.1: Project foundation & `aictl` command skeleton

As the maintainer,
I want the Go module, Cobra command skeleton, internal package layout, and CI quality gates established,
So that every later story builds on a consistent, enforced foundation.

**Acceptance Criteria:**

**Given** a clean repository
**When** the foundation story is implemented
**Then** `go mod init github.com/<owner>/aictl` exists with Go 1.26 and dependencies cobra v1.10.2, creack/pty v1.1.24, and goccy/go-yaml
**And** the `cmd/aictl` Cobra root plus `internal/` packages (`app`, `session`, `config`, `git`, `handoff`, `providers`, `shell`, `checkpoint`, `fallback`, `ui`) are scaffolded per the architecture tree
**And** `aictl --help` and `aictl --version` run successfully with no subcommand logic yet

**Given** the CI pipeline
**When** it runs on a push
**Then** gofmt, `go vet`, golangci-lint, and `go test ./...` all execute
**And** an import-check fails the build if any package imports a networking library (enforces NFR-1)

### Story 1.2: Initialize a Session Directory (`aictl init`)

As a developer,
I want to run `aictl init` in my repo,
So that aictl has a durable place to keep my task state.

**Acceptance Criteria:** (FR-1, NFR-3, NFR-4)

**Given** a repo with no `.ai-session/`
**When** I run `aictl init`
**Then** `.ai-session/` is created containing `config.yaml` (defaults), `state.yaml` (empty Task State), and a `checkpoints/` directory
**And** ignore/keep entries are added so transient/sensitive artifacts are not accidentally committed

**Given** a repo that already has `.ai-session/`
**When** I run `aictl init`
**Then** existing state is not destroyed (no-op or explicit refusal)

**Given** any write of a Session artifact
**When** it is persisted
**Then** it is written via the atomic helper (temp file + fsync + rename), and all files are human-readable plaintext

### Story 1.3: Start a Session with a Goal (`aictl start`)

As a developer,
I want to run `aictl start "<goal>"`,
So that my task has a recorded, immutable objective and aictl knows my starting repo state.

**Acceptance Criteria:** (FR-2)

**Given** an initialized repo
**When** I run `aictl start "Refactor the classifier"`
**Then** the Goal is persisted verbatim to Task State and is immutable for the Session's life
**And** the initial repo context (branch, clean/dirty status) is captured

**Given** an already-active Session in the same working tree
**When** I run `aictl start` again
**Then** the second start is refused (single-session lockfile)

**Given** a repo with no prior `aictl init`
**When** I run `aictl start`
**Then** aictl behaves deterministically and documented (auto-init or a clear instruction to run `init`)

### Story 1.4: Maintain Task State offline (`note` / `done` / `next` / `fail`)

As a developer,
I want to record decisions, completed steps, next steps, and failures without any provider running,
So that my task state never depends on a live provider or network.

**Acceptance Criteria:** (FR-3, FR-4, NFR-1)

**Given** an active Session
**When** I run `aictl note`, `done`, `next`, or `fail` with a message
**Then** the message appends to the corresponding Task State field and is persisted immediately
**And** the command completes with no network access and no provider invoked

**Given** Task State on disk
**When** I open `state.yaml`
**Then** it is readable and hand-editable

**Given** a Session where a provider wrote Task State updates per its injected instruction
**When** aictl next loads the Session
**Then** valid updates are consumed, and malformed or absent updates are treated as a no-op without corrupting or blocking the Session

## Epic 2: Portable Handoffs & Recovery

A developer can generate a deterministic, portable handoff (and a last-resort recovery prompt) from current git + task state, usable by hand with any tool, even with zero providers available.

### Story 2.1: Capture faithful git state

As a developer,
I want aictl to capture my branch, status, diff, and recent commits from the real `git` binary,
So that handoffs reflect exactly what I see, with secrets excluded.

**Acceptance Criteria:** (FR-13, NFR-6, privacy guardrail)

**Given** a repo with changes
**When** aictl captures git state
**Then** it shells out to `git` for `status --short`, working+staged `diff`, and `log -n N`, capturing output verbatim
**And** the diff is truncated at the configurable `maxDiffChars` with an explicit `… [truncated N chars]` marker

**Given** tracked files matching the configurable secret denylist (e.g. `.env*`, `*.pem`)
**When** the diff is captured
**Then** those file sections are stripped before any consumer receives the diff
**And** the entire capture works offline

### Story 2.2: Generate a deterministic Handoff (`aictl handoff` / `prepare`)

As a developer,
I want to generate a portable `handoff.md` from my task and git state,
So that I can hand my in-progress task to any tool without re-explaining it.

**Acceptance Criteria:** (FR-11, FR-12, NFR-1)

**Given** a Session with task and git state
**When** I run `aictl handoff` (or `aictl prepare`)
**Then** `handoff.md` is written from a fixed template sourcing Goal, branch, status, diff, recent commits, latest verify output, command log, and plan — with no LLM/network call, exiting 0 without invoking a provider

**Given** identical inputs
**When** the Handoff is generated twice
**Then** the output is byte-identical (deterministic; covered by a golden-file test)

**Given** any generated Handoff
**When** I read it
**Then** it contains explicit anti-redo instructions ("continue from current state; do not restart; inspect changed files first; run verification before finishing")

### Story 2.3: Verify commands feed the Handoff (`aictl verify`)

As a developer,
I want to configure verification commands and run them,
So that the next handoff carries the real build/test state.

**Acceptance Criteria:** (FR-18)

**Given** configured Verify Commands
**When** I run `aictl verify`
**Then** the commands run, their combined output is stored to `latest-verify.txt`, and the invocation is recorded in `command-log.md`

**Given** stored verify output
**When** the next Handoff is generated
**Then** the latest verify output appears in it

### Story 2.4: Last-resort recovery (`aictl recover`)

As a developer,
I want to generate a clean continuation prompt from minimal state,
So that I have a safety net even when other state is missing and no provider is available.

**Acceptance Criteria:** (FR-14, NFR-1)

**Given** a repo with a current diff, branch, and an original Goal
**When** I run `aictl recover`
**Then** a usable continuation prompt is produced from only those inputs
**And** it works with no provider available and even if other Session state is missing

## Epic 3: Supervised Provider Runs

A developer can run a real provider CLI through aictl, keeping its native TUI intact, with the handoff auto-injected and the run checkpointed before and after.

### Story 3.1: Run a provider in a pseudo-terminal (`aictl run <provider>`)

As a developer,
I want to run a provider CLI through aictl and use its native terminal UI,
So that the supervisor is invisible and the tool behaves exactly as if launched directly.

**Acceptance Criteria:** (FR-5, NFR-2)

**Given** an installed provider CLI
**When** I run `aictl run <provider>`
**Then** the provider launches in a pseudo-terminal and its native UI renders without distortion
**And** keyboard input (including raw-mode/interactive keys) reaches the provider, and terminal resize (SIGWINCH) reflows its UI
**And** the provider's exit code is preserved and surfaced

**Given** a run that ends normally, panics, or is interrupted by a signal
**When** the run terminates
**Then** the terminal is restored to its prior mode every time (idempotent, signal-safe restore — covered by an integration test)

### Story 3.2: Faithful transcript & quiet supervision

As a developer,
I want aictl to record the run and stay silent while the provider owns the screen,
So that I get a faithful log without any UI interference.

**Acceptance Criteria:** (FR-6, FR-7, NFR-2, NFR-5)

**Given** a provider run
**When** output streams from the provider
**Then** a per-Attempt raw-ANSI transcript is written, and on-screen output is unchanged by the capture (single-write fan-out)

**Given** a provider run in progress
**When** the provider UI is active
**Then** aictl emits no output to the controlling terminal between provider start and exit
**And** aictl prints concise status only before (handoff prepared, launching) and after (exit reason, file-change verdict, next action)

### Story 3.3: Provider adapters — Trio, config-driven, and injection

As a developer,
I want built-in adapters for Claude/Codex/Gemini and the ability to add my own via config,
So that I can supervise any CLI with the handoff injected appropriately.

**Acceptance Criteria:** (FR-15, FR-16, FR-17)

**Given** an installed, authenticated Trio provider
**When** I run `aictl run claude|codex|gemini`
**Then** its native UI opens with sensible default command and default usage-limit patterns

**Given** a provider defined only in `config.yaml` (command, args, injection mode + text, usage-limit patterns)
**When** I run `aictl run <name>`
**Then** it is runnable with no code change, resolved through the same registry as built-ins

**Given** the default injection mode
**When** a provider is launched
**Then** aictl injects a short file-reference instruction ("read `.ai-session/handoff.md` and continue; do not restart from scratch")
**And** the injection mode is configurable per provider (file-ref/arg/stdin/paste), with a paste fallback for providers that cannot read files

### Story 3.4: Pre/post-run checkpoints with durability ordering

As a developer,
I want aictl to checkpoint my repo before and after each run, persisting state before the provider starts,
So that my progress survives even instant provider failure.

**Acceptance Criteria:** (FR-8, FR-9, NFR-3)

**Given** a provider about to run
**When** `aictl run` executes
**Then** the Handoff and a pre-run Checkpoint (branch, status, diff, recent commits, command log, summary) are persisted and fsync'd to `checkpoints/NNNN-before-<provider>/` BEFORE the provider process starts
**And** if pre-run persistence fails, the provider is not launched (fail-closed)

**Given** a provider that exits (including instantly)
**When** the run ends
**Then** a post-run Checkpoint (exit code, transcript ref, status, diff) is written to `checkpoints/NNNN-after-<provider>/`
**And** a "files changed during this run" verdict is computed from the pre/post git comparison

### Story 3.5: On-demand checkpoint (`aictl checkpoint`)

As a developer,
I want to snapshot my repo and task context on demand,
So that I can capture a labeled recovery point any time.

**Acceptance Criteria:** (FR-10)

**Given** an active Session
**When** I run `aictl checkpoint "<label>"`
**Then** a labeled Checkpoint (patch + status + commits text files) is written under `checkpoints/`
**And** it works with no provider running and no network access, never mutating the working tree, branch, or index

## Epic 4: Automatic Provider Fallback

A developer can run a provider chain that automatically switches to the next provider on a usage limit, with every attempt recorded — and ship v1.0.

### Story 4.1: Classify provider Exit Reason

As a developer,
I want aictl to reliably classify why a run ended,
So that fallback decisions are correct and false positives are avoided.

**Acceptance Criteria:** (FR-19)

**Given** a provider run whose output contains a configured usage-limit phrase
**When** the phrase appears mid-run
**Then** the match is remembered but not acted on until the provider process exits (confirm-on-exit)
**And** matching is case-insensitive and detects phrases that straddle read boundaries (overlap buffer)

**Given** a finished run
**When** aictl classifies it
**Then** a single `classify(exitCode, detectorState)` function returns one of `success` (0, no match), `usage_limit` (match + nonzero), `user_cancel` (SIGINT/130, no match), or `auth_failure`/`crash`/`unknown`
**And** usage-limit patterns are user-configurable per provider

### Story 4.2: Auto-fallback across a provider chain (`run --fallback`)

As a developer,
I want `aictl run --fallback p1,p2,p3` to switch providers automatically on a usage limit,
So that I never lose a session to quota.

**Acceptance Criteria:** (FR-20)

**Given** a fallback chain where the current provider exits `usage_limit`
**When** aictl advances
**Then** it launches the next provider; if no files changed it reuses the pre-run Handoff, and if files changed it regenerates the Handoff from updated git state first

**Given** a run that ends in a non-usage failure (`crash`/`auth_failure`) or `user_cancel`
**When** the run ends
**Then** aictl stops the chain (diagnostic for failures; intentional halt for cancel) rather than advancing

**Given** a run that ends in `success`
**When** the run ends
**Then** aictl runs verification, updates the Session, and stops the chain

**Given** every provider in the chain has been exhausted
**When** the chain ends
**Then** aictl reports "all providers exhausted" and leaves a complete Session usable by `aictl recover`

### Story 4.3: Record every Attempt (attempts log)

As a developer,
I want every provider attempt recorded with evidence,
So that I have an auditable history and the handoff never lies about progress.

**Acceptance Criteria:** (FR-21, NFR-5)

**Given** a chain of one or more runs
**When** each run completes
**Then** `attempts.log` records the provider, Exit Reason, and pre/post Checkpoint references for that Attempt

**Given** any generated Handoff or Task State
**When** it describes progress
**Then** it never asserts "Provider X did Y" unless backed by git/file/verify evidence

### Story 4.4: Ship v1.0 — distribution & public-surface docs

As a developer who wants to install aictl,
I want installable binaries and documented public surfaces,
So that I can adopt the tool and rely on stable contracts.

**Acceptance Criteria:** (Additional requirements: distribution, public-surface docs)

**Given** a tagged release
**When** the release pipeline runs
**Then** `go install github.com/<owner>/aictl/cmd/aictl@latest` works, and prebuilt darwin/linux × amd64/arm64 binaries are attached to the GitHub release

**Given** the public surfaces
**When** I read the docs
**Then** `docs/config-reference.md` documents the `config.yaml` contract and `docs/session-layout.md` documents the `.ai-session/` layout
**And** the README contains a quickstart (`init` → `start` → `run --fallback`)
