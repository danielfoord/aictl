package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAtomicWritesContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	if err := WriteAtomic(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("content = %q, want %q", got, "hello")
	}
}

func TestWriteAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	if err := WriteAtomic(path, []byte("a"), 0o644); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected exactly 1 file, got %d: %v", len(entries), names)
	}
}

func TestWriteAtomicOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.yaml")

	if err := WriteAtomic(path, []byte("first"), 0o644); err != nil {
		t.Fatalf("first WriteAtomic: %v", err)
	}
	if err := WriteAtomic(path, []byte("second"), 0o644); err != nil {
		t.Fatalf("second WriteAtomic: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "second" {
		t.Fatalf("content = %q, want %q", got, "second")
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file after overwrite, got %d", len(entries))
	}
}

func TestWriteAtomicAppliesPerm(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f")

	if err := WriteAtomic(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteAtomic: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm = %o, want %o", info.Mode().Perm(), 0o600)
	}
}
