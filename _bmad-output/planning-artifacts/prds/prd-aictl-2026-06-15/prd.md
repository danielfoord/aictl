---
title: aictl
status: final
created: 2026-06-15
updated: 2026-06-15
---

# PRD: aictl
*Working title — confirmed via project config (`project_name: aictl`). Brief floated alternatives (agentmux, relay, codemux); not adopted.*

## 0. Document Purpose

This PRD is for the maintainer (Daniel) and future open-source contributors who will build, review, and extend `aictl`. It defines **what v1.0 must do and why**, scoped to a public open-source release — capabilities, not implementation. Vocabulary is anchored in the Glossary (§3); every feature groups its Functional Requirements (FRs) with globally stable IDs; assumptions are tagged inline as `[ASSUMPTION]` and indexed in §9. Implementation-level "how" from the source brief (Go internals, PTY library usage, code shapes, package layout, prompt-injection mechanics) has been moved to `addendum.md` and is the input to a later architecture/solution-design pass — this PRD does not duplicate it. Source: `BRIEF.md` (project root).

## 1. Vision

Agentic coding CLIs — Claude Code, Codex, Gemini CLI, and the growing field behind them — each lock the important state of a coding task inside their own private session format. When one hits a usage limit, the next tool has no idea what the goal was, what files changed, what decisions were made, or what to do next. The developer either waits hours for quota to reset or restarts the task from scratch with a different tool. That is lost work and lost momentum, and it happens precisely at the moment of highest frustration.

`aictl` is a **provider-agnostic session supervisor** for agentic coding CLIs. It does not replace those tools or their UIs — it wraps the real CLI in a pseudo-terminal so the developer interacts with Claude Code (or Codex, or Gemini) exactly as they do today, while `aictl` quietly owns a durable, portable record of the task outside any provider: the goal, the git state, the commands and tests run, the decisions, and the next step. The governing principle is that **the orchestrator is the source of truth, not the AI CLI**. Because an exhausted provider can die instantly — too dead to summarize itself — `aictl` generates the handoff packet *before* every risky provider call, not after. The provider is treated as a disposable, stateless worker; the session memory belongs to `aictl`.

The result: when a provider runs out of credits, the next one picks up from a clean, highly-informed state with no work lost and no manual re-explaining. For developers who juggle multiple AI coding subscriptions to stretch their quota, `aictl` turns "my session just died" from a dead end into a one-command switch. **Never lose an agentic coding workflow to quota limits again.**

## 2. Target User

### 2.1 Jobs To Be Done

- **When my AI coding CLI hits a usage limit mid-task**, I want to continue the *same* task on a different provider without losing progress, so I keep momentum instead of waiting hours or restarting.
- **When I switch providers**, I want the next tool to know the goal, what changed, what's failing, and the next step, so it doesn't redo finished work or undo my decisions.
- **When I run an AI coding CLI through a wrapper**, I want its real terminal UI — keybindings, approval prompts, rendering — to behave exactly as native, so the supervisor is invisible until I need it.
- **When all my providers are exhausted at once**, I want a portable handoff I can still generate from local state alone, so I'm never blocked on a live provider to recover.
- **As a developer who uses more than one agentic CLI**, I want a single, durable, git-resident record of each task that travels with the repo, so the task survives any individual tool's failure.
- **As an open-source user with a CLI not built in**, I want to register my own provider via config, so I'm not blocked waiting for the maintainer to add it.

### 2.2 Non-Users (v1)

- Developers who use a single AI CLI and never hit limits — the core value (portable cross-provider handoff) doesn't apply.
- Teams wanting a shared/remote orchestration server or multi-user session sync — `aictl` v1 is local and single-user, repo-resident.
- Users wanting `aictl` to *be* an AI agent (make model calls, edit code itself) — it is a supervisor of other tools, not an agent. `[ASSUMPTION]`
- Windows users — deferred (see §5, §"Language/Runtime Targets").

### 2.3 Key User Journeys

*CLI dev tool — journeys are lightweight. Personas are developers; context is what matters.*

