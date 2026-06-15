package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/danielfoord/aictl/internal/session"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
}

func TestStartSetsGoalAndCapturesContext(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	if err := newTestApp().Start(context.Background(), "Refactor the classifier"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Goal != "Refactor the classifier" {
		t.Errorf("Goal = %q, want %q", state.Goal, "Refactor the classifier")
	}
	if state.Branch == "" {
		t.Error("expected a captured branch")
	}
}

func TestStartAutoInitsWithoutPriorInit(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	// No `init` first.
	if err := newTestApp().Start(context.Background(), "do a thing"); err != nil {
		t.Fatalf("Start without init: %v", err)
	}
	if _, err := os.Stat(session.NewPaths(dir).Config()); err != nil {
		t.Errorf("expected auto-init to create config.yaml: %v", err)
	}
}

func TestStartIsRefusedWhenSessionActive(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Start(context.Background(), "first goal"); err != nil {
		t.Fatalf("first Start: %v", err)
	}

	err := a.Start(context.Background(), "second goal")
	if !errors.Is(err, session.ErrSessionLocked) {
		t.Fatalf("second Start error = %v, want ErrSessionLocked", err)
	}

	// Goal must be immutable: still the first goal.
	state, _ := session.LoadState(session.NewPaths(dir).State())
	if state.Goal != "first goal" {
		t.Errorf("Goal changed to %q; expected immutable %q", state.Goal, "first goal")
	}
}

func TestStartReleasesLockOnFailure(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)
	paths := session.NewPaths(dir)

	// Pre-create .ai-session/ (so auto-init is skipped) with state.yaml as a
	// directory, so LoadState fails *after* the lock is acquired.
	if err := os.MkdirAll(paths.State(), 0o755); err != nil {
		t.Fatalf("seed bad state: %v", err)
	}

	if err := newTestApp().Start(context.Background(), "g"); err == nil {
		t.Fatal("expected Start to fail when state.yaml is unreadable")
	}

	// The lock must have been released so the user can recover and retry.
	if _, err := os.Stat(paths.Lock()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock should be released on failure; stat err = %v", err)
	}
}

func TestStartRejectsEmptyGoal(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	if err := newTestApp().Start(context.Background(), "   "); err == nil {
		t.Fatal("expected an error for an empty goal")
	}
}

func TestStartWorksOutsideGitRepo(t *testing.T) {
	dir := t.TempDir() // no git init
	t.Chdir(dir)

	// Should still start (git context capture is best-effort).
	if err := newTestApp().Start(context.Background(), "no git here"); err != nil {
		t.Fatalf("Start outside git repo: %v", err)
	}
	state, _ := session.LoadState(session.NewPaths(dir).State())
	if state.Goal != "no git here" {
		t.Errorf("Goal = %q, want %q", state.Goal, "no git here")
	}
	if state.Branch != "" {
		t.Errorf("expected empty branch outside a repo, got %q", state.Branch)
	}
}
