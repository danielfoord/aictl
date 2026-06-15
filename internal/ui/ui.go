// Package ui is the single user-facing output surface for aictl.
//
// Centralizing output here lets aictl stay silent while a provider owns the
// terminal during a supervised run (NFR-2: TUI non-interference) — nothing
// else in the codebase should write to the user's terminal directly.
package ui

import (
	"fmt"
	"io"
)

// UI writes user-facing messages to the configured output and error streams.
type UI struct {
	out io.Writer
	err io.Writer
}

// New constructs a UI writing to out (normal messages) and err (errors).
func New(out, err io.Writer) *UI {
	return &UI{out: out, err: err}
}

// Out returns the normal output writer.
func (u *UI) Out() io.Writer { return u.out }

// Err returns the error output writer.
func (u *UI) Err() io.Writer { return u.err }

// Printf writes a formatted message to the output stream. Write errors are
// deliberately ignored here (e.g. EPIPE when piped to `head`); a deliberate
// output-error policy is deferred to the Epic 3 fan-out writer.
func (u *UI) Printf(format string, args ...any) {
	_, _ = fmt.Fprintf(u.out, format, args...)
}

// Errorf writes a formatted message to the error stream. Write errors are
// deliberately ignored (see Printf).
func (u *UI) Errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(u.err, format, args...)
}
