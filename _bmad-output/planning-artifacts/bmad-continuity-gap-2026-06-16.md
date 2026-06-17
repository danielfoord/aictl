# Architecture note — BMad task-continuity gap

- **Date:** 2026-06-16
- **Status:** Open / proposed (backlog story `3-7-bmad-task-continuity-bridge`)
- **Related:** Story 3.6 (smart-default injection), Epic 4 (auto-fallback), sprint-change-proposal-2026-06-16

## The gap

For a **BMad-driven** workflow — `aictl run claude`, then you type `/dev-story` or
`/code-review` inside the provider — running out of tokens mid-task does **not**
currently hand the *task* over to the next provider. Two concrete reasons:

1. **The task memory is never captured.** aictl's durable memory
   (`state.yaml`: goal, plan, decisions, next steps) is only populated by
   `aictl start`/`note`/`next`/`fail`, or by a provider instructed to write it.
   In the drive-it-yourself flow none of that happens — the BMad slash command
   doesn't know aictl exists. So the regenerated handoff contains only **git
   state** (the code that changed), with no goal/plan. And per Story 3.6's
   smart default, an empty task state means the next provider gets **no injection
   at all**.
2. **Auto-fallback is unbuilt.** Usage-limit detection + chain advance is **Epic 4**
   (`4-1`…`4-4`, backlog). aictl does not notice a quota hit; switching is manual.

**What survives a handover today:** the git diff/status/commits (real source of
truth for code), the raw transcript, and pre/post checkpoints.
**What is lost:** the goal, the plan, "which story," decisions, and next steps.

## The key insight

For BMad, **BMad already is the durable task memory** — the story file
(`_bmad-output/implementation-artifacts/<key>.md`), `sprint-status.yaml`, and
related artifacts are committed to the repo. So aictl's goal/handoff layer
*overlaps* with what BMad persists; that overlap is exactly why 3.6 has aictl get
out of the way (no injection) on a fresh, undriven session.

The continuity fix is therefore not "make the user retype the goal into aictl" —
it is to **bridge** aictl's task state from the artifacts BMad already maintains,
so the handoff (and 3.6 injection) carry real context with zero extra typing.

> BMad is only the **motivating example**. The fix must be **tool-agnostic** —
> it has to work for any skill/tool/workflow (Linear, Jira, a plain `TASK.md`,
> a custom CLI), with **zero tool-specific code in aictl core**.

## Two pieces needed

| Piece | Status | Note |
|---|---|---|
| Usage-limit detection + auto-fallback on quota | Epic 4 backlog (`4-1`,`4-2`) | the "automatic switch" half |
| **Configurable goal/context source** | NEW — `3-7-configurable-goal-source` (backlog) | the "carry real context" half, generic |

## Design (tool-agnostic): a configurable goal source

aictl already shells out to user-configured commands (`git`, providers, the
`verify` list) and stays agnostic. The continuity fix is the same pattern: a
**user-configured source** that aictl reads to populate the goal when the session
has none — aictl ships **no** knowledge of BMad or any other tool.

`config.yaml` (new, optional):

```yaml
# Where aictl derives the task goal when state.yaml has none. Tool-agnostic:
# point it at any file or command. Leave unset to keep today's manual behavior.
goalSource:
  file: TASK.md                       # read the goal from a file, OR
  command: ["mytool", "current-goal"] # run a command; use its stdout
```

- aictl reads `file` (or runs `command`) and uses the result as the goal. That's
  the entire contract — no tool coupling.
- **BMad** users set `command:` to a tiny script that reads `sprint-status.yaml` +
  the active story file. **Linear/Jira** users point at their CLI. **Anyone** can
  use `file: TASK.md`. The integration lives in *their* config, never in aictl.
- Mirrors `verify` (user-configured commands aictl runs) and respects the no-core-
  coupling rule.

**Open design choices (to confirm before building):**
1. **Trigger:** populate the goal automatically in `run` when it's empty (zero
   friction, but runs the configured command pre-launch, like `verify`), **or**
   keep it explicit (`aictl start` with no arg reads the source / a dedicated
   `aictl sync` command).
2. **Scope:** just the goal, or richer context (e.g. also next-steps) from the
   source.
3. **Security:** `command` runs an arbitrary user-configured process — fine (same
   trust model as `verify`/providers), but document it.

Rejected: a BMad-specific parser / `aictl bmad-goal` command in core (couples aictl
to one tool). A `goalSource: { command: [...] }` makes BMad just one of many
configurations.

### Interim manual answer (works today, no new code)

Run `aictl start "<goal>"` (or a couple of `aictl next`) before/while you work;
that populates `state.yaml`, so the handoff is meaningful and 3.6 injects on the
next provider. The configurable goal source just removes the retyping — generically.

## Tracking

- Story `3-7-configurable-goal-source` (Epic 3).
- Epic 4 (`4-1`/`4-2`) remains the other half (auto-switch on quota) and should
  reuse `TaskState.InProgress()` for the "continue" decision.

## Implemented (2026-06-17)

The explicit half is built (tool-agnostic, no spec ceremony — design settled here):

- `config.GoalSource{File, Command}` on `config.yaml`.
- `internal/goalsource` — `Read` (file or `sh -c` command) + `Parse` (structured
  YAML `goal`/`nextSteps`/`decisions`/`knownFailures`, else plain text → goal).
- `App.Sync` + `aictl sync` — populate the goal (and structured context) from the
  source when the session has none; never overwrites an existing goal.
- Verified end-to-end (`aictl init` → configure `goalSource.command` → `aictl sync`
  sets goal + next steps). Full suite / `-race` / lint / Windows build green.

**Deferred (per the "both" trigger choice):** auto-populate in `run` when the goal
is empty, behind a config flag. **Not yet done:** README/docs for `aictl sync` +
`goalSource`; a formal code review of this change.