- **UJ-1. Daniel survives an instant quota wall.**
  Daniel is mid-refactor with Claude Code under `aictl run --fallback claude,codex,gemini`. Claude prints "usage limit reached, try again in 5 hours" within seconds — before touching a file. `aictl` detects the usage pattern, sees no file changes, records the attempt as `usage_limit`, and launches Codex in its own real UI with the same pre-generated handoff. Codex opens already knowing the goal and the current diff. **No work lost; one command; Daniel keeps typing.** Realizes the core promise.

- **UJ-2. Mid-task handoff with real progress.**
  Codex worked for 20 minutes — added a domain model and a migration — then hit its own limit. Because changes exist on disk, `aictl` regenerates the handoff from the *updated* git state before launching the next provider, so Gemini sees the actual current diff and the failing test, not stale context. Realizes UJ-1's promise under partial-progress conditions.

- **UJ-3. Recover when everything is dead.**
  Every provider is exhausted. Daniel runs `aictl recover`. From git diff + branch + original goal + task state alone, `aictl` writes a clean continuation prompt to disk that Daniel can hand to any tool later — no live provider required.

- **UJ-4. Run a provider that isn't built in.**
  Daniel uses a niche CLI. He adds a block to `.ai-session/config.yaml` — command, args, prompt-injection mode, usage-limit patterns — and `aictl run mytool` supervises it like any first-class provider. Realizes the extensibility JTBD.

- **UJ-5. The supervisor stays invisible.**
  Throughout, `aictl` prints only a couple of lines *before* a provider starts ("Prepared handoff", "Launching Claude Code") and *after* it exits ("usage limit detected", "no file changes", "launching Codex"). While the provider owns the screen, `aictl` is silent and the TUI renders and resizes correctly. The tool feels like running `claude` directly.

## 3. Glossary

- **Session** — The durable, repo-resident unit of work `aictl` manages for one coding task. Has one **Goal**, lives in the **Session Directory**, persists across provider switches and process restarts. **Exactly one active Session per repo working tree**; attempting to start a second is refused.
- **Session Directory** — The `.ai-session/` folder at the repo root holding all Session state (Task State, Handoff, Checkpoints, logs, Config). Travels with the repo; not a global cache.
- **Goal** — The original task description the user gives at Session start. Immutable for the Session's life. `[ASSUMPTION]`
- **Task State** — The provider-independent, `aictl`-owned record of the Session: Goal, completed steps, decisions, known failures, next steps, verify commands. Persisted as structured data (`state.yaml`). The "durable task state" layer.
- **Provider** — An external agentic coding CLI that `aictl` supervises (e.g. Claude Code, Codex, Gemini, or a config-defined generic CLI). Treated as a disposable, stateless worker.
- **Provider Adapter** — The definition that tells `aictl` how to invoke a Provider: command, args, prompt-injection mode, usage-limit patterns. Built-in for the Trio; user-definable via Config for any other CLI.
- **Trio** — The three Providers with tested built-in adapters in v1: Claude Code, Codex, Gemini CLI.
- **Handoff** — The portable, deterministically-generated continuation packet (`handoff.md`) assembled from Task State + git state + verify output. Generated *before* each Provider run. The thing the next Provider reads to continue.
- **Checkpoint** — A timestamped snapshot of repo + task context (git status, git diff, recent commits, command log, summary) captured before and after each Provider run. Lives under `Session Directory/checkpoints/`.
- **Run** — One supervised execution of one Provider against the current Session, inside a pseudo-terminal.
- **Attempt** — A recorded Run outcome: which Provider, exit reason, pre/post Checkpoint references. Multiple Attempts may occur in one Fallback Chain.
- **Fallback Chain** — An ordered list of Providers `aictl` tries in sequence within a single `run --fallback` invocation, advancing on usage-limit exits.
- **Exit Reason** — `aictl`'s classification of how a Run ended: `success`, `usage_limit`, `auth_failure`, `crash`, `user_cancel`, `unknown`.
- **Usage-Limit Detection** — Pattern-matching the Provider's terminal output stream against configured phrases to classify a Run as `usage_limit`.
- **Prompt Injection** — How `aictl` delivers the continuation instruction to a Provider at Run start. Default mode references the Handoff file on disk rather than pasting its full text.
- **Verify Command** — A configured command (e.g. `go test ./...`, `npm run build`) `aictl` can run to capture current build/test state into the Handoff.
- **Config** — `Session Directory/config.yaml`: default provider, Provider Adapter definitions, verify commands, handoff options.

