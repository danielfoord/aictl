package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/git"
	"github.com/danielfoord/aictl/internal/handoff"
	"github.com/danielfoord/aictl/internal/session"
)

// Handoff generates the deterministic handoff packet (.ai-session/handoff.md)
// from current task + git state. It is offline and invokes no provider. Git
// capture is best-effort: a non-repo / unborn repo simply yields empty sections.
func (a *App) Handoff(ctx context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)

	switch _, statErr := os.Stat(paths.Dir()); {
	case errors.Is(statErr, os.ErrNotExist):
		return ErrNoSession
	case statErr != nil:
		return fmt.Errorf("check session directory: %w", statErr)
	}

	cfg, err := config.Load(paths.Config())
	if err != nil {
		return err
	}
	state, err := session.LoadState(paths.State())
	if err != nil {
		return err
	}

	in := handoff.Input{
		Goal:          state.Goal,
		Branch:        state.Branch,
		NextSteps:     state.NextSteps,
		Decisions:     state.Decisions,
		KnownFailures: state.KnownFailures,
		VerifyOutput:  readIfExists(paths.LatestVerify()),
		CommandLog:    readIfExists(paths.CommandLog()),
	}
	// Best-effort git capture: outside a repo (ErrNotARepo) sections are simply
	// empty; a *real* git failure is surfaced as a warning so the handoff isn't
	// silently rendered as a clean tree. The diff is redacted+bounded inside
	// git.Diff; status paths are redacted here for symmetry.
	if status, gerr := git.Status(ctx, root); gerr == nil {
		in.Status = git.RedactStatusPaths(status, cfg.Denylist)
	} else if !errors.Is(gerr, git.ErrNotARepo) {
		a.UI.Errorf("warning: git status capture failed: %v\n", gerr)
	}
	if diff, gerr := git.Diff(ctx, root, cfg.Denylist, cfg.Handoff.MaxDiffChars); gerr == nil {
		in.Diff = diff
	} else if !errors.Is(gerr, git.ErrNotARepo) {
		a.UI.Errorf("warning: git diff capture failed: %v\n", gerr)
	}
	if commits, gerr := git.RecentCommits(ctx, root, 10); gerr == nil {
		in.RecentCommits = commits
	} else if !errors.Is(gerr, git.ErrNotARepo) {
		a.UI.Errorf("warning: git log capture failed: %v\n", gerr)
	}

	md, err := handoff.Generate(in)
	if err != nil {
		return err
	}
	if err := session.WriteAtomic(paths.Handoff(), []byte(md), 0o644); err != nil {
		return fmt.Errorf("write handoff: %w", err)
	}

	a.UI.Printf("Wrote handoff to %s\n", paths.Handoff())
	return nil
}

// readIfExists returns the file's contents, or "" if it is absent/unreadable.
func readIfExists(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
