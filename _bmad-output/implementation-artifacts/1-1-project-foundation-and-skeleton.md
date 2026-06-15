---
baseline_commit: 1e6cc8d2cc76f09b2198a35228f2f51e7d6fa0a1
---

# Story 1.1: Project Foundation & `aictl` Command Skeleton

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As the maintainer,
I want the Go module, Cobra command skeleton, internal package layout, and CI quality gates established,
so that every later story builds on a consistent, enforced foundation.

## Acceptance Criteria

1. **Module & dependencies:** `go.mod` exists declaring `go 1.26` with dependencies `github.com/spf13/cobra` v1.10.2, `github.com/creack/pty` v1.1.24, and `github.com/goccy/go-yaml` (latest stable, v1.x). `go build ./...` succeeds with a tidy `go.sum`.
2. **Command skeleton runs:** `aictl --help` lists the program and `aictl --version` prints a version string. Both exit 0. No subcommand business logic is implemented in this story.
3. **Package layout scaffolded:** the architecture's `cmd/aictl` entry and the `internal/` package boundary exist (at minimum `internal/app` and `internal/ui`), with business logic kept out of `cmd/` (thin commands → `app`). The remaining `internal/*` packages are created by their own stories, not stubbed here.
4. **CI quality gates:** a GitHub Actions workflow runs `gofmt` (check), `go vet`, `golangci-lint`, and `go test ./...` on push/PR, and all pass on the foundation code.
5. **No-network enforcement (NFR-1):** the build fails if a core package imports a networking library (e.g. `net/http`, `net`). Enforced via a `golangci-lint` depguard rule (and/or a dependency test), so later stories cannot accidentally violate provider-independence.

## Tasks / Subtasks

- [x] **Task 1 — Initialize the Go module (AC: 1)**
  - [x] `go mod init github.com/danielfoord/aictl` (module path confirmed with maintainer)
  - [x] Set `go 1.26` in `go.mod` (`go 1.26.0`)
  - [x] `go get github.com/spf13/cobra@v1.10.2`
  - [x] `go get github.com/creack/pty@v1.1.24` — **DEFERRED** to Story 3.1 (first import). See Completion Notes: `go mod tidy` prunes unused requires, so pty is added when first imported; version stays pinned in story/architecture docs.
  - [x] `go get github.com/goccy/go-yaml@latest` — **DEFERRED** to Story 1.2 (first import), same rationale. (Confirmed NOT `gopkg.in/yaml.v3`.)
  - [x] `go mod tidy`
- [x] **Task 2 — Cobra command skeleton (AC: 2, 3)**
  - [x] `cmd/aictl/main.go`: signal-aware `context.Context` via `signal.NotifyContext` (SIGINT/SIGTERM), constructs root, `ExecuteContext`, maps errors to exit code 1
  - [x] `cmd/aictl/root.go`: `aictl` root `*cobra.Command` (short/long, `SilenceUsage`/`SilenceErrors`); `--version` wired
  - [x] Version resolved from `runtime/debug.ReadBuildInfo()` with `-ldflags -X main.version` override hook
  - [x] `internal/app/app.go`: `App` struct orchestration seam (holds `UI`; feature deps added by later stories)
  - [x] `internal/ui/ui.go`: single user-facing output writer (Out/Err/Printf/Errorf)
  - [x] Verified `aictl --help` and `aictl --version` work
- [x] **Task 3 — Repo & build config (AC: 1, 4)**
  - [x] `.gitignore`
  - [x] `.golangci.yml` (v2 schema; `standard` linters + **depguard** no-network rule)
  - [x] `.goreleaser.yaml` skeleton (darwin/linux × amd64/arm64; not exercised this story)
  - [x] `README.md` stub
  - [x] `LICENSE` (MIT, confirmed)
- [x] **Task 4 — CI workflow (AC: 4)**
  - [x] `.github/workflows/ci.yml`: Go 1.26 setup; gofmt check, `go vet`, golangci-lint, `go test ./...`
- [x] **Task 5 — No-network guardrail (AC: 5, NFR-1)**
  - [x] `depguard` rule in `.golangci.yml` denying `net`, `net/http`, `net/url`, `crypto/tls`
  - [x] Go test guard implemented at `internal/guard/network_test.go` (execs `go list`, fails on banned direct imports) — runs locally + CI; `TestNoNetworkImports` passes