## 4. Features

*FRs are numbered globally (FR-1…FR-N) for stable downstream reference. Tech "how" lives in `addendum.md`.*

### 4.1 Session lifecycle & task state

**Description:** `aictl` creates and maintains a repo-resident Session that is the durable source of truth for a coding task. `aictl init` scaffolds the Session Directory; `aictl start "<goal>"` opens a Session with an immutable Goal. The user (and, opportunistically, a Provider) maintains Task State through small, credit-free commands — because state maintenance must never depend on a live Provider. Realizes UJ-3, and underwrites every other feature.

**Functional Requirements:**

#### FR-1: Initialize a Session Directory

A user can run `aictl init` in a repo to create the `.ai-session/` Session Directory with a default Config and empty Task State.

**Consequences (testable):**
- Running `init` creates `.ai-session/` with at least `config.yaml`, `state.yaml`, and a `checkpoints/` directory.
- Running `init` when `.ai-session/` already exists does not destroy existing state (no-op or explicit refusal). `[ASSUMPTION]`
- `init` adds appropriate ignore/keep entries so transient artifacts and secrets are not accidentally committed. `[ASSUMPTION — see §Constraints/Privacy]`

#### FR-2: Start a Session with a Goal

A user can run `aictl start "<goal>"` to record the immutable Goal and initialize Task State. Realizes UJ-1.

**Consequences (testable):**
- Goal text is persisted verbatim to Task State.
- Starting a Session captures the initial repo context (branch, clean/dirty status).
- `start` without a prior `init` either auto-initializes or instructs the user to run `init` (deterministic, documented behavior). `[ASSUMPTION]`

#### FR-3: Maintain Task State via credit-free commands

A user can update Task State without any Provider running, via `aictl note "<decision>"`, `aictl done "<completed step>"`, `aictl next "<next step>"`, and `aictl fail "<known failure>"`.

**Consequences (testable):**
- Each command appends to the corresponding Task State field (decisions, completed, nextSteps, knownFailures) and persists immediately.
- All four commands succeed with no network access and no Provider invoked.
- Task State is human-readable and hand-editable (structured YAML).

#### FR-4: Provider may update Task State as an optimization

When a Provider has credits, the injected instruction asks it to update Task State before finishing; `aictl` consumes those updates on the next operation. This is an **optimization, never a dependency** — if the Provider dies first, the last user-maintained Task State still stands.

**Consequences (testable):**
- A Session with zero Provider-contributed updates can still produce a complete Handoff from user-maintained Task State alone.
- Malformed or missing Provider updates do not corrupt or block the Session.

### 4.2 Interactive provider runner (PTY)

**Description:** `aictl run <provider>` launches the real Provider CLI inside a pseudo-terminal so its native UI works unchanged — rendering, keyboard shortcuts, raw-mode input, approval flows, and resize all behave as if run directly. `aictl` passes keyboard input through, mirrors output to the screen, and simultaneously tees output to a transcript and scans it for usage-limit patterns — without corrupting the TUI. Realizes UJ-5. This is the foundational runner all other run-time features build on.

**Functional Requirements:**

#### FR-5: Run a Provider in a pseudo-terminal with full passthrough

A user can run `aictl run <provider>` and interact with the Provider's native terminal UI as if launched directly.

**Consequences (testable):**
- Keyboard input (including raw-mode/interactive keys) reaches the Provider.
- Provider output renders to the user's terminal without visible distortion from `aictl`.
- Terminal resize (SIGWINCH) propagates to the Provider so the UI reflows correctly.
- The Provider's exit code is preserved and surfaced by `aictl`.

