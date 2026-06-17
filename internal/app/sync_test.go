package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
)

func writeGoalSourceConfig(t *testing.T, dir string, gs config.GoalSource) {
	t.Helper()
	cfg := config.Default()
	cfg.GoalSource = gs
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	paths := session.NewPaths(dir)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Config(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	// A real session always has state.yaml; write an empty one.
	if err := session.SaveState(paths.State(), session.TaskState{}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncPopulatesGoalAndContextFromFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeGoalSourceConfig(t, dir, config.GoalSource{File: "TASK.md"})
	if err := os.WriteFile(filepath.Join(dir, "TASK.md"),
		[]byte("goal: Implement the goal source\nnextSteps:\n  - read the source\n  - merge into state\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := newTestApp().Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatal(err)
	}
	if state.Goal != "Implement the goal source" {
		t.Fatalf("Goal = %q", state.Goal)
	}
	if len(state.NextSteps) != 2 || state.NextSteps[0] != "read the source" {
		t.Fatalf("NextSteps = %v", state.NextSteps)
	}
}

func TestSyncFromCommand(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeGoalSourceConfig(t, dir, config.GoalSource{Command: "printf 'just a plain goal\\n'"})

	if err := newTestApp().Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	state, _ := session.LoadState(session.NewPaths(dir).State())
	if state.Goal != "just a plain goal" {
		t.Fatalf("Goal = %q, want plain-text command output", state.Goal)
	}
}

func TestSyncRequiresSession(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := newTestApp().Sync(context.Background()); !errors.Is(err, ErrNoSession) {
		t.Fatalf("Sync error = %v, want ErrNoSession", err)
	}
}

func TestSyncLeavesExistingGoal(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeGoalSourceConfig(t, dir, config.GoalSource{File: "TASK.md"})
	if err := os.WriteFile(filepath.Join(dir, "TASK.md"), []byte("goal: from source\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	paths := session.NewPaths(dir)
	if err := session.SaveState(paths.State(), session.TaskState{Goal: "already mine"}); err != nil {
		t.Fatal(err)
	}

	if err := newTestApp().Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	state, _ := session.LoadState(paths.State())
	if state.Goal != "already mine" {
		t.Fatalf("Sync overwrote an existing goal: %q", state.Goal)
	}
}

func TestSyncNoSourceConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := newTestApp().Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	// Default config has no goalSource → no-op, no error, no goal set.
	if err := newTestApp().Sync(context.Background()); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	state, _ := session.LoadState(session.NewPaths(dir).State())
	if state.Goal != "" {
		t.Fatalf("Goal should stay empty with no source, got %q", state.Goal)
	}
}
