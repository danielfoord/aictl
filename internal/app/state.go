package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/danielfoord/aictl/internal/session"
)

// ErrNoSession is returned when a state command runs outside an initialized
// aictl Session Directory.
var ErrNoSession = errors.New("no aictl session here; run `aictl start` first")

const stateLockRetryInterval = 10 * time.Millisecond

type stateField struct {
	usage        string
	confirmation string
	append       func(*session.TaskState, string)
}

var (
	decisionField = stateField{
		usage:        "note \"<decision>\"",
		confirmation: "noted decision",
		append: func(s *session.TaskState, value string) {
			s.Decisions = append(s.Decisions, value)
		},
	}
	completedField = stateField{
		usage:        "done \"<completed step>\"",
		confirmation: "recorded completed step",
		append: func(s *session.TaskState, value string) {
			s.Completed = append(s.Completed, value)
		},
	}
	nextStepField = stateField{
		usage:        "next \"<next step>\"",
		confirmation: "recorded next step",
		append: func(s *session.TaskState, value string) {
			s.NextSteps = append(s.NextSteps, value)
		},
	}
	knownFailureField = stateField{
		usage:        "fail \"<known failure>\"",
		confirmation: "recorded known failure",
		append: func(s *session.TaskState, value string) {
			s.KnownFailures = append(s.KnownFailures, value)
		},
	}
)

// Note records a decision in the current Session Task State.
func (a *App) Note(ctx context.Context, value string) error {
	return a.appendEntry(ctx, decisionField, value)
}

// Done records a completed step in the current Session Task State.
func (a *App) Done(ctx context.Context, value string) error {
	return a.appendEntry(ctx, completedField, value)
}

// Next records a next step in the current Session Task State.
func (a *App) Next(ctx context.Context, value string) error {
	return a.appendEntry(ctx, nextStepField, value)
}

// Fail records a known failure in the current Session Task State.
func (a *App) Fail(ctx context.Context, value string) error {
	return a.appendEntry(ctx, knownFailureField, value)
}

func (a *App) appendEntry(ctx context.Context, field stateField, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("a value is required: aictl %s", field.usage)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)

	info, err := os.Stat(paths.Dir())
	switch {
	case errors.Is(err, os.ErrNotExist):
		return ErrNoSession
	case err != nil:
		return fmt.Errorf("check session directory: %w", err)
	case !info.IsDir():
		return ErrNoSession
	}

	if err := acquireStateLock(ctx, paths.StateLock()); err != nil {
		return err
	}
	defer func() { _ = session.ReleaseLock(paths.StateLock()) }()

	state, err := session.LoadState(paths.State())
	if err != nil {
		return fmt.Errorf("load task state: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	field.append(&state, value)

	if err := session.SaveState(paths.State(), state); err != nil {
		return fmt.Errorf("save task state: %w", err)
	}

	a.UI.Printf("%s: %q\n", field.confirmation, value)
	return nil
}

func acquireStateLock(ctx context.Context, path string) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := session.AcquireLock(path)
		if err == nil {
			return nil
		}
		if !errors.Is(err, session.ErrSessionLocked) {
			return err
		}

		timer := time.NewTimer(stateLockRetryInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}
