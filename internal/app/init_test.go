package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/ui"
)

func newTestApp() *App {
	return New(ui.New(io.Discard, io.Discard))
}

func TestInitCreatesSessionDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := newTestApp().Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	paths := session.NewPaths(dir)
	for _, p := range []string{paths.Dir(), paths.Config(), paths.State(), paths.Checkpoints(), paths.GitIgnore()} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist: %v", p, err)
		}
	}
}

func TestInitWritesValidDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := newTestApp().Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	data, err := os.ReadFile(session.NewPaths(dir).Config())
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	cfg, err := config.Unmarshal(data)
	if err != nil {
		t.Fatalf("config is not valid YAML: %v", err)
	}
	if cfg.Handoff.MaxDiffChars != config.DefaultMaxDiffChars {
		t.Errorf("MaxDiffChars = %d, want %d", cfg.Handoff.MaxDiffChars, config.DefaultMaxDiffChars)
	}
	if len(cfg.Denylist) == 0 {
		t.Error("expected a non-empty default secret denylist")
	}
}

func TestInitWritesEmptyTaskState(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := newTestApp().Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	data, err := os.ReadFile(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	state, err := session.UnmarshalTaskState(data)
	if err != nil {
		t.Fatalf("state is not valid YAML: %v", err)
	}
	if state.Goal != "" {
		t.Errorf("expected empty Goal, got %q", state.Goal)
	}
}

func TestInitIsNonDestructive(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	// Mutate state to detect any overwrite.
	statePath := session.NewPaths(dir).State()
	sentinel := []byte("goal: do-not-clobber\n")
	if err := os.WriteFile(statePath, sentinel, 0o644); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	err := a.Init(context.Background())
	if !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Init error = %v, want ErrAlreadyInitialized", err)
	}

	got, _ := os.ReadFile(statePath)
	if string(got) != string(sentinel) {
		t.Fatalf("second Init clobbered existing state: %q", got)
	}
}

func TestInitGitignoreIgnoresTransient(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := newTestApp().Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(session.NewPaths(dir).Dir(), ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	for _, want := range []string{".lock", ".state.lock", ".tmp-*", "latest-verify.txt", "command-log.md", "transcript.ansi", "checkpoints/"} {
		if !strings.Contains(string(data), want) {
			t.Errorf(".gitignore missing entry %q; got:\n%s", want, data)
		}
	}
}
