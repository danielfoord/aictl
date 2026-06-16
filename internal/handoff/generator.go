// Package handoff generates the deterministic, LLM-free continuation packet
// that lets any provider resume an in-progress coding task. Generation is a
// pure function of its Input — no I/O, clock, or randomness — so the output is
// byte-stable and golden-testable (FR-11).
package handoff

import (
	"bytes"
	"fmt"
)

// Input is the complete, deterministic input to the handoff template. Callers
// assemble it from session + git state; the generator never reads the world.
type Input struct {
	Goal          string
	Branch        string
	Status        string
	Diff          string
	RecentCommits string
	VerifyOutput  string
	CommandLog    string
	NextSteps     []string
	Decisions     []string
	KnownFailures []string
}

// Generate renders the handoff markdown from in. Identical Input yields
// byte-identical output.
func Generate(in Input) (string, error) {
	var b bytes.Buffer
	if err := tmpl.Execute(&b, in); err != nil {
		return "", fmt.Errorf("render handoff: %w", err)
	}
	return b.String(), nil
}
