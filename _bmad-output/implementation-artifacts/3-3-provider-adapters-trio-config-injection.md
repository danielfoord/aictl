---
baseline_commit: ce74d56
context:
  - _bmad-output/implementation-artifacts/3-2-faithful-transcript-and-quiet-supervision.md
  - _bmad-output/implementation-artifacts/3-1-run-provider-in-pty.md
---

# Story 3.3: Provider adapters — Trio, config-driven, and injection

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As a developer,
I want built-in adapters for Claude/Codex/Gemini plus the ability to define my own provider entirely in config, with the handoff injected appropriately per provider,
so that I can run `aictl run <name>` for any supported or self-defined CLI and have it pick up the in-progress task without me re-explaining it.

## Acceptance Criteria

1. **Built-in Trio with sensible defaults:** given an installed, authenticated Trio provider, when I run `aictl run claude|codex|gemini`, it launches the right executable (default command + default args) inside the PTY with its native UI, and the provider carries default usage-limit patterns. (FR-15)
2. **Config-defined providers, no code change:** a provider defined only in `config.yaml` (command, args, injection mode + text, usage-limit patterns) is runnable via `aictl run <name>`, resolved through the **same** registry as the built-ins. A config entry for a built-in name overrides that built-in's fields; a new name adds a new provider. Code must never branch on "is this built-in vs config" — resolution goes through one overlaid map. (FR-16)
3. **Default file-reference injection:** when a provider is launched with the default injection mode, `aictl` injects a short instruction referencing the handoff file (e.g. "Read `.ai-session/handoff.md` and continue the in-progress task; do not restart from scratch"), using the path from `session.Paths.Handoff()`. (FR-17)
4. **Configurable injection mode per provider:** the injection mode is configurable per provider as one of `file-ref` (default) / `arg` / `stdin` / `paste`. `file-ref` and `arg` deliver the prompt as a command-line argument; `stdin`/`paste` deliver it by writing into the provider's PTY after launch (the paste fallback for providers that cannot take a prompt argument). (FR-17)
5. **Missing executable still fails clearly, before raw mode:** if a resolved provider's command is not on `PATH`, `aictl run` fails before entering raw mode / starting the PTY, with a clear error and no provider side effects — preserving Story 3.1 behavior. (FR-5 / Story 3.1 AC6 carried forward)
6. **No later-story scope:** this story does not implement pre-run handoff generation, pre/post checkpoints or durability ordering (Story 3.4), usage-limit **detection**/classification or the fallback chain (Epic 4), or the attempts log. Built-in usage-limit patterns are defined here as provider **data** only; nothing scans output for them yet. (Scope boundary)

## Tasks / Subtasks

- [x] **Task 1 — Provider config surface (AC: 2, 3, 4)**
  - [x] Extend `internal/config/config.go` `Provider` with a `PromptInjection` field: `PromptInjection PromptInjection` carrying `Mode string` (`yaml:"mode"`) and `Text string` (`yaml:"text"`). Use explicit camelCase yaml tags (`yaml:"promptInjection"`, `mode`, `text`) per the config-contract convention.
  - [x] Keep `config.Default()` Providers map **empty** — built-in providers live in the registry (code), not in the default config file. Do not seed the Trio into `config.yaml`.
  - [x] Update `internal/config/config_test.go` for the new field (round-trip marshal/unmarshal of a provider with a `promptInjection`).
