# Sprint Change Proposal — Smart-default handoff injection

- **Date:** 2026-06-16
- **Author:** Daniel (via correct-course)
- **Status:** Proposed
- **Scope classification:** Minor–Moderate (one new story in Epic 3 + minor PRD/architecture/README clarifications)

## 1. Issue Summary

`aictl run <provider>` **always** injects the file-reference handoff prompt
("Read `.ai-session/handoff.md` and continue the in-progress task…") as the
provider's opening prompt. This was introduced in **Story 3.3 (FR-17)** as the
default injection behavior.

The problem surfaced in real use: when the developer intends to **drive the
provider themselves** — e.g. launching Claude Code and running a BMad slash
command like `/dev-story` or `/code-review` — the injected prompt *hijacks* the
session. The provider opens by reading a (often goal-less) handoff and "continuing"
instead of waiting for the user's command. In these workflows the **goal emerges
in-session** (or already lives in a story file); it is not framed up front via
`aictl start`.

**Evidence (code):** `internal/app/run.go` `App.Run` → `resolveLaunch` →
`providers.Provider.Inject` (`internal/providers/injection.go`) unconditionally
appends the prompt as the provider's argv (file-ref/arg) or types it into the PTY
(stdin/paste). There is no condition on whether a task is actually in progress.

What already works (not the blocker): `run` is goal-optional and does **not** take
the session lock, so `aictl run <provider>` then `aictl start "<goal>"` already
lets you capture the goal *after* a run. The forced injection is the real blocker.

## 2. Impact Analysis

### Epic impact
- **Epic 3 (Supervised Provider Runs):** the behavior to change was delivered by
  Story 3.3. Epic 3's stories 3.1–3.5 are `done`; rather than reopen 3.3, add a
  **new Story 3.6** that refines the injection decision. Epic 3 remains coherent.
- **Epic 4 (Automatic Fallback):** a fallback advance (FR-20) is by definition a
  *continue* run — a provider already ran and progress/goal exist — so injection
  should occur there. The smart-default rule is compatible; **Story 4.2 must reuse
  the same injection-decision predicate** (note carried forward, no rework now).
- No epics become obsolete; no new epic required; no resequencing.

### Artifact conflicts
- **PRD FR-17** (`prd.md`): the "Consequences (testable)" imply injection happens
  on every launch. Needs a one-line clarification: injection happens when the
  session has a task to **continue**; a fresh run with no goal/progress launches
  the provider clean. *(Modify, not remove.)*
- **Architecture** (`architecture.md`, "API & Communication — Child process I/O"
  / "Provider adapter contract"): add a note that *whether* to inject is decided
  by the run loop from session progress; the per-provider mode still decides *how*.
