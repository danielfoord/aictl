// Package session manages aictl's durable, repo-resident Session state: the
// `.ai-session/` directory, its layout, and atomic persistence of the Task
// State and other artifacts.
package session

import "path/filepath"

// DirName is the name of the Session Directory created at the repo root.
const DirName = ".ai-session"

// Paths resolves the locations of Session artifacts under a repo root. It is
// the single source of truth for `.ai-session/` paths — no other package should
// construct these paths by hand.
type Paths struct {
	root string // the .ai-session directory itself
}

// NewPaths returns the Paths rooted at repoRoot (the directory that contains,
// or will contain, the `.ai-session/` directory).
func NewPaths(repoRoot string) Paths {
	return Paths{root: filepath.Join(repoRoot, DirName)}
}

// Dir is the Session Directory (`<repo>/.ai-session`).
func (p Paths) Dir() string { return p.root }

// Config is the path to `config.yaml`.
func (p Paths) Config() string { return filepath.Join(p.root, "config.yaml") }

// State is the path to `state.yaml` (the Task State).
func (p Paths) State() string { return filepath.Join(p.root, "state.yaml") }

// Checkpoints is the path to the `checkpoints/` directory.
func (p Paths) Checkpoints() string { return filepath.Join(p.root, "checkpoints") }

// GitIgnore is the path to the Session Directory's `.gitignore`.
func (p Paths) GitIgnore() string { return filepath.Join(p.root, ".gitignore") }

// Lock is the path to the single-session lockfile (used from Story 1.3).
func (p Paths) Lock() string { return filepath.Join(p.root, ".lock") }

// StateLock is the transient lock used to serialize Task State mutations. It
// is intentionally separate from Lock, which marks an active provider session.
func (p Paths) StateLock() string { return filepath.Join(p.root, ".state.lock") }

// Handoff is the path to the current Handoff packet (`handoff.md`).
func (p Paths) Handoff() string { return filepath.Join(p.root, "handoff.md") }

// LatestVerify is the path to the most recent verification output. Populated by
// `aictl verify` (Story 2.3); read include-if-present by the handoff generator.
func (p Paths) LatestVerify() string { return filepath.Join(p.root, "latest-verify.txt") }

// CommandLog is the path to the aictl-invoked command log. Read
// include-if-present by the handoff generator.
func (p Paths) CommandLog() string { return filepath.Join(p.root, "command-log.md") }
