package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/shell"
)

func TestCheckpointRequiresSession(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := newTestApp().Checkpoint(context.Background(), "before risk")
	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("Checkpoint error = %v, want ErrNoSession", err)
	}
	if _, statErr := os.Stat(session.NewPaths(dir).Dir()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("checkpoint without session should not create session dir, stat = %v", statErr)
	}
}

func TestCheckpointWritesLabeledArtifactsWithoutProvider(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	a := newTestApp()
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		t.Fatal("Checkpoint must not launch a provider")
		return shell.Result{}, nil
	}
	if err := a.Start(context.Background(), "snapshot task state"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := a.Next(context.Background(), "wire checkpoint command"); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if err := a.Note(context.Background(), "reuse checkpoint package"); err != nil {
		t.Fatalf("Note: %v", err)
	}
	if err := a.Fail(context.Background(), "watch redaction"); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	paths := session.NewPaths(dir)
	if err := session.WriteAtomic(paths.Handoff(), []byte("sentinel handoff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteAtomic(paths.CommandLog(), []byte("2026-06-16T12:00:00Z $ go test ./... (exit 0)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := session.WriteAtomic(paths.LatestVerify(), []byte("verify ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=topsecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", "main.go", ".env")

	if err := a.Checkpoint(context.Background(), "../Before risky refactor"); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	cpDir := filepath.Join(paths.Checkpoints(), "0001-checkpoint-Before-risky-refactor")
	for _, name := range []string{"git-status.txt", "git-diff.patch", "recent-commits.txt", "command-log.md", "summary.md", "latest-verify.txt"} {
		if _, err := os.Stat(filepath.Join(cpDir, name)); err != nil {
			t.Fatalf("expected checkpoint artifact %s: %v", name, err)
		}
	}
	summary, err := os.ReadFile(filepath.Join(cpDir, "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Before risky refactor", "snapshot task state", "wire checkpoint command", "reuse checkpoint package", "watch redaction"} {
		if !strings.Contains(string(summary), want) {
			t.Fatalf("summary missing %q:\n%s", want, summary)
		}
	}
	diff, err := os.ReadFile(filepath.Join(cpDir, "git-diff.patch"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(diff), "topsecret") {
		t.Fatalf("secret leaked into checkpoint diff:\n%s", diff)
	}
	handoff, err := os.ReadFile(paths.Handoff())
	if err != nil {
		t.Fatal(err)
	}
	if string(handoff) != "sentinel handoff\n" {
		t.Fatalf("Checkpoint mutated handoff.md: %q", handoff)
	}
}

func TestCheckpointRejectsBlankLabel(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := newTestApp().Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	err := newTestApp().Checkpoint(context.Background(), " \n\t ")
	if err == nil {
		t.Fatal("expected blank label error")
	}
	entries, readErr := os.ReadDir(session.NewPaths(dir).Checkpoints())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("blank label should not create checkpoints, got %d", len(entries))
	}
}
