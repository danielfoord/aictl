package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/git"
	"github.com/danielfoord/aictl/internal/handoff"
	"github.com/danielfoord/aictl/internal/session"
)

// Recover writes a last-resort continuation prompt from only the original Goal
// and current git branch/diff. It intentionally avoids optional handoff inputs
// such as verify output, command log, commits, checkpoints, and transcripts.
func (a *App) Recover(ctx context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)

	state, err := session.LoadState(paths.State())
	if err != nil {
		return fmt.Errorf("recover requires original goal from task state: %w", err)
	}
	if strings.TrimSpace(state.Goal) == "" {
		return fmt.Errorf("recover requires original goal from task state")
	}

	cfg, err := config.Load(paths.Config())
	if err != nil {
		a.UI.Errorf("warning: config unavailable for recovery, using defaults: %v\n", err)
		cfg = config.Default()
	}

	branch, err := git.Branch(ctx, root)
	if err != nil {
		return fmt.Errorf("capture recovery branch: %w", err)
	}
	diff, err := git.Diff(ctx, root, cfg.Denylist, cfg.Handoff.MaxDiffChars)
	if err != nil {
		return fmt.Errorf("capture recovery diff: %w", err)
	}

	md, err := handoff.GenerateRecovery(handoff.RecoveryInput{
		Goal:   state.Goal,
		Branch: branch,
		Diff:   diff,
	})
	if err != nil {
		return err
	}
	if err := session.WriteAtomic(paths.Recovery(), []byte(md), 0o644); err != nil {
		return fmt.Errorf("write recovery prompt: %w", err)
	}

	a.UI.Printf("Wrote recovery prompt to %s\n", paths.Recovery())
	return nil
}