#### FR-6: Capture a faithful transcript without disturbing the UI

While mirroring Provider output to the screen, `aictl` records the output stream to a per-Attempt transcript.

**Consequences (testable):**
- A transcript file is written under the Attempt's directory for each Run.
- Transcript capture does not alter what the user sees on screen.
- Raw terminal (ANSI) output is preserved faithfully for the log. `[ASSUMPTION: ANSI-fidelity transcript per brief Milestone 2]`

#### FR-7: Keep the supervisor quiet while the Provider owns the screen

`aictl` prints its own status only before a Provider starts and after it exits; it stays silent while the Provider's UI is active. Realizes UJ-5.

**Consequences (testable):**
- No `aictl` log lines are emitted to the controlling terminal between Provider UI start and Provider exit.
- Pre-run and post-run status lines are concise (handoff prepared, launching, exit reason, file-change verdict, next action).

### 4.3 Git checkpointing

**Description:** Git is `aictl`'s primary source of truth for what actually changed. Around every Run, `aictl` captures a Checkpoint before and after, so progress survives even instant Provider failure and the next Provider can see exactly what changed. `aictl checkpoint "<label>"` also lets the user snapshot on demand.

**Functional Requirements:**

#### FR-8: Capture a pre-run Checkpoint before every Provider Run

Before launching any Provider, `aictl` captures a pre-run Checkpoint of repo and task context.

**Consequences (testable):**
- The pre-run Checkpoint includes branch, git status, git diff (working + staged), recent commits, command log, and current summary.
- The pre-run Checkpoint exists *before* the Provider process starts (verifiable by ordering).
- A Provider that fails instantly still leaves a complete, resumable pre-run Checkpoint.

#### FR-9: Capture a post-run Checkpoint after every Provider Run

After a Provider exits, `aictl` captures a post-run Checkpoint and computes whether files changed during the Run.

**Consequences (testable):**
- The post-run Checkpoint includes exit code, transcript reference, git status, and git diff.
- `aictl` records a boolean "files changed during this Run" derived from pre/post git comparison.
- When no changes occurred, the system can reuse the pre-run Handoff for the next Provider (see FR-20).

#### FR-10: On-demand Checkpoint

A user can run `aictl checkpoint "<label>"` at any time to snapshot current repo + task context with a human label.

**Consequences (testable):**
- A labeled Checkpoint is written under `checkpoints/`.
- The command works with no Provider running and without network access.

### 4.4 Handoff generation (deterministic)

**Description:** The Handoff is the heart of the tool: a portable continuation packet assembled from a fixed template filled with Task State + git state + verify output. It is **deterministic and LLM-free** so it can always be produced — even when every Provider is exhausted. It is generated *before* each risky Provider call. `aictl handoff` writes it on demand; `aictl recover` produces a clean continuation prompt from minimal state as a safety net. Realizes UJ-2, UJ-3.

**Functional Requirements:**

#### FR-11: Generate a deterministic Handoff before each Provider Run

`aictl` generates `handoff.md` from a template, sourcing Goal, branch, git status, git diff, recent commits, latest verify output, command log, and the known plan — with no LLM call.

**Consequences (testable):**
- Handoff generation succeeds with zero Providers available/authenticated.
- Given identical inputs, the Handoff is byte-stable (deterministic).
- The Handoff contains explicit anti-redo instructions ("continue from current state; do not restart; inspect changed files first; run verification before finishing").
- The Handoff is generated before the Provider process starts.

#### FR-12: On-demand and pre-run Handoff command

A user can run `aictl handoff` (or `aictl prepare`) to (re)generate the current Handoff; `aictl run` performs this generation automatically before launching.

**Consequences (testable):**
- `handoff`/`prepare` writes `handoff.md` and exits 0 without invoking a Provider.
- `run` regenerates the Handoff immediately before each Provider in a Fallback Chain.

#### FR-13: Bound Handoff size

`aictl` truncates large inputs (notably git diff) to a configured maximum so the Handoff stays usable.

