# Input Reconciliation — BRIEF.md vs prd.md + addendum.md

Checking the source brief for material dropped by the PRD's FR structure.

## Covered well
- Provider-agnostic supervisor thesis; orchestrator-as-source-of-truth; handoff-before-risky-call → Vision, NFR-3, FR-8, FR-11.
- Session recorder / minimum state → Task State, FR-1…FR-4.
- Pre-run + post-run checkpoints → FR-8/FR-9. On-demand checkpoint → FR-10.
- Provider adapters + interface + generic config → FR-15/FR-16; signatures in addendum.
- Deterministic template handoff (no LLM) → FR-11; template sections in addendum.
- `recover` safety net → FR-14, UJ-3.
- note/done/next/fail credit-free state → FR-3.
- Usage-limit detection + exit-reason enum + auto-fallback → FR-19/FR-20, Glossary.
- PTY runner (creack/pty, raw mode, SIGWINCH, tee/MultiWriter, "don't hijack UI") → 4.2, FR-7; mechanics in addendum.
- Prompt-injection strategies (arg/stdin/pty/file-ref, bracketed paste) → FR-17; detail in addendum.
- Verify-aware handoff → FR-18.
- Cost-aware selection, summarize/compaction, file watcher → correctly deferred in §6.2.
- Naming alternatives, taglines, build order/milestones → addendum.
- "Strict, file-specific, no hallucinated progress" handoff quality → FR-21 evidence rule + FR-11 anti-redo.

## Gaps surfaced
1. **Command-log population is undefined (MEDIUM).** The brief lists `command-log.md` and "command/test history" as a feature, and the PRD references a "command log" as a Checkpoint content (FR-8) and Handoff source (FR-11) — but no FR says *how* the command log gets populated. Needs an explicit scope statement (what aictl logs vs. what it cannot observe inside the PTY).
2. **Manual `switch` verb (LOW).** The brief's early UX used `aictl switch --provider claude`. The PRD folds manual switching into `aictl run <provider>` (FR-5) and automatic switching into `run --fallback` (FR-20). This is a deliberate simplification, not a loss — noting for traceability.
3. **`run <provider> --handoff` flag (LOW).** The brief showed `aictl run claude --handoff`. The PRD makes handoff generation automatic on every `run` (FR-12) and injection file-ref by default (FR-17), so the explicit flag is unnecessary. Deliberate; no action.
4. **Qualitative framing (NONE lost).** The "black-box flight recorder / out-of-fuel pilot" mental model is framing; its intent is preserved in the Vision and NFR-3 without the metaphor. Acceptable.

## Verdict
High fidelity. One real gap (command-log population) worth fixing; the rest are intentional, traceable simplifications.
