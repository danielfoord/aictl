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

// Printf writes a formatted message to the output stream.
func (u *UI) Printf(format string, args ...any) {
	fmt.Fprintf(u.out, format, args...)
}

// Errorf writes a formatted message to the error stream.
func (u *UI) Errorf(format string, args ...any) {
	fmt.Fprintf(u.err, format, args...)
}