**Consequences (testable):**
- A diff exceeding the configured `maxDiffChars` is truncated with a clear marker indicating truncation.
- The truncation limit is configurable in Config.

#### FR-14: `recover` — last-resort continuation prompt

A user can run `aictl recover` to generate a clean continuation prompt from minimal state (current git diff + branch + original Goal) even if other state is missing.

**Consequences (testable):**
- `recover` produces a usable continuation prompt using only git diff, branch, and Goal.
- `recover` works with no Provider available.

### 4.5 Provider adapters & configuration

**Description:** `aictl` ships tested adapters for the Trio (Claude Code, Codex, Gemini) and supports a config-driven generic adapter so users can register any CLI without code changes. The Config (`config.yaml`) is the user-facing contract: default provider, per-Provider command/args, prompt-injection mode and text, usage-limit patterns, verify commands, and handoff options. Realizes UJ-4.

**Functional Requirements:**

#### FR-15: Built-in Trio adapters

`aictl` provides working built-in Provider Adapters for Claude Code, Codex, and Gemini CLI, each with sensible default command, prompt-injection, and usage-limit patterns.

**Consequences (testable):**
- `aictl run claude|codex|gemini` works against an installed, authenticated instance of each, opening its native UI.
- Each built-in adapter ships default usage-limit patterns appropriate to that Provider.

#### FR-16: Config-driven generic adapter

A user can define a new Provider entirely in Config (command, args, prompt-injection mode + text, usage-limit patterns) and run it via `aictl run <name>`.

**Consequences (testable):**
- A Provider defined only in Config (no code change) is runnable.
- Config-defined usage-limit patterns drive Usage-Limit Detection for that Provider.

#### FR-17: Default prompt injection by file reference

By default, `aictl` injects a short instruction telling the Provider to read `handoff.md` and continue, rather than pasting the full Handoff into the terminal. Other injection modes (arg, stdin, paste) are configurable per Provider.

**Consequences (testable):**
- Default injection text references the Handoff file and instructs "do not restart from scratch."
- Injection mode is configurable per Provider in Config (file-ref / arg / stdin / paste).
- For Providers that cannot read files, a paste fallback delivers the Handoff text. `[ASSUMPTION]`

#### FR-18: Verify commands feed the Handoff

A user can configure Verify Commands; `aictl verify` runs them and captures output, which subsequent Handoffs embed.

**Consequences (testable):**
- `aictl verify` runs configured commands and stores their combined output.
- The latest verify output appears in the next generated Handoff.

### 4.6 Usage-limit detection & auto-fallback

**Description:** `aictl` classifies how each Run ends and, in a Fallback Chain, advances to the next Provider on a usage-limit exit — reusing the pre-generated Handoff (or regenerating it if files changed). This is what makes "my session just died" a one-command recovery. Realizes UJ-1, UJ-2.

**Functional Requirements:**

#### FR-19: Classify Provider Exit Reason

`aictl` classifies each Run's outcome into an Exit Reason using exit code and output-stream pattern matching.

**Consequences (testable):**
- A Run whose output matches configured usage-limit patterns is classified `usage_limit`.
- Exit Reasons distinguish at least: `success`, `usage_limit`, `crash`/`unknown`, `user_cancel`.
- Detection is treated as fuzzy; patterns are user-configurable per Provider (see FR-16).

#### FR-20: Auto-fallback across a Provider chain

A user can run `aictl run --fallback p1,p2,p3`; on a `usage_limit` exit `aictl` advances to the next Provider with an appropriate Handoff. Realizes UJ-1.

**Consequences (testable):**
- On `usage_limit`, `aictl` launches the next Provider in the chain automatically.
- If no files changed during the failed Run, the existing pre-run Handoff is reused; if files changed, the Handoff is regenerated from updated git state before the next Provider starts (realizes UJ-2).
- On a non-usage failure (e.g. `crash`, `auth_failure`), `aictl` stops and shows a diagnostic rather than blindly advancing.
- On `user_cancel` (Ctrl-C), `aictl` stops the chain entirely and does not advance to the next Provider — cancellation is treated as an intentional halt.
- On `success`, `aictl` runs verification, updates the Session, and stops the chain.
- When the chain is exhausted, `aictl` reports "all providers exhausted" and leaves a complete Session for later `recover`.

