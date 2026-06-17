package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/goalsource"
	"github.com/danielfoord/aictl/internal/session"
)

// Sync populates the session goal (and any structured next-steps/decisions/known-
// failures) from the configured goalSource, when the session has no goal yet. It
// is tool-agnostic — aictl just reads the user's configured file or command — and
// does not overwrite an existing goal.
func (a *App) Sync(ctx context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)
	switch info, statErr := os.Stat(paths.Dir()); {
	case errors.Is(statErr, os.ErrNotExist):
		return ErrNoSession
	case statErr != nil:
		return fmt.Errorf("check session directory: %w", statErr)
	case !info.IsDir():
		return ErrNoSession
	}

	cfg, err := config.Load(paths.Config())
	if err != nil {
		return err
	}
	if cfg.GoalSource.File == "" && cfg.GoalSource.Command == "" {
		a.UI.Printf("No goalSource configured in config.yaml; nothing to sync.\n")
		return nil
	}

	text, ok, err := goalsource.Read(ctx, root, cfg.GoalSource.File, cfg.GoalSource.Command)
	if err != nil {
		return err
	}
	if !ok {
		a.UI.Printf("goalSource produced no output; state unchanged.\n")
		return nil
	}
	parsed := goalsource.Parse(text)
	if parsed.Goal == "" {
		a.UI.Printf("goalSource produced no goal; state unchanged.\n")
		return nil
	}

	state, err := session.LoadState(paths.State())
	if err != nil {
		return err
	}
	if state.Goal != "" {
		a.UI.Printf("Goal already set (%q); leaving it. (source goal: %q)\n", state.Goal, parsed.Goal)
		return nil
	}

	state.Goal = parsed.Goal
	state.NextSteps = append(state.NextSteps, parsed.NextSteps...)
	state.Decisions = append(state.Decisions, parsed.Decisions...)
	state.KnownFailures = append(state.KnownFailures, parsed.KnownFailures...)
	if err := session.SaveState(paths.State(), state); err != nil {
		return err
	}

	a.UI.Printf("Synced goal: %q\n", parsed.Goal)
	if n := len(parsed.NextSteps); n > 0 {
		a.UI.Printf("  + %d next step(s) from source\n", n)
	}
	return nil
}
