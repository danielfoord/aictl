package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/danielfoord/aictl/internal/checkpoint"
	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
)

// Checkpoint captures an on-demand labeled snapshot of repo and task context.
// It is offline, provider-free, and does not regenerate the current handoff.
func (a *App) Checkpoint(ctx context.Context, label string) error {
	label = strings.TrimSpace(label)
	if label == "" {
		return errors.New("checkpoint label must not be empty")
	}
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
	state, err := session.LoadState(paths.State())
	if err != nil {
		return err
	}
	manual, err := checkpoint.CaptureManual(ctx, checkpoint.ManualOptions{
		Paths:        paths,
		RepoRoot:     root,
		Label:        label,
		State:        state,
		CommandLog:   readIfExists(paths.CommandLog()),
		LatestVerify: readIfExists(paths.LatestVerify()),
		Denylist:     cfg.Denylist,
		MaxDiffChars: cfg.Handoff.MaxDiffChars,
	})
	if err != nil {
		return err
	}
	a.UI.Printf("Wrote checkpoint %s to %s\n", filepath.Base(manual.Dir), manual.Dir)
	return nil
}