#### FR-21: Record every Attempt

Each Run in a chain is recorded as an Attempt with Provider, Exit Reason, and pre/post Checkpoint references.

**Consequences (testable):**
- An attempts log lists each Attempt with its Provider and Exit Reason.
- Handoff/Task State never asserts "Provider X did Y" unless backed by git/file/verify evidence (no hallucinated progress).

## 5. Non-Goals (Explicit)

- **Not an AI agent.** `aictl` makes no model API calls and writes no code itself; it supervises other tools. Deterministic handoff is the deliberate consequence.
- **Not a replacement UI.** It will not reimplement or "improve" any Provider's terminal UI; the real CLI's UI is the UX.
- **Not a native-session translator.** It will not attempt to migrate Claude/Codex/Gemini internal conversation state between tools. It migrates *engineering state*, not provider sessions.
- **Not a remote/multi-user orchestrator.** No server, no shared sessions, no team sync in v1.
- **Not Windows-supported in v1.** ConPTY is a separate code path; deferred.
- **Not a cost/billing optimizer.** Cost-aware strategies (`--strategy cheapest/best`) are out of v1 (see §6.2).
- **Not dependent on any live Provider for its core function.** Any feature that would only work with credits available is, by definition, out of the core path.

## 6. MVP Scope

### 6.1 In Scope (v1.0)

- Session lifecycle + credit-free Task State commands: `init`, `start`, `note`, `done`, `next`, `fail` (FR-1…FR-4).
- PTY interactive runner with full passthrough, transcript capture, quiet supervision (FR-5…FR-7).
- Pre/post-run + on-demand git Checkpointing (FR-8…FR-10).
- Deterministic, LLM-free Handoff generation, including `handoff`/`prepare` and `recover`, with size bounding (FR-11…FR-14).
- Built-in Trio adapters + config-driven generic adapter + configurable prompt injection + verify commands (FR-15…FR-18).
- Usage-limit detection, `run --fallback` chain, Attempt recording (FR-19…FR-21).
- Platforms: macOS + Linux.

### 6.2 Out of Scope for MVP

- **LLM-powered summarization / context compaction** (`aictl summarize`) — deferred; deterministic Handoff is the v1 contract. *Revisit when long sessions strain Handoff size.* `[NOTE FOR PM]`
- **Cost-aware provider selection** (`--strategy cheapest/best`, priorities) — deferred to post-v1.
- **Windows / ConPTY support** — deferred.
- **Live in-Run file watcher** — v1 uses before/after git diff to detect changes; continuous watching during a Run is post-v1 (brief explicitly says before/after is enough for MVP).
- **Remote/multi-user/server mode** — out.
- **Rich transcript cleaning / structured parsing of provider tool-calls** — raw ANSI transcript only in v1.
- **Automatic capture of commands the Provider runs inside the PTY** — v1's command log records only `aictl`-invoked commands (notably Verify Commands, FR-18); parsing commands the Provider ran out of the transcript is post-v1. The Handoff's "recent commands" section is sourced from this `aictl`-invoked log.

## 7. Success Metrics

**Primary**
- **SM-1 — Zero-loss provider switch.** In a controlled test where Provider A hits a usage limit (instantly, and after partial progress), Provider B resumes with a Handoff that reflects the true current git state 100% of the time, with no manual re-explaining. Validates FR-8, FR-9, FR-11, FR-20.
- **SM-2 — Native UX fidelity.** Pass criterion is the enumerated checklist — rendering, raw-mode keybindings, approval prompts, and resize reflow — behaving the same under `aictl run` as under direct launch for each Trio Provider, verified across macOS + Linux. Validates FR-5, FR-7.
- **SM-3 — Maintainer dogfood retention.** Daniel uses `aictl` as the default way to run agentic CLIs and still uses it weekly one month after v1. Validates the whole product.

