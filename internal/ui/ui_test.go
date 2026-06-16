package ui

import (
	"bytes"
	"testing"
)

func TestPrintfWritesToOut(t *testing.T) {
	var out, errBuf bytes.Buffer
	u := New(&out, &errBuf)

	u.Printf("hello %s", "world")

	if got := out.String(); got != "hello world" {
		t.Fatalf("Printf wrote %q, want %q", got, "hello world")
	}
	if errBuf.Len() != 0 {
		t.Fatalf("Printf unexpectedly wrote to err: %q", errBuf.String())
	}
}

func TestErrorfWritesToErr(t *testing.T) {
	var out, errBuf bytes.Buffer
	u := New(&out, &errBuf)

	u.Errorf("boom: %d", 42)

	if got := errBuf.String(); got != "boom: 42" {
		t.Fatalf("Errorf wrote %q, want %q", got, "boom: 42")
	}
	if out.Len() != 0 {
		t.Fatalf("Errorf unexpectedly wrote to out: %q", out.String())
	}
}

func TestMuteBuffersUntilFlush(t *testing.T) {
	var out, errBuf bytes.Buffer
	u := New(&out, &errBuf)

	u.Printf("before\n")
	u.Mute()
	u.Printf("during-%d\n", 1)
	u.Errorf("during-err\n")

	if out.String() != "before\n" {
		t.Fatalf("muted output leaked to terminal: %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("muted error leaked to terminal: %q", errBuf.String())
	}

	u.Flush()
	if got, want := out.String(), "before\nduring-1\n"; got != want {
		t.Fatalf("after flush out = %q, want %q", got, want)
	}
	if got, want := errBuf.String(), "during-err\n"; got != want {
		t.Fatalf("after flush err = %q, want %q", got, want)
	}

	// Writes after flush pass straight through again.
	u.Printf("after\n")
	if got, want := out.String(), "before\nduring-1\nafter\n"; got != want {
		t.Fatalf("post-flush passthrough out = %q, want %q", got, want)
	}
}

func TestFlushWithoutMuteIsNoop(t *testing.T) {
	var out, errBuf bytes.Buffer
	u := New(&out, &errBuf)
	u.Flush() // must not panic or write
	u.Printf("x")
	if out.String() != "x" {
		t.Fatalf("out = %q, want %q", out.String(), "x")
	}
}
