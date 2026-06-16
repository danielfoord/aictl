package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
)

func TestRecoverWritesMinimalPrompt(t *testing.T) {
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
	if out, err := exec.Command("git", "-C", dir, "add", "main.go").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	if err := a.Recover(context.Background()); err != nil {
		t.Fatalf("recover: %v", err)
	}

	got := readFileT(t, session.NewPaths(dir).Recovery())
	for _, want := range []string{"Refactor classifier", "main.go", "Recover This Coding Task"} {
		if !strings.Contains(got, want) {
			t.Errorf("recovery prompt missing %q:\n%s", want, got)
		}
	}
	for _, forbidden := range []string{"Latest Verification Output", "Recent Commands", "Recent Commits"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("recovery prompt should not include %q:\n%s", forbidden, got)
		}
	}

	if err := a.Recover(context.Background()); err != nil {
		t.Fatalf("second recover: %v", err)
	}
	if again := readFileT(t, session.NewPaths(dir).Recovery()); again != got {
		t.Fatal("recovery prompt is not byte-identical for identical inputs")
	}
}

func TestRecoverSucceedsWithOnlyStateAndGit(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	a := newTestApp()
	if err := a.Start(context.Background(), "Minimal recovery"); err != nil {
		t.Fatalf("start: %v", err)
	}
	paths := session.NewPaths(dir)
	for _, p := range []string{paths.Config(), paths.Handoff(), paths.LatestVerify(), paths.CommandLog(), paths.Checkpoints()} {
		if err := os.RemoveAll(p); err != nil {
			t.Fatalf("remove optional artifact %s: %v", p, err)
		}
	}

	if err := a.Recover(context.Background()); err != nil {
		t.Fatalf("recover with optional artifacts missing: %v", err)
	}
	if got := readFileT(t, paths.Recovery()); !strings.Contains(got, "Minimal recovery") {
		t.Fatalf("recovery prompt missing goal:\n%s", got)
	}
}

func TestRecoverFallsBackWhenConfigMalformed(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	a := newTestApp()
	if err := a.Start(context.Background(), "Malformed config recovery"); err != nil {
		t.Fatalf("start: %v", err)
	}
	paths := session.NewPaths(dir)
	if err := os.WriteFile(paths.Config(), []byte("handoff: [\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := a.Recover(context.Background()); err != nil {
		t.Fatalf("recover should fall back to default config: %v", err)
	}
	if got := readFileT(t, paths.Recovery()); !strings.Contains(got, "Malformed config recovery") {
		t.Fatalf("recovery prompt missing goal:\n%s", got)
	}
}

func TestRecoverFailsOutsideGitRepo(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()
	if err := a.Start(context.Background(), "not in git"); err != nil {
		t.Fatalf("start: %v", err)
	}
	paths := session.NewPaths(dir)

	err := a.Recover(context.Background())
	if err == nil {
		t.Fatal("expected recover to fail outside a git repo")
	}
	if !strings.Contains(err.Error(), "recovery branch") {
		t.Fatalf("error should mention branch capture, got %v", err)
	}
	if _, statErr := os.Stat(paths.Recovery()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovery artifact should not be written, stat err = %v", statErr)
	}
}

func TestRecoverUsesDiffRedactionAndTruncation(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)

	a := newTestApp()
	if err := a.Start(context.Background(), "Protect secrets"); err != nil {
		t.Fatalf("start: %v", err)
	}
	cfg := config.Default()
	cfg.Handoff.MaxDiffChars = 500
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(session.NewPaths(dir).Config(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=topsecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(strings.Repeat("x\n", 1000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", ".env", "big.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	if err := a.Recover(context.Background()); err != nil {
		t.Fatalf("recover: %v", err)
	}
	got := readFileT(t, session.NewPaths(dir).Recovery())
	if strings.Contains(got, "topsecret") {
		t.Fatalf("secret leaked into recovery prompt:\n%s", got)
	}
	for _, want := range []string{"[redacted", "[truncated"} {
		if !strings.Contains(got, want) {
			t.Errorf("recovery prompt missing %q:\n%s", want, got)
		}
	}
}

func TestRecoverRequiresOriginalGoal(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, paths session.Paths)
	}{
		{
			name: "missing state",
			setup: func(t *testing.T, paths session.Paths) {
				t.Helper()
				if err := os.Remove(paths.State()); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "malformed state",
			setup: func(t *testing.T, paths session.Paths) {
				t.Helper()
				if err := os.WriteFile(paths.State(), []byte("goal: [\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "empty goal",
			setup: func(t *testing.T, paths session.Paths) {
				t.Helper()
				if err := session.SaveState(paths.State(), session.TaskState{}); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			gitInit(t, dir)
			t.Chdir(dir)
			a := newTestApp()
			if err := a.Start(context.Background(), "goal"); err != nil {
				t.Fatalf("start: %v", err)
			}
			paths := session.NewPaths(dir)
			tc.setup(t, paths)

			err := a.Recover(context.Background())
			if err == nil {
				t.Fatal("expected recover to fail without a readable original goal")
			}
			if !strings.Contains(err.Error(), "original goal") {
				t.Fatalf("error should mention original goal, got %v", err)
			}
			if _, statErr := os.Stat(paths.Recovery()); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("recovery artifact should not be written, stat err = %v", statErr)
			}
		})
	}
}
