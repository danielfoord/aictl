# PRD Quality Review — aictl

## Overall verdict
A genuinely strong, coherent PRD with a clear thesis ("the orchestrator owns the session; the provider is a disposable worker; the handoff is generated before the risky call") that the features, NFRs, and success metrics all serve. Scope honesty and downstream usability are excellent for a chain-top dev-tool spec. What's at risk: one broken cross-reference, an undefined command-log population path, and a couple of performance NFRs that lean on adjectives rather than bounds. None are blockers; all are cheap fixes.

## Decision-readiness — strong
Decisions are stated as decisions, not buried: concurrency (one Session per working tree), distribution (`go install` + GitHub releases), Ctrl-C semantics, deterministic-only handoff, before/after-diff over a live file watcher. Trade-offs name what's given up (e.g. §6.2 file watcher: "before/after git diff is enough for MVP"). Open Questions (§8) are actually open — verification unknowns, not rhetorical. Counter-metrics (SM-C1/C2) guard the two real temptations (over-clever handoff, chatty supervisor). No findings.

## Substance over theater — strong
No persona theater (lightweight UJs, single real protagonist appropriate to a solo-built dev tool). Vision is product-specific and would not swap into another PRD. NFRs are mostly product-specific (NFR-1 provider-independence, NFR-3 pre-run ordering) rather than boilerplate. One soft spot noted under done-ness (perf adjectives). No standalone findings.

## Strategic coherence — strong
Clear thesis, and feature prioritization follows it: the foundational PTY runner (4.2) and pre-run durability (NFR-3/FR-8) come first because the thesis demands them. SMs validate the thesis directly — SM-1 (zero-loss switch) and SM-4 (recover with zero providers) test the central bet, not vanity activity. MVP scope kind is clearly "problem-solving," and the cut lines match. No findings.

## Done-ness clarity — adequate
Most FRs carry concrete testable consequences (FR-13 maxDiffChars marker, FR-3 no-network, FR-8 ordering, FR-9 changed-files boolean). Soft spots:

### Findings
- **medium** Perf NFRs lean on adjectives (§NFR-6, §Performance budgets) — "negligible latency," "no perceptible input latency," "budget to be set during architecture." An engineer can't test these. *Fix:* add a rough order-of-magnitude target now (e.g. pre-run prep well under ~1s on a typical repo; passthrough overhead imperceptible = no added buffering beyond the tee), to be tightened in architecture.
- **low** SM-2 / NFR-2 "behave identically" is hard to verify objectively. *Fix:* anchor to the enumerated checklist already present (render, raw-mode keys, approval prompts, resize) and call that the pass criterion.

## Scope honesty — strong
Non-Goals (§5) does real work (not-an-agent, not-a-UI, not-a-translator, not-remote, not-cost-optimizer). Assumptions are tagged inline and indexed; `[NOTE FOR PM]` sits on the one emotionally load-bearing deferral (summarize). Open-items density is low and appropriate for the stakes. One mechanical roundtrip gap noted below.

### Findings
- **low** Command-log scope unstated (§6.2) — the PRD consumes a "command log" (FR-8, FR-11) but never bounds what populates it. *Fix:* add an explicit out-of-scope line: v1 logs only aictl-invoked commands (e.g. verify); parsing provider-run commands from the transcript is post-v1. (Also raised in reconcile-brief.md.)

## Downstream usability — strong
This is a chain-top PRD (feeds architecture → epics), so this dimension matters most, and it largely delivers: Glossary present and used consistently; FR 1–21, UJ 1–5, SM 1–5/C1–C2, NFR 1–6 all contiguous and unique; sections extract cleanly. One broken cross-ref:

### Findings
- **medium** Broken cross-reference (§FR-9) — "the system can reuse the pre-run Handoff for the next Provider (see FR-15)" points to *Built-in Trio adapters*; the reuse logic lives in FR-20. *Fix:* change "(see FR-15)" → "(see FR-20)".

## Shape fit — strong
Correctly shaped as a capability spec with lightweight named-protagonist UJs — right for a single-operator CLI dev tool. Not over-formalized (no UJ bloat) and not under-formalized (UJs still present where the cross-provider flow needs narrating). Dev-product adapt-ins (Config-as-contract, versioning, runtime targets, distribution) are the right clusters. No findings.

## Mechanical notes
- **Broken cross-ref:** FR-9 "(see FR-15)" should be "(see FR-20)". *(also in Downstream findings)*
- **Assumptions Index roundtrip incomplete:** inline `[ASSUMPTION]` tags in the adapt-in sections are not all indexed — §Privacy "no telemetry," §Dev-Product "Session Directory layout is a secondary public surface," §Versioning semver, §Runtime Go, §Why Now timing. *Fix:* add them to §9 or drop the tags.
- **ID continuity:** clean (no gaps/dupes in FR/UJ/SM/NFR).
- **UJ protagonists:** UJ-1–4 named (Daniel); UJ-5 is a cross-cutting continuation — acceptable.
- **Glossary drift:** none detected; capitalized domain nouns used consistently.

## Grade
**Good** — all dimensions strong/adequate, no high/critical findings; two medium + minor mechanical fixes stand between this and Excellent.
