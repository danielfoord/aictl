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