**Secondary**
- **SM-4 — Recover-without-provider.** A complete Handoff / `recover` prompt is produced with zero authenticated Providers, every time. Validates FR-11, FR-14.
- **SM-5 — Extensibility.** A new Provider can be added via Config alone (no code), demonstrated by at least one community/maintainer example beyond the Trio. Validates FR-16. *(OSS adoption signals — stars, external contributors — are watched but not targeted for v1.)* `[ASSUMPTION]`

**Counter-metrics (do not optimize)**
- **SM-C1 — Don't trade fidelity for cleverness.** Do not add Handoff "intelligence" (LLM summarization, aggressive truncation) that improves brevity at the cost of SM-1's faithfulness. Counterbalances any future SM on Handoff size/quality.
- **SM-C2 — Don't get chatty.** Do not add supervisor output/features that compromise SM-2 (TUI quietness). More status messaging is not better. Counterbalances SM-2 regressions disguised as "more informative."

## 8. Open Questions

1. **Prompt-injection reliability per Provider.** Do Claude Code / Codex / Gemini reliably accept a file-ref instruction at launch (arg vs. interactive paste)? Per-Provider verification needed; may change the default injection mode per adapter. (Affects FR-17.)
2. **Usage-limit pattern accuracy.** Real exhaustion phrasings drift over time and differ per Provider; what's the maintenance story for default patterns, and how do we bound false-positive risk mid-Run? (Affects FR-19.)
3. **"Files changed" granularity.** Is working-tree + staged git diff comparison sufficient, or are there edits outside git's view (untracked, ignored) that matter for handoff fidelity? (Affects FR-9, FR-20.)

*Resolved during drafting (see `.decision-log.md`): `recover` in MVP (yes), Session concurrency (one per working tree), distribution (`go install` + GitHub releases), Ctrl-C mid-chain (stop entirely).*

## 9. Assumptions Index

*Remaining unconfirmed inferences (the load-bearing scope/process calls were resolved with the user — see `.decision-log.md`):*

- **§2.1 / §2.3** — `aictl` is purely a supervisor (no model calls, no code edits of its own).
- **§3** — Goal is immutable for the Session's life.
- **§4.1 FR-1** — `init` is non-destructive on an existing Session Directory; adds ignore/keep entries to protect secrets and avoid committing transient artifacts.
- **§4.1 FR-2** — `start` either auto-inits or instructs the user; behavior is deterministic and documented.
- **§4.2 FR-6** — Transcript preserves raw ANSI for fidelity (per brief Milestone 2).
- **§4.5 FR-17** — Paste-mode fallback exists for Providers that cannot read files.
- **§7 SM-5** — OSS adoption metrics are watched, not v1 targets.
- **§Constraints/Privacy** — v1 secret handling = git-tracked-only diffs + configurable denylist (defaults TBD); deeper content scanning deferred.
- **§Constraints/Privacy** — `aictl` performs no telemetry/network calls of its own in v1.
- **§Developer-Product** — the Session Directory layout is treated as a secondary public surface (readable by users/scripts).
- **§Versioning** — v1.x follows semantic versioning; schema/layout breaks require a major bump + migration note.
- **§Runtime** — implemented in Go as a single static binary (confirmed in brief; specifics to architecture).
- **§Why Now** — the timing rationale is a light-touch inference, not researched market data.

---

## Cross-Cutting NFRs

*System-wide quality attributes not tied to one feature.*

