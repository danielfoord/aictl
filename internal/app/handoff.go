package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/danielfoord/aictl/internal/checkpoint"
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

	prepared, err := a.prepareHandoff(ctx, root, paths, false)
	if err != nil {
		return err
	}
	if err := session.WriteAtomic(paths.Handoff(), []byte(prepared.Markdown), 0o644); err != nil {
		return fmt.Errorf("write handoff: %w", err)
	}

	a.UI.Printf("Wrote handoff to %s\n", paths.Handoff())
	return nil
}

type preparedHandoff struct {
	Config     config.Config
	State      session.TaskState
	Markdown   string
	CommandLog string
	Git        checkpoint.GitState
}

func (a *App) prepareHandoff(ctx context.Context, root string, paths session.Paths, allowCreate bool) (preparedHandoff, error) {
	if allowCreate {
		if err := ensureRunSession(paths); err != nil {
			return preparedHandoff{}, err
		}
	} else {
		switch _, statErr := os.Stat(paths.Dir()); {
		case errors.Is(statErr, os.ErrNotExist):
			return preparedHandoff{}, ErrNoSession
		case statErr != nil:
			return preparedHandoff{}, fmt.Errorf("check session directory: %w", statErr)
		}
	}

	cfg, err := config.Load(paths.Config())
	if err != nil {
		return preparedHandoff{}, err
	}
	state, err := session.LoadState(paths.State())
	if err != nil {
		return preparedHandoff{}, err
	}

	commandLog := readIfExists(paths.CommandLog())
	in := handoff.Input{
		Goal:          state.Goal,
		Branch:        state.Branch,
		NextSteps:     state.NextSteps,
		Decisions:     state.Decisions,
		KnownFailures: state.KnownFailures,
		VerifyOutput:  readIfExists(paths.LatestVerify()),
		CommandLog:    commandLog,
	}
	gitState := checkpoint.GitState{}
	if status, gerr := git.Status(ctx, root); gerr == nil {
		gitState.Available = true
		gitState.Status = git.RedactStatusPaths(status, cfg.Denylist)
		in.Status = gitState.Status
	} else if !errors.Is(gerr, git.ErrNotARepo) {
		if allowCreate {
			return preparedHandoff{}, fmt.Errorf("capture git status: %w", gerr)
		}
		a.UI.Errorf("warning: git status capture failed: %v\n", gerr)
	}
	if diff, gerr := git.Diff(ctx, root, cfg.Denylist, cfg.Handoff.MaxDiffChars); gerr == nil {
		gitState.Available = true
		gitState.Diff = diff
		in.Diff = diff
	} else if !errors.Is(gerr, git.ErrNotARepo) {
		if allowCreate {
			return preparedHandoff{}, fmt.Errorf("capture git diff: %w", gerr)
		}
		a.UI.Errorf("warning: git diff capture failed: %v\n", gerr)
	}
	if commits, gerr := git.RecentCommits(ctx, root, 10); gerr == nil {
		gitState.Available = true
		gitState.RecentCommits = commits
		in.RecentCommits = commits
	} else if !errors.Is(gerr, git.ErrNotARepo) {
		if allowCreate {
			return preparedHandoff{}, fmt.Errorf("capture git log: %w", gerr)
		}
		a.UI.Errorf("warning: git log capture failed: %v\n", gerr)
	}

	md, err := handoff.Generate(in)
	if err != nil {
		return preparedHandoff{}, err
	}
	return preparedHandoff{Config: cfg, State: state, Markdown: md, CommandLog: commandLog, Git: gitState}, nil
}

func ensureRunSession(paths session.Paths) error {
	if err := os.MkdirAll(paths.Checkpoints(), 0o755); err != nil {
		return fmt.Errorf("create session checkpoints directory: %w", err)
	}
	if _, statErr := os.Stat(paths.GitIgnore()); errors.Is(statErr, os.ErrNotExist) {
		if err := session.WriteAtomic(paths.GitIgnore(), []byte(gitignoreContents), 0o644); err != nil {
			return fmt.Errorf("write session .gitignore: %w", err)
		}
	} else if statErr != nil {
		return fmt.Errorf("check session .gitignore: %w", statErr)
	}
	if _, statErr := os.Stat(paths.State()); errors.Is(statErr, os.ErrNotExist) {
		if err := session.SaveState(paths.State(), session.TaskState{}); err != nil {
			return err
		}
	} else if statErr != nil {
		return fmt.Errorf("check task state: %w", statErr)
	}
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
