package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/session"
)

func TestHandoffWritesRedactedPacket(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	a := newTestApp()
	if err := a.Start(context.Background(), "Refactor classifier"); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=topsecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "main.go", ".env").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	if err := a.Handoff(context.Background()); err != nil {
		t.Fatalf("handoff: %v", err)
	}

	data, err := os.ReadFile(session.NewPaths(dir).Handoff())
	if err != nil {
		t.Fatalf("read handoff: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "Refactor classifier") {
		t.Error("handoff missing the goal")
	}
	if !strings.Contains(s, "main.go") {
		t.Error("handoff missing the diff of main.go")
	}
	if strings.Contains(s, "topsecret") {
		t.Fatalf("SECRET leaked into the handoff:\n%s", s)
	}
}

func TestHandoffRequiresSession(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := newTestApp().Handoff(context.Background()); !errors.Is(err, ErrNoSession) {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
}