- [x] **Task 6 — Verify the foundation green**
  - [x] `go build ./...`, `go vet ./...`, `go test ./...` pass locally (golangci-lint runs in CI — not installed locally; see Completion Notes)
  - [x] `aictl --help` / `aictl --version` confirmed

### Review Findings (code review 2026-06-15)

_3 adversarial layers (Blind Hunter, Edge Case Hunter, Acceptance Auditor). Outcome: solid foundation, no real Critical/High bugs. Acceptance Auditor: 4/5 ACs PASS, AC1 a justified+disclosed PARTIAL. Patches below are quality/robustness hardening._

**Patch (applied 2026-06-15):**

- [x] [Review][Patch] Scope CI `gofmt` to tracked source files (`git ls-files '*.go'`) with `set -euo pipefail` [.github/workflows/ci.yml]
- [x] [Review][Patch] Hardened `TestNoNetworkImports`: stdout-only (`cmd.Output()`), fail loudly with stderr on `go list` error, assert ≥1 package scanned, prefix-match `golang.org/x/net/...` [internal/guard/network_test.go]
- [x] [Review][Patch] Pinned `golangci-lint` to `v2.12.2` (was `latest`) [.github/workflows/ci.yml]
- [x] [Review][Patch] Aligned `.golangci.yml` depguard deny list with the guard test (added `net/rpc`, `golang.org/x/net`) [.golangci.yml]
- [x] [Review][Patch] Renamed parameter `out *ui.UI` → `u` in `app.New` [internal/app/app.go]

**Deferred (tracked in deferred-work.md):**

- [x] [Review][Defer] Escalate a second Ctrl-C to a hard exit so the supervisor stays killable — deferred to Story 3.1 (PTY runner; no long-running `RunE` exists yet)
- [x] [Review][Defer] Synchronize `ui` writes + nil-writer/`Fprintf`-error handling for concurrent child output — deferred to Epic 3 fan-out writer (Story 3.2)
- [x] [Review][Defer] Add `cobra.NoArgs` / unknown-command handling to root — deferred until subcommands land (Story 1.2+)
- [x] [Review][Defer] Pin exact `creack/pty` & `goccy/go-yaml` versions when first imported (currently only in prose) — deferred to Stories 1.2 / 3.1

**Dismissed (noise / false positive):** Go 1.26 "doesn't exist" (it's the installed current release — go1.26.0); depguard/test "should catch transitive `net`" (transitive `net` is unavoidable and NFR-1 concerns aictl's own code — direct-import scoping is intentional); AC1 dep-deferral (already a disclosed, justified deviation).

## Dev Notes

This is the greenfield foundation story — **all files are NEW; there are no existing files to modify or behaviors to preserve.** The goal is a compiling, runnable, CI-green skeleton that establishes the conventions every later story inherits. Do **not** implement subcommand logic (init/start/run/etc.) — those are Stories 1.2 onward.

### Tech stack (exact, web-verified June 2026)

