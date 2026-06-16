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
//
// While muted (between provider start and exit), messages are buffered in order
// and replayed on Flush, so aictl stays silent on the controlling terminal while
// a provider owns the screen (NFR-2/NFR-5) without losing any notices.
type UI struct {
	out io.Writer
	err io.Writer

	muted   bool
	pending []bufferedWrite
}

// bufferedWrite records one muted message and which stream it targets, so Flush
// can replay messages in their original order across out and err.
type bufferedWrite struct {
	toErr bool
	data  []byte
}

// New constructs a UI writing to out (normal messages) and err (errors).
func New(out, err io.Writer) *UI {
	return &UI{out: out, err: err}
}

// Mute buffers subsequent Printf/Errorf output instead of writing it to the
// terminal. It is used to keep aictl silent while a provider owns the screen.
// Mute is idempotent.
func (u *UI) Mute() { u.muted = true }

// Flush writes any buffered messages (in original order) and resumes direct
// writes. Flush is a no-op when not muted and is safe to call once per run.
func (u *UI) Flush() {
	if !u.muted {
		return
	}
	u.muted = false
	for _, w := range u.pending {
		if w.toErr {
			_, _ = u.err.Write(w.data)
		} else {
			_, _ = u.out.Write(w.data)
		}
	}
	u.pending = nil
}

// Out returns the normal output writer.
func (u *UI) Out() io.Writer { return u.out }

// Err returns the error output writer.
func (u *UI) Err() io.Writer { return u.err }

// Printf writes a formatted message to the output stream (or buffers it while
// muted). Write errors are deliberately ignored here (e.g. EPIPE when piped to
// `head`). During a provider run, terminal output is owned by the shell fan-out
// writer (internal/shell/fanwriter.go), which holds the output-error policy.
func (u *UI) Printf(format string, args ...any) {
	if u.muted {
		u.pending = append(u.pending, bufferedWrite{data: []byte(fmt.Sprintf(format, args...))})
		return
	}
	_, _ = fmt.Fprintf(u.out, format, args...)
}

// Errorf writes a formatted message to the error stream (or buffers it while
// muted). Write errors are deliberately ignored (see Printf).
func (u *UI) Errorf(format string, args ...any) {
	if u.muted {
		u.pending = append(u.pending, bufferedWrite{toErr: true, data: []byte(fmt.Sprintf(format, args...))})
		return
	}
	_, _ = fmt.Fprintf(u.err, format, args...)
}