- **UX:** N/A — no UX document (CLI passthrough).
- **Other:** `README.md` Usage wording ("tells the provider to read it and
  continue") should reflect the smart default; `internal/app`/`internal/providers`
  tests gain cases for inject-vs-skip. No CI/lint/deploy impact.

### Technical impact
- Narrow: `App.Run` gates the prompt portion of the launch on a session-progress
  predicate; it **still** generates `handoff.md` and the pre/post checkpoints
  (durability/NFR-3 unchanged). Per-provider injection mode unchanged. No new deps.

## 3. Recommended Approach

**Option 1 — Direct Adjustment (selected).** Add **Story 3.6** to Epic 3
implementing smart-default injection, plus minor PRD/architecture/README
clarifications. Effort **Low**, risk **Low**, timeline impact negligible. Rollback
(Option 2) is unnecessary — Story 3.3's registry/injection machinery is correct and
reused as-is. MVP review (Option 3) is unnecessary — MVP scope is intact; FR-17 is
clarified, not reduced.

### The smart-default rule (design intent for Story 3.6)

> `aictl run` injects the handoff prompt **only when the session has something to
> continue.** Inject when the Task State has a non-empty **Goal** *or* any recorded
> **progress** (decisions, completed steps, next steps, known failures). Otherwise
> (fresh session — no goal, no progress) **skip injection**: launch the provider
> clean so the user drives it (e.g. `/dev-story`).

- The predicate is computed from **Task State only** (not "does `handoff.md`
  exist" — `run` rewrites that every time, which would wrongly force injection on
  every subsequent run).
- Composes with existing behavior: after a fresh run you can `aictl start "<goal>"`
  (or record progress); the **next** run then injects and continues.
- The per-provider injection **mode** (file-ref/arg/stdin/paste) still decides
  *how* to inject when injection does happen; the smart default decides *whether*.
- Handoff generation + pre/post checkpoints still happen on every run (durability
  is unchanged); only the prompt delivery is gated.
- Out of scope (note for later, not this story): an explicit per-run override
  (`--inject`/`--no-inject`) or a `none` injection mode. Daniel chose the implicit
  smart default; an explicit escape hatch can be added later if needed.

## 4. Detailed Change Proposals

### 4.1 New Story (Epics / sprint-status)

Add **Story 3.6: Smart-default handoff injection** under Epic 3.

```
Story 3.6: Smart-default handoff injection
As a developer,
I want `aictl run` to inject the handoff prompt only when there's a task to continue,
so that I can launch a provider and drive it myself (e.g. BMad /dev-story) without
the supervisor hijacking the session, while still getting auto-continue when I'm
resuming in-progress work.

Acceptance Criteria:
1. A run on a session with no goal AND no recorded progress launches the provider
   with NO injected prompt (clean drive) — supervision (transcript, pre/post
   checkpoints, exit code) still applies.
2. A run on a session with a goal OR any task-state progress injects the handoff
   per the provider's configured mode (unchanged from Story 3.3).
3. The inject/skip decision is computed from Task State only; handoff.md is still
   generated and the pre/post checkpoints still written on every run.
4. Unknown (bare-executable) providers continue to receive no injection.
5. Tests cover: fresh-session skip, goal-present inject, progress-present inject,
   and that handoff/checkpoints are written in both cases.
```

sprint-status.yaml: add `3-6-smart-default-handoff-injection: backlog` under Epic 3
(before `epic-3-retrospective`). Epic 3 stays `in-progress`.

### 4.2 PRD — FR-17 clarification

```
Story: PRD FR-17 (Prompt injection)
Section: Consequences (testable)

ADD a consequence:
- aictl injects the handoff prompt only when the Session has a task to continue
  (a recorded Goal or task-state progress). A fresh run with no goal/progress
  launches the Provider without an injected prompt so the user can drive it.

Rationale: the configurable injection mode (file-ref/arg/stdin/paste) decides HOW
to inject; this clarifies WHEN, so supervisor-driven and user-driven (e.g. BMad
slash-command) workflows both work.
```

### 4.3 Architecture — note

```
Document: architecture.md
Section: API & Communication — Child process I/O (PTY runner) / Provider adapter contract

ADD: "Whether to inject the handoff is decided by the run loop from Task State
(goal/progress); the per-provider mode decides how. Fallback advances (Epic 4) are
'continue' runs and therefore inject."
```

### 4.4 README — wording

```
File: README.md (Usage → How a session is meant to work, step 3)
Adjust the line about the run injecting "read the handoff and continue" to note it
applies when resuming in-progress work; a fresh run lets you drive the provider.
```

## 5. Implementation Handoff

- **Scope:** Minor–Moderate. One new story (backlog), plus small clarifications to
  PRD/architecture/README.
- **Route to:**
  - **create-story → dev-story (Developer)** for Story 3.6 implementation + tests.
  - The PRD/architecture/README edits can ride with Story 3.6 (documentation tasks)
    or be applied directly — they are wording clarifications, not new requirements.
- **Carry-forward:** Story 4.2 (fallback) must reuse the same inject/skip predicate.
- **Success criteria:** the Story 3.6 ACs above pass; `aictl run claude` on a fresh
  session opens clean (no injected prompt) and BMad `/dev-story` works; resuming a
  session with a goal still auto-injects; all existing run/checkpoint tests green;
  `go test ./...`, `-race`, vet, lint, and the Windows cross-build stay green.
```