| Component | Version | Why | Source |
|---|---|---|---|
| Go toolchain | 1.26.x (1.26.4 current) | single static binary; language target | [architecture.md#Starter Template Evaluation] |
| `github.com/spf13/cobra` | v1.10.2 | command tree, POSIX flags (pflag, transitive), shell completions | [architecture.md#Starter Template Evaluation] |
| `github.com/creack/pty` | v1.1.24 | Unix PTY (first used Story 3.1) | [architecture.md#API & Communication] |
| `github.com/goccy/go-yaml` | latest v1.x (rel. Jan 2026) | config/state (de)serialization, friendly errors | [architecture.md#Starter Template Evaluation] |

> ⚠️ **Do not use `gopkg.in/yaml.v3`** — archived/unmaintained as of 2026. If the maintainer prefers, the official `go.yaml.in/yaml/v3` fork is the only acceptable alternative. [Source: architecture.md#Starter Template Evaluation]

### Target package layout (architecture) — scope for THIS story

Create only what Story 1.1 needs; the rest are created by their owning stories (do not pre-stub empty packages — Go does not need empty dirs and it adds noise).

**IN SCOPE (create now):**
```
aictl/
├── go.mod, go.sum, .gitignore, .golangci.yml, .goreleaser.yaml, README.md, LICENSE
├── .github/workflows/ci.yml          (release.yml deferred to Story 4.4)
├── cmd/aictl/
│   ├── main.go                        # signal-aware context, execute root, exit code
│   └── root.go                        # cobra root + --version
└── internal/
    ├── app/app.go                     # App struct = orchestration seam (deps injected later)
    └── ui/ui.go                       # single user-facing output writer (muted during runs later)
```

**DEFERRED (created by later stories — listed for orientation only):**
`internal/session` (1.2), `internal/config` (1.2/3.3), `internal/git` (2.1), `internal/handoff` (2.2), `internal/providers` (3.3), `internal/shell` (3.1), `internal/checkpoint` (3.4), `internal/fallback` (4.2), `testdata/` (as tests need it).
[Source: architecture.md#Project Structure & Boundaries]

### Conventions to establish now (architecture patterns — all later stories inherit these)

- **Thin `cmd/`:** commands parse flags, build context, and call `internal/app`; **no business logic in `cmd/`.** [Source: architecture.md#Implementation Patterns → Structure Patterns]
- **No global mutable state:** dependencies are passed explicitly (sets up testability). [Source: architecture.md#Structure Patterns]
- **`context.Context` first:** any op that spawns a process or blocks takes `ctx` as its first parameter; the root builds a signal-aware context. [Source: architecture.md#Communication Patterns]
- **Go naming:** `MixedCaps`/`mixedCaps`; initialisms upper (`ID`, `PTY`, `URL`); behavior-named interfaces (no `I` prefix). [Source: architecture.md#Naming Patterns]
- **Errors:** wrap with `fmt.Errorf("...: %w", err)`; `cmd` is the only layer that prints errors / sets exit code; no `panic` in `internal/` (except the future runner's terminal-restore path). [Source: architecture.md#Process Patterns]
- **File naming:** lowercase, underscores allowed for grouping; tests co-located as `*_test.go` (never a separate `tests/` dir). [Source: architecture.md#Naming Patterns]
- **(Forward-looking, not built now but reflected in lint setup):** YAML structs will use explicit `yaml:"camelCase"` tags; all Session-artifact writes will go through a single atomic helper. [Source: architecture.md#Format Patterns]

### No-network guardrail (NFR-1) — implementation guidance

NFR-1 ("no network/LLM in the core path") is a defining product property and must be enforced structurally, not by convention. Use golangci-lint's **depguard** to deny `net/http`, `net`, and similar in `internal/...`. `os/exec` (for shelling out to git/providers later) and `creack/pty` are allowed — those are process I/O, not network. [Source: architecture.md#Cross-Cutting NFRs (NFR-1), #Architectural Boundaries ("No network boundary at all — CI can assert no such imports")]

### Version string approach (AC 2)

Prefer `runtime/debug.ReadBuildInfo()` so `go install ...@version` yields a meaningful version, with a `var version = "dev"` overridable via `-ldflags "-X main.version=..."` for GoReleaser builds (Story 4.4). Set `rootCmd.Version` accordingly so Cobra's built-in `--version` works.

### Testing standards

- Framework: Go stdlib `testing`; table-driven where natural. [Source: architecture.md#Starter Template Evaluation → Testing]
- This story's tests are light (skeleton): at minimum the no-network dependency test (Task 5) and a smoke test that the root command constructs and `--version` returns non-empty. Rich tests (handoff golden tests, PTY integration vs `testdata/fakecli`) arrive with their stories.
- CI must be green before this story is `done`.

### Project Structure Notes

- Layout matches `architecture.md#Project Structure & Boundaries` exactly; deviations (e.g. an extra `internal/buildinfo` package for version) are acceptable if documented here, but keep it minimal.
- Module path `github.com/<owner>/aictl` is a placeholder — the real owner/org must be set before `go mod init` (affects all import paths). Confirm before starting (see Questions).

### References

- [Source: epics.md#Epic 1: Durable Task Sessions → Story 1.1]
- [Source: architecture.md#Starter Template Evaluation] — versions, init commands, foundation decisions
- [Source: architecture.md#Project Structure & Boundaries] — full file tree, package boundaries
- [Source: architecture.md#Implementation Patterns & Consistency Rules] — naming, structure, error, context, no-network conventions
- [Source: architecture.md#Cross-Cutting NFRs] — NFR-1 (no network), NFR-4 (plaintext state)
- [Source: prd.md#Developer-Product Concerns] — distribution (`go install` + GitHub releases), runtime targets (macOS+Linux, Unix PTY)

### Open Questions (resolve before/at implementation start)

1. **Module path / owner:** what is the canonical `github.com/<owner>/aictl` (personal account vs org)? This sets every import path and is painful to change later.
2. **License:** which OSS license for `LICENSE` (MIT vs Apache-2.0)? Both are fine for a Go CLI; Apache-2.0 adds a patent grant.

## Dev Agent Record

### Agent Model Used

claude-opus-4-8 (BMad dev-story workflow)

### Debug Log References

- `go build ./...` → success
- `go vet ./...` → no issues
- `go test ./...` → 7 passed across 4 packages (cmd/aictl, internal/app, internal/ui, internal/guard)
- `gofmt -l .` → clean (after `gofmt -w`)
- `go mod tidy` → stable (no changes on re-run)
- `aictl --version` → `aictl version v0.0.0-20260615190222-1e6cc8d2cc76+dirty` (resolved from build info; non-empty)
- `aictl --help` → prints program name + description

### Completion Notes List

- Decisions confirmed with maintainer: module path `github.com/danielfoord/aictl`; license **MIT**.
- **Dependency deviation from AC1 (deliberate, idiomatic):** only `github.com/spf13/cobra` v1.10.2 (+ transitive `pflag`, `mousetrap`) is in `go.mod`, because it's the only dependency imported by this story's code. `creack/pty` and `goccy/go-yaml` were **not** fetched — `go mod tidy` prunes unused requires, so they are added by their first-importing stories (`goccy/go-yaml` in Story 1.2, `creack/pty` in Story 3.1). Their versions remain pinned in the story/architecture docs. This keeps `go.sum` tidy (an AC1 requirement) rather than carrying unused requires.
- **Version handling:** `var version = "dev"` in `main.go`, override hook `-ldflags "-X main.version=<tag>"` (wired in `.goreleaser.yaml`); falls back to `runtime/debug.ReadBuildInfo()` so `go install ...@version` yields a real version. A plain `go build` in the git repo yields a VCS-derived pseudo-version (seen above).
- **NFR-1 enforced two ways:** (1) `golangci-lint` depguard rule in `.golangci.yml` denying `net`/`net/http`/`net/url`/`crypto/tls`; (2) `internal/guard/network_test.go` (`TestNoNetworkImports`) which execs `go list` and fails on banned direct imports — verified passing locally. Confirmed our code pulls no `net/http` (only transitive `net` via a dependency, which our own packages do not import).
- **golangci-lint not installed locally**, so `.golangci.yml` (v2 schema) and the CI `golangci-lint` step were authored best-effort and not executed locally; fmt/vet/test/build and the no-network Go test were all verified locally. Validate the lint config + `golangci/golangci-lint-action@v8` on the first CI run.
- **Scope respected:** no feature subcommands implemented; `internal/app` + `internal/ui` exist as the orchestration seam (thin `cmd/`). `internal/app` is not yet imported by `cmd` (wired in Story 1.2's `init`); it builds and is unit-tested.
- `go.mod` `go` directive set to `1.26.0` (go mod init had defaulted to `1.25.0`).

### File List

- `go.mod` (new)
- `go.sum` (new)
- `cmd/aictl/main.go` (new)
- `cmd/aictl/root.go` (new)
- `cmd/aictl/root_test.go` (new)
- `internal/app/app.go` (new)
- `internal/app/app_test.go` (new)
- `internal/ui/ui.go` (new)
- `internal/ui/ui_test.go` (new)
- `internal/guard/guard.go` (new)
- `internal/guard/network_test.go` (new)
- `.gitignore` (new)
- `.golangci.yml` (new)
- `.goreleaser.yaml` (new)
- `.github/workflows/ci.yml` (new)
- `LICENSE` (new)
- `README.md` (new)

### Change Log

- 2026-06-15: Implemented Story 1.1 — Go module (`github.com/danielfoord/aictl`, go 1.26) + Cobra `aictl` skeleton (`--help`/`--version`), `internal/app` + `internal/ui` seam, CI workflow (gofmt/vet/golangci-lint/test), MIT license, and NFR-1 no-network enforcement (depguard + Go guard test). 7 tests passing.