- [x] **Task 2 — `internal/providers` package: resolved provider + registry overlay (AC: 1, 2)**
  - [x] Create `internal/providers/provider.go`: a resolved `Provider` value type `{ Name, Command string; Args []string; Mode Mode; PromptText string; UsageLimitPatterns []string }` and a `Mode` string type with constants `ModeFileRef="file-ref"`, `ModeArg="arg"`, `ModeStdin="stdin"`, `ModePaste="paste"`.
  - [x] Create `internal/providers/claude.go`, `codex.go`, `gemini.go`: built-in adapters returning the default resolved `Provider` for each (command = `claude`/`codex`/`gemini`, no default args, `Mode=ModeFileRef`, default file-ref `PromptText`, and default `UsageLimitPatterns`). Suggested patterns (from the brief's example config): claude `["usage limit","try again"]`, codex `["usage limit","quota exceeded","try again in","billing"]`, gemini `["quota","rate limit"]`.
  - [x] Create `internal/providers/registry.go`: a `Resolve(cfg config.Config) map[string]Provider` that starts from the built-in Trio and overlays `cfg.Providers` — a config entry **replaces/extends** matching fields of a built-in by name and adds brand-new providers. Provide `Lookup(resolved map[string]Provider, name string) (Provider, bool)`. The dependency direction is `providers → config` only (config must not import providers).
  - [x] Overlay semantics: a config `Provider` with empty `Command` for an existing built-in keeps the built-in command; non-empty fields override; `usageLimitPatterns`/`args` from config replace the built-in's when provided. Document the merge rule in code.
- [x] **Task 3 — Injection builder (AC: 3, 4)**
  - [x] Create `internal/providers/injection.go` with a method, e.g. `func (p Provider) Inject(handoffPath string, userArgs []string) Injection`, returning `Injection{ Args []string; InitialInput []byte }`.
  - [x] Resolve the effective prompt text: if `p.PromptText` is empty, use the default file-ref text referencing `handoffPath`. For `file-ref`, always reference the handoff file; for `arg`, use the configured `Text` (fall back to the file-ref text if empty).
  - [x] `file-ref` / `arg`: `Args = append(append(p.Args, userArgs...), promptText)` (prompt as the final positional argument); `InitialInput = nil`.
  - [x] `stdin` / `paste`: `Args = append(p.Args, userArgs...)` (no prompt arg); `InitialInput = []byte(promptText + "\r")` to be written into the PTY after launch. Consider bracketed-paste framing (`ESC[200~ … ESC[201~`) for multi-line text — see Open Questions.
  - [x] Unknown/empty mode defaults to `file-ref`.
- [x] **Task 4 — Runner support for post-launch PTY input (AC: 4)**
  - [x] Add `InitialInput []byte` to `shell.Options`. When non-empty, the runner writes it into the PTY master shortly after the child starts (a small fixed delay so TUIs finish initializing — make the delay a named const), then continues normal stdin passthrough. When empty, behavior is exactly Story 3.1/3.2 (no change).
  - [x] The initial-input write must not block or corrupt the existing stdin→ptmx and ptmx→fan-out copy loops, must respect context cancellation, and must not break raw-mode restore or exit-code mapping. Keep it inside `internal/shell` (process boundary).
  - [x] Tolerate write errors on the initial input without aborting the run (best-effort injection; the user can still type).
- [x] **Task 5 — Wire registry + injection into `App.Run` (AC: 1, 2, 3, 4, 5)**
  - [x] In `internal/app/run.go`, resolve the repo root (`os.Getwd`), load config via `config.Load(paths.Config())` (a missing config yields defaults, so a config-less repo still runs the built-in Trio), and build the resolved registry with `providers.Resolve(cfg)`.
  - [x] Resolve the requested name via `providers.Lookup`. If found: use its `Command`, build `Inject(paths.Handoff(), userArgs)`. If **not** found: fall back to treating the name as a bare executable with no injection (preserves Story 3.1 "run any executable"; see Open Questions / decision).
  - [x] Run `exec.LookPath(resolvedCommand)` and fail before the PTY if missing (AC5 / Story 3.1 AC6). Pass the resolved command, injected args, and `InitialInput` into `shell.Options`, preserving the Story 3.2 transcript wiring, `Mute`/`defer Flush`, and exit-code propagation untouched.
  - [x] Update the pre-run UI line if useful (still concise, NFR-5): e.g. "Launching <name>".
- [x] **Task 6 — `cmd/aictl/run.go` (AC: 1, 2)**
  - [x] Keep the Cobra command thin: `run <provider> [-- provider-args...]` already delegates to `App.Run(ctx, args[0], args[1:])`. Confirm `--` passes provider args through (Story 3.1 already covers this). No business logic in `cmd/`.
- [x] **Task 7 — Test coverage and verification (AC: 1–6)**
  - [x] `internal/providers` unit tests: built-in Trio defaults (command, mode, patterns); `Resolve` overlay (config overrides a built-in field; config adds a new provider; built-in untouched fields persist); `Lookup` hit/miss.
  - [x] `internal/providers` injection tests: `file-ref`/`arg` put the prompt as the final arg and set no `InitialInput`; `stdin`/`paste` set `InitialInput` and no prompt arg; empty `PromptText` falls back to the file-ref default referencing the handoff path; unknown mode → file-ref.
  - [x] `internal/shell` test: `Options.InitialInput` is written into the PTY after launch (use the fake helper child that echoes stdin, assert the injected bytes appear in the transcript/output); empty `InitialInput` preserves prior behavior; an initial-input write error does not fail the run.
  - [x] `internal/app` tests: a built-in name resolves to its command + file-ref injection (assert the `shell.Options` the stubbed `runProvider` receives); a config-defined provider resolves through the registry; an unknown name falls back to bare execution; a resolved-but-missing command fails before the runner (no transcript/provider side effects). Use the existing `a.runProvider` stub seam and `t.Chdir(t.TempDir())` (see Previous Story Intelligence — the transcript writes to `.ai-session/`).
  - [x] `internal/config` test: round-trip a provider with `promptInjection: { mode: ..., text: ... }`.
  - [x] Run `go test ./...`, `go vet ./...`, `go build ./...`, **`go test -race ./internal/shell ./internal/app ./internal/providers ./internal/config ./cmd/aictl`**, and `GOOS=windows GOARCH=amd64 go build ./...`. Run `golangci-lint run` if installed. Confirm no networking imports are added (NFR-1 CI import-check).

## Dev Notes

**Third story of Epic 3.** Builds on the Story 3.1 PTY runner and the Story 3.2 transcript/quiet-supervision wiring. This story replaces `App.Run`'s raw `exec.LookPath(provider)` with a **registry-resolved provider** (built-in Trio overlaid by config) and adds **prompt injection** so the launched provider is told to read the handoff and continue.

> **Baseline:** Story 3.2 is committed at `ce74d56`; that is this story's clean diff baseline.

### Scope Boundary

- **In scope:** `internal/providers` (resolved provider type, built-in Trio adapters, config-overlay registry, injection builder); a `PromptInjection {Mode, Text}` field on `config.Provider`; a `shell.Options.InitialInput` for paste/stdin delivery; rewiring `App.Run` to resolve + inject; built-in usage-limit patterns **as data** on each provider.
- **Out of scope:** pre-run handoff **generation** and durability ordering, pre/post checkpoints, per-Attempt directories (all Story 3.4); usage-limit **detection**/stream scanning, `classify(...)`, the fallback chain, the attempts log (all Epic 4); secret scanning/redaction of injected prompt text or transcript bytes (deferred post-v1). The file-ref injection references `.ai-session/handoff.md` even though its pre-run generation is wired in Story 3.4 — that is expected.
- **Do not make `aictl` an alternate UI.** Keep the provider's native TUI. Injection that types into the PTY (paste/stdin) must not add visible wrapper chrome; pre/post lines stay concise (NFR-2/NFR-5).

### Reuse / Do Not Reinvent

- **Resolve providers through one overlaid map; never branch on built-in vs config** (Architecture Communication Patterns). Built-in registry is code; `config.Provider` overlays it at load time into one resolved map; lookups use only that map.
- **Reuse `config.Load(paths.Config())`** exactly as `handoff.go`/`recover.go`/`verify.go` do (`root := os.Getwd()` → `session.NewPaths(root)` → `config.Load(paths.Config())`). A missing config returns `config.Default()`, so the built-in Trio still runs without a config file.
- **Reuse `session.Paths.Handoff()`** for the handoff path in the file-ref text — never hand-build `.ai-session/handoff.md`.
- **Extend the Story 3.1/3.2 runner; do not fork it.** Add `InitialInput` to the existing `shell.Options`; keep `enterRawMode`, `watchTerminalRestore`, `startResizeWatcher`, the fan-out writer, transcript, exit-code mapping, and the `outputDone`/`ptmx.Close`/`<-outputDone` choreography intact.
- **Dependency direction:** `providers → config` (one way). `internal/shell` must not import `providers`/`config`/`session`/`app`. `app` imports `providers` + `config` + `session` + `shell`. Feature packages never import `app`.
- **No new dependencies, no network imports** (NFR-1). Injection is stdlib + existing `goccy/go-yaml`.

### Expected Implementation Shape

Architecture names these files under `internal/providers/`: `provider.go` (interface/registry), `registry.go` (built-in + overlay → resolved map), `claude.go`/`codex.go`/`gemini.go` (built-in adapters), `injection.go` (file-ref/arg/stdin/paste). This story realizes them as a **resolved value type** rather than an interface (simpler, testable, no behavior beyond data + an `Inject` method) — acceptable given the architecture's stated latitude ("Suggested contracts can evolve; keep them testable; prefer DI over global state").

```go
// internal/providers/provider.go
type Mode string
const ( ModeFileRef Mode = "file-ref"; ModeArg Mode = "arg"; ModeStdin Mode = "stdin"; ModePaste Mode = "paste" )

type Provider struct {
    Name               string
    Command            string
    Args               []string
    Mode               Mode
    PromptText         string   // empty ⇒ default file-ref text at injection time
    UsageLimitPatterns []string // data only this story (detection is Epic 4)
}

// internal/providers/registry.go
func Resolve(cfg config.Config) map[string]Provider // built-in Trio overlaid by cfg.Providers
func Lookup(resolved map[string]Provider, name string) (Provider, bool)

// internal/providers/injection.go
type Injection struct { Args []string; InitialInput []byte }
func (p Provider) Inject(handoffPath string, userArgs []string) Injection
```

```go
// internal/shell/runner.go — Options gains:
InitialInput []byte // written into the PTY a short delay after launch (paste/stdin); nil ⇒ no change
```

### Current Files to Modify (read in full before editing)

- `internal/config/config.go` — `Config`/`Provider`/`Handoff` types + `Load`/`normalized`. **What changes:** add `PromptInjection` to `Provider` (the type comment already says it is "expanded in later stories (prompt-injection modes)"). **Preserve:** `Load` returning `Default()` on a missing file, and `normalized()` guardrails (denylist/maxDiffChars) — do not let the new field disturb them.
- `internal/config/defaults.go` — `Default()` returns empty Providers. **Preserve** that (Trio lives in the registry, not the default file).
- `internal/app/run.go` — current `App.Run` does `exec.LookPath(provider)` then runs with the user args, plus the Story 3.2 transcript + `Mute`/`defer Flush` + exit-code logic. **What changes:** load config, resolve via registry, build injection, resolve+LookPath the *resolved* command, pass `Args`+`InitialInput`. **Preserve:** the transcript open/close, `Mute()`+`defer a.UI.Flush()`+inline `Flush()`, `ExitError` propagation, and "fail before side effects when the executable is missing" ordering (do LookPath of the resolved command before `openTranscript`, mirroring 3.1/3.2 where LookPath precedes side effects).
- `internal/shell/runner.go` — Story 3.1/3.2 runner. **What changes:** add `Options.InitialInput` and a post-start PTY write. **Preserve:** all existing goroutine/close/restore/exit-code behavior; the new write must be additive and best-effort.
- `cmd/aictl/run.go` — already thin and already forwards `args[1:]` after `--`; likely no change.

### Edge Cases That Must Not Be Missed

- **Config provider overriding a built-in:** e.g. `config.yaml` sets `claude: { command: claude-canary }` — the resolved `claude` must use `claude-canary` but keep other built-in defaults unless also overridden. Test this.
- **Unknown provider name:** decide and implement deterministically. This story's decision: fall back to bare executable (no injection), preserving Story 3.1's "run any executable" and its tests. (Flagged in Open Questions — confirm before/with implementation.)
- **Missing resolved command:** `exec.LookPath` of the resolved command fails → clear error before raw mode, no transcript/provider side effect (AC5). Order LookPath before `openTranscript`.
- **Empty `PromptText`:** must fall back to the default file-ref text referencing the actual handoff path; never inject an empty prompt arg.
- **`paste`/`stdin` initial input vs keyboard passthrough:** the injected bytes and the user's keystrokes both flow to the PTY master; the injection writes once after a short delay, then normal passthrough resumes. Ensure no deadlock with the existing `io.Copy(ptmx, stdin)` goroutine, and that the injected write respects ctx cancellation.
- **Provider that takes no prompt arg:** `paste` is the documented fallback; make sure file-ref/arg providers and paste/stdin providers are both launchable and tested.
- **Config with a provider but empty mode:** defaults to `file-ref`.
- **Secrets:** the handoff (referenced by file-ref) is git-tracked-diff-redacted upstream (Story 2.1); this story injects only a *reference* by default, not handoff contents — keep it that way for the default. Do not log injected text to the TTY during the run (NFR-2).

### Testing Guidance

- Prefer table-driven tests for `Resolve` overlay and `Inject` modes.
- Reuse the Story 3.1 `TestHelperProcess` fake-CLI pattern for the runner `InitialInput` test (echo stdin → assert injected bytes land in the captured transcript). Keep PTY tests platform-aware (macOS/Linux) and **race-clean** (Story 3.1's review found a `-race`-only test race; Story 3.2 added concurrent paths — any new goroutine assertions must use atomics/channels).
- App-level tests assert the `shell.Options` handed to the stubbed `a.runProvider` (Command, Args, InitialInput) rather than launching real CLIs. Remember `App.Run` now writes a transcript under `.ai-session/`, so **every app test that calls `Run` must `t.Chdir(t.TempDir())`** (the Story 3.2 review caught a leaked `.ai-session/` from a test that didn't).
- Do not depend on real `claude`/`codex`/`gemini` binaries being installed.

### Previous Story Intelligence (Stories 3.1 + 3.2)

- **Runner API:** `shell.Run(ctx, shell.Options{Command, Args, Dir, Env, Stdin, Stdout, Stderr, Transcript, /* add InitialInput */})` → `Result{ExitCode}`. `New(u)` injects `runProvider = shell.Run`; tests swap `a.runProvider`.
- **`App.Run` already does (3.2):** `openTranscript()` (creates `.ai-session/` lazily, writes `.gitignore` if absent, opens `transcript.ansi` `0o600`), `a.UI.Mute()` + `defer a.UI.Flush()` + inline `Flush()`, and `ExitError{Code}` → `main` exits with the provider's code. Keep all of this; just change how Command/Args/InitialInput are derived.
- **Quiet supervision is structural** — never print during the run; the `defer a.UI.Flush()` makes it panic-safe. Don't add mid-run output for injection.
- **Test hygiene:** `t.Chdir(t.TempDir())` for any `App.Run` test (transcript side effect). The cmd-level `run` test learned this the hard way.
- **Review standards applied to this epic:** fail before side effects when the executable is missing (3.1 AC6); restore terminal on all paths (3.1); `-race` is part of the gate; conventional one-feature-per-story commits.
- **Config loading pattern** is established in `handoff.go`/`recover.go`/`verify.go`: `os.Getwd` → `session.NewPaths` → `config.Load(paths.Config())`, with `config.Default()` on absence.

### Latest Technical Information

- No new dependencies. `github.com/creack/pty v1.1.24`, `golang.org/x/term v0.44.0`, `github.com/spf13/cobra v1.10.2`, `github.com/goccy/go-yaml` are sufficient. Adding any networking import breaks the NFR-1 CI import-check.
- Built-in provider command names are `claude`, `codex`, `gemini` (run from PATH). Default usage-limit patterns are seeded from the brief's example config (addendum) and remain user-overridable per provider.
- The PTY paste/bracketed-paste reliability is a known PRD open question — treat the post-launch write delay and bracketed-paste framing as tunable, and keep `file-ref` (no PTY write) the robust default for the Trio.
- Go toolchain stays `go 1.26.0`.

### Project Structure Notes

- NEW: `internal/providers/{provider,registry,claude,codex,gemini,injection}.go` (+ `_test.go`).
- MODIFIED: `internal/config/config.go` (+ `config_test.go`), `internal/app/run.go` (+ `run_test.go`), `internal/shell/runner.go` (+ `runner_test.go`). Likely no change to `cmd/aictl/run.go`.
- No `internal/fallback`, `internal/checkpoint`, or detector code in this story.

### Resolved Decisions & Open Questions

**Resolved by Daniel (2026-06-16):**

1. **Unknown provider name → bare-executable fallback.** A name that is not a built-in or config provider runs as a plain executable with **no injection**, preserving Story 3.1's "run any CLI" behavior and tests. Implement this in `App.Run` (Task 5).
2. **`arg` mode injects the configured `PromptInjection.text` as a CLI arg** (and `file-ref` injects the short "read handoff.md" reference). `arg` does **not** read or inline the handoff contents — that would couple to handoff generation (Story 3.4). If `arg` text is empty, fall back to the file-ref text.

**Still open (sensible defaults applied; revisit only if a Trio CLI disagrees):**

3. **Prompt-arg position:** prompt appended as the **final** positional arg after provider args + user args (`claude [args] "<prompt>"`). A provider needing a flag (e.g. `-p`) expresses it via `args` in config. Default stands unless a Trio CLI needs otherwise.
4. **Paste delivery framing:** plain text + `\r` for v1; bracketed-paste (`ESC[200~ … ESC[201~`) only if a Trio CLI needs it for multi-line. `file-ref` (no PTY write) stays the robust default.

### References

- [Source: _bmad-output/planning-artifacts/epics.md#Story 3.3: Provider adapters — Trio, config-driven, and injection]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/prd.md#FR-15/FR-16/FR-17]
- [Source: _bmad-output/planning-artifacts/architecture.md#API & Communication — Provider adapter contract]
- [Source: _bmad-output/planning-artifacts/architecture.md#Communication Patterns — Provider registration]
- [Source: _bmad-output/planning-artifacts/architecture.md#Project Structure — internal/providers]
- [Source: _bmad-output/planning-artifacts/prds/prd-aictl-2026-06-15/addendum.md#Provider abstraction (interface sketch) + Prompt-injection strategies + Example Config]
- [Source: internal/config/config.go]
- [Source: internal/app/run.go]
- [Source: internal/app/handoff.go (config-load pattern)]
- [Source: internal/shell/runner.go]

## Review Findings

_Code review 2026-06-16 (Blind Hunter + Edge Case Hunter + Acceptance Auditor; clean diff vs committed 3.2 baseline `ce74d56`). All three layers completed. AC1, AC2, AC3, AC5, AC6 audited as satisfied; AC4 implemented with a coverage gap._

### Patch

- [x] [Review][Patch] `InitialInput` goroutine lingers and writes after teardown on a fast exit [internal/shell/runner.go] — FIXED: added an `injectDone` channel closed right after `cmd.Wait()` (before `ptmx.Close()`) and added it to the goroutine's `select`, so a fast-exiting provider returns the goroutine promptly and skips the post-close write. (`os.File` already returned `ErrClosed` rather than corrupting an fd, so this was a bounded leak, not Critical.)
- [x] [Review][Patch] Unrecognized config injection mode silently delivered as file-ref [internal/providers/provider.go + internal/app/run.go] — FIXED: added `Mode.Valid()`; `resolveLaunch` now returns a clear "unknown injection mode" error for an unrecognized resolved mode (before LookPath/side effects). `Inject` keeps its lenient default as a defensive safety net. Covered by `TestModeValid` and `TestRunInvalidConfigModeFailsBeforeRunner`.
- [x] [Review][Patch] No `App.Run` test exercises stdin/paste end-to-end [internal/app/run_test.go] — FIXED: `TestRunPasteProviderSetsInitialInput` resolves a config `paste` provider and asserts `InitialInput == "do it\r"` with no prompt arg.
- [x] [Review][Patch] No deliberate assertion that an unknown name produces no injection [internal/app/run_test.go] — FIXED: `TestRunUnknownNameRunsBareWithoutInjection` asserts an unknown name passes only the user args and sets no `InitialInput`.

### Deferred

- [x] [Review][Defer] paste/stdin injection robustness [internal/shell/runner.go, internal/providers/injection.go] — deferred. The 250ms delay is a magic constant with no TUI-readiness handshake; the `"\r"` submit byte may not suit every TUI; the injected write can interleave with early keystrokes. Inherent to the paste approach and flagged as a PRD open question; `file-ref` (no PTY write) is the robust default for the Trio. Revisit if a real Trio CLI needs paste.
- [x] [Review][Defer] provider-specific positional-arg handling [internal/providers/injection.go] — deferred. The prompt is appended as the final positional arg; a provider needing a flag (e.g. `-p`) or a `--` separator before the prompt expresses it via `args` in config today. Revisit per real provider needs.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.8 (claude-opus-4-8)

### Debug Log References

- `go test ./internal/config -run TestProviderPromptInjection` — RED (`PromptInjection` undefined) → GREEN after adding the field.
- `go test ./internal/providers` — GREEN (9 tests: built-in Trio defaults, overlay override/add, lookup miss, all injection modes + fallbacks).
- `go test ./internal/shell -run TestRunWritesInitialInputToPTY` — RED (`InitialInput` unknown field) → GREEN after adding the option + post-launch PTY write.
- `go test ./internal/app -run 'TestRunResolvesBuiltin|TestRunConfigProvider'` — GREEN; existing app tests stayed green via the unknown-name bare-exec fallback.
- `go test ./...` — 132 passed (11 packages). `go vet ./...`, `go build ./...`, `gofmt -l` — clean.
- `go test -race ./internal/shell ./internal/app ./internal/providers ./internal/config ./cmd/aictl` — 84 passed, race-clean.
- `GOOS=windows GOARCH=amd64 go build ./...` — clean. No networking imports added (NFR-1).
- `golangci-lint` — not installed in this environment.

### Completion Notes List

- Added `config.PromptInjection {Mode, Text}` to `config.Provider` (camelCase yaml tags); `config.Default()` Providers stays empty — the Trio lives in the registry, not the default file.
- New `internal/providers` package: a resolved `Provider` value type + `Mode` constants; built-in `claude`/`codex`/`gemini` adapters (command, file-ref mode, default usage-limit patterns as data only); `Resolve(cfg)` overlays config onto the Trio (non-empty fields override, new names add) into one map; `Lookup`. Dependency direction is `providers → config` only.
- `injection.go`: `Inject(handoffPath, userArgs)` → `Injection{Args, InitialInput}`. file-ref/arg append the prompt as the final argv arg; stdin/paste set `InitialInput` (`text + "\r"`) and no prompt arg; empty text falls back to the default file-reference instruction; unknown mode behaves as file-ref.
- `shell.Options.InitialInput`: the runner writes it into the PTY once, `initialInputDelay` (250ms) after launch, best-effort, respecting ctx cancellation; empty preserves Story 3.1/3.2 behavior exactly.
- `App.Run` rewired: `os.Getwd` → `session.NewPaths` → `config.Load(paths.Config())` → `providers.Resolve` → `Lookup`. Resolved command + injection; **unknown name → bare executable, no injection** (decision 1, preserves 3.1 + its tests). `exec.LookPath` of the resolved command runs before `openTranscript`, so a missing executable still fails before any side effect (AC5/3.1 AC6). Transcript wiring, `Mute`/`defer Flush`/inline `Flush`, and `ExitError` propagation from Story 3.2 are unchanged. `openTranscript` now takes `session.Paths` (computed once) instead of re-resolving the cwd.
- `arg` mode injects the provider's configured text (decision 2); it does not read handoff contents (that coupling belongs to Story 3.4). `cmd/aictl/run.go` unchanged — already thin and forwards `args[1:]`.
- Out of scope and not added (AC6): pre-run handoff generation/durability/checkpoints (3.4), usage-limit detection/classification/fallback/attempts (Epic 4), transcript/prompt secret scanning. Usage-limit patterns are provider data only.

### File List

- `internal/config/config.go` (modified — `PromptInjection` type + `Provider.PromptInjection`)
- `internal/config/config_test.go` (modified — promptInjection round-trip test)
- `internal/providers/provider.go` (new — resolved `Provider` + `Mode`)
- `internal/providers/claude.go` (new — built-in adapter)
- `internal/providers/codex.go` (new — built-in adapter)
- `internal/providers/gemini.go` (new — built-in adapter)
- `internal/providers/registry.go` (new — `Resolve` overlay + `Lookup`)
- `internal/providers/injection.go` (new — `Inject` + `Injection`)
- `internal/providers/providers_test.go` (new — registry + injection tests)
- `internal/shell/runner.go` (modified — `Options.InitialInput` + post-launch PTY write)
- `internal/shell/runner_test.go` (modified — `TestRunWritesInitialInputToPTY`)
- `internal/app/run.go` (modified — registry resolve + injection; `openTranscript(paths)`)
- `internal/app/run_test.go` (modified — built-in + config-provider resolution tests)

## Change Log

- 2026-06-16: Implemented Story 3.3 — `internal/providers` package (built-in Trio + config-overlay registry + prompt injection), `config.Provider.PromptInjection`, `shell.Options.InitialInput` for paste/stdin injection, and `App.Run` rewired to resolve+inject (unknown name → bare exec). Story moved to review.
- 2026-06-16: Addressed code review findings — 4 patches resolved (injection goroutine tied to run lifecycle via `injectDone`; `Mode.Valid()` + fail-fast on an unknown config injection mode; app-level stdin/paste `InitialInput` test; explicit unknown-name no-injection test). 2 items deferred to `deferred-work.md`. 8 dismissed (incl. verified false positives: `%`-in-path Sprintf, slice aliasing).