- **NFR-1 — Provider-independence (reliability).** Every core capability (Session, Task State, Checkpoint, Handoff, `recover`) must function with zero authenticated/available Providers and no network. This is the product's defining reliability property; it gates SM-1/SM-4.
- **NFR-2 — TUI non-interference.** Supervision must not visibly degrade any Provider's interactive UI under normal terminal use (render, raw-mode keys, resize). Gates SM-2.
- **NFR-3 — Pre-run durability.** No Provider Run may begin before its pre-run Checkpoint and Handoff are persisted to disk. The ordering guarantee is the basis of instant-failure recovery.
- **NFR-4 — Deterministic, inspectable state.** All Session artifacts (Task State, Handoff, Checkpoints, attempts log, Config) are plain text, human-readable, hand-editable, and git-friendly.
- **NFR-5 — Observability without noise.** `aictl`'s own actions are recoverable after the fact via the attempts log / Session Directory, without printing during a Provider Run (reconciles with NFR-2).
- **NFR-6 — Startup overhead.** Pre-run preparation (Checkpoint + Handoff) should add launch latency well under ~1s on a typical repo (rough target; tighten during architecture). The dominant cost is `git diff`, bounded by `maxDiffChars` (FR-13). *See Performance Budgets.*

## Constraints & Guardrails

### Safety
- `aictl` must never destroy or overwrite uncommitted user work. Pre-run Checkpoints capture working state via patch/diff before any Provider runs; switching Providers must preserve, not discard, in-progress changes.
- Non-usage Provider failures stop and surface a diagnostic rather than silently advancing the Fallback Chain (FR-20).

### Privacy / Secrets
- **Secrets must never enter Handoff, transcript, or Checkpoints.** **v1 strategy:** generated diffs/Handoff include only git-tracked changes, so gitignored secret files (`.env`, keys) are naturally excluded; plus a **configurable denylist** of paths (e.g. `.env*`, `*.pem`) to suppress anything tracked-but-sensitive. Deeper content-scanning redaction (entropy/regex over diff bodies) is deferred to post-v1. `[ASSUMPTION: denylist defaults TBD]`
- Session artifacts are local and repo-resident; `aictl` performs no telemetry/network calls of its own in v1. `[ASSUMPTION]`
- `init` configures ignore rules so the Session Directory's transient/sensitive parts aren't accidentally committed (FR-1).

### Cost
- `aictl` itself incurs no model cost (no LLM calls). It does not manage or report Provider billing in v1; cost-aware selection is a Non-Goal (§5, §6.2).

## Developer-Product Concerns

### Public surface: the Config contract
- `config.yaml` is the primary public, user-facing contract (Provider Adapter schema: command, args, prompt-injection mode + text, usage-limit patterns; plus verify commands and handoff options). Changes to this schema are breaking changes and must follow the versioning policy below.
- The Session Directory layout (`.ai-session/` files and `checkpoints/` structure) is a secondary public surface — users and scripts may read it; document it as such. `[ASSUMPTION]`

### Versioning & deprecation
- v1.x follows semantic versioning. Breaking changes to the Config schema or Session Directory layout require a major bump and a documented migration note. `[ASSUMPTION]`

### Language / runtime targets & dependency policy
- Go, single static binary, no runtime dependencies for the user beyond the Provider CLIs they choose to run. `[ASSUMPTION — Go confirmed by brief; details to architecture]`
- Targets: macOS and Linux (Unix PTY). Windows/ConPTY deferred.
- Keep the dependency tree lean; PTY handling is the one essential native concern. *(Library choices live in `addendum.md`.)*

### Distribution
- v1 ships via **`go install`** (for users with a Go toolchain) **plus prebuilt macOS + Linux binaries attached to GitHub releases**. Homebrew tap deferred unless demand warrants the maintenance.

### Performance budgets
- Interactive passthrough must feel native: the tee/scan path adds no buffering or batching beyond a single pass-through write, so keystroke-to-render latency is indistinguishable from direct launch.
- Pre-run prep target ~1s on a typical repo (NFR-6), to be tightened during architecture; diff capture bounded by `maxDiffChars` (FR-13).

## Why Now

Agentic coding CLIs proliferated fast, and most developers now juggle several to stretch per-provider usage caps. Hitting a quota wall mid-task is a common, acute, recurring pain with no good cross-tool answer today — the tools deliberately don't interoperate. A thin, provider-agnostic supervisor that owns portable engineering state is buildable now precisely because these tools already solved the hard UI problem; `aictl` only has to wrap them and own the memory. `[ASSUMPTION: timing rationale — light-touch]`
