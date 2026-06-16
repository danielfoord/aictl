package shell

import (
	"bytes"
	"errors"
	"testing"
)

// errWriter fails every write after the first failAt successful writes.
type errWriter struct {
	buf     bytes.Buffer
	writes  int
	failAt  int // -1 never fails
	failErr error
}

func (w *errWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.failAt >= 0 && w.writes > w.failAt {
		return 0, w.failErr
	}
	return w.buf.Write(p)
}

func TestFanWriterMirrorsToTerminalAndTap(t *testing.T) {
	var term, tap bytes.Buffer
	w := newFanWriter(&term, &tap)

	payload := []byte("hello \x1b[31mred\x1b[0m world\r\n")
	n, err := w.Write(payload)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(payload) {
		t.Fatalf("n = %d, want %d", n, len(payload))
	}
	if !bytes.Equal(term.Bytes(), payload) {
		t.Fatalf("terminal = %q, want %q", term.Bytes(), payload)
	}
	if !bytes.Equal(tap.Bytes(), term.Bytes()) {
		t.Fatalf("tap = %q, want byte-identical to terminal %q", tap.Bytes(), term.Bytes())
	}
}

func TestFanWriterTapFailureDoesNotDegradeTerminal(t *testing.T) {
	var term bytes.Buffer
	tap := &errWriter{failAt: 1, failErr: errors.New("disk full")}
	w := newFanWriter(&term, tap)

	first := []byte("first\n")
	if n, err := w.Write(first); err != nil || n != len(first) {
		t.Fatalf("first Write = (%d, %v), want (%d, nil)", n, err, len(first))
	}

	// Second write: the tap errors, but the terminal must still receive it and
	// Write must report success for the terminal byte count.
	second := []byte("second\n")
	n, err := w.Write(second)
	if err != nil {
		t.Fatalf("tap failure must not surface as Write error, got %v", err)
	}
	if n != len(second) {
		t.Fatalf("n = %d, want %d (terminal byte count)", n, len(second))
	}

	// Third write: failed tap is dropped, terminal keeps receiving.
	third := []byte("third\n")
	if _, err := w.Write(third); err != nil {
		t.Fatalf("third Write: %v", err)
	}
	if got, want := term.String(), "first\nsecond\nthird\n"; got != want {
		t.Fatalf("terminal = %q, want %q", got, want)
	}
}

func TestFanWriterTerminalErrorPropagates(t *testing.T) {
	term := &errWriter{failAt: 0, failErr: errors.New("broken pipe")}
	var tap bytes.Buffer
	w := newFanWriter(term, &tap)

	_, err := w.Write([]byte("data"))
	if err == nil {
		t.Fatal("expected terminal write error to propagate")
	}
	if tap.Len() != 0 {
		t.Fatalf("tap should not be written when terminal fails, got %q", tap.Bytes())
	}
}

func TestFanWriterSkipsNilTaps(t *testing.T) {
	var term bytes.Buffer
	w := newFanWriter(&term, nil)

	payload := []byte("no tap")
	n, err := w.Write(payload)
	if err != nil || n != len(payload) {
		t.Fatalf("Write = (%d, %v), want (%d, nil)", n, err, len(payload))
	}
	if !bytes.Equal(term.Bytes(), payload) {
		t.Fatalf("terminal = %q, want %q", term.Bytes(), payload)
	}
}
