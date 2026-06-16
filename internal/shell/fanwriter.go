package shell

import "io"

// fanWriter mirrors provider PTY output to the terminal and tees the same bytes
// to zero or more taps. For Story 3.2 the only tap is the per-run transcript;
// the usage-limit detector is added as another tap in Story 4.1.
//
// Output-error policy (NFR-2 — capture must never degrade the UI):
//   - The terminal is authoritative. A terminal write error is returned to the
//     caller and ends the copy, exactly as the plain io.Copy did in Story 3.1.
//   - Taps are best-effort. A tap write error disables that tap for the rest of
//     the run; it never reduces the reported byte count and never surfaces as a
//     Write error, so a failing transcript can never disturb the on-screen TUI.
//
// This is deliberately not io.MultiWriter, which fails the whole write if any
// writer errors — that would let a transcript failure degrade the screen.
type fanWriter struct {
	term io.Writer
	taps []io.Writer
}

// newFanWriter builds a fan-out writing to term and teeing to taps. Nil taps
// are skipped, so callers can pass an optional transcript writer directly.
func newFanWriter(term io.Writer, taps ...io.Writer) *fanWriter {
	kept := make([]io.Writer, 0, len(taps))
	for _, t := range taps {
		if t != nil {
			kept = append(kept, t)
		}
	}
	return &fanWriter{term: term, taps: kept}
}

// Write sends p to the terminal first, then tees the accepted bytes to each
// live tap. See the type doc for the error policy.
func (w *fanWriter) Write(p []byte) (int, error) {
	n, err := w.term.Write(p)
	if err != nil {
		return n, err
	}
	for i, t := range w.taps {
		if t == nil {
			continue
		}
		if _, terr := t.Write(p[:n]); terr != nil {
			w.taps[i] = nil // drop the failing tap; never degrade the screen
		}
	}
	return n, nil
}
