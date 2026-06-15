package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/danielfoord/aictl/internal/git"
	"github.com/danielfoord/aictl/internal/session"
)

// Start opens a Session with an immutable Goal. It auto-initializes the Session
// Directory if needed, then acquires the single-session lock — a second Start in
// the same working tree is refused (session.ErrSessionLocked), which is what
// keeps the Goal immutable for the Session's life. The initial repo context
// (branch + clean/dirty) is captured best-effort.
func (a *App) Start(ctx context.Context, goal string) error {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return errors.New("a goal is required: aictl start \"<goal>\"")
	}

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)

	// Capture initial repo context BEFORE touching the tree: auto-init writes
	// .ai-session/, so capturing first keeps `dirty` faithful to the user's
	// working tree at session start. Best-effort — starting outside a git repo
	// still succeeds (records an empty branch).
	snap, gitErr := git.Capture(ctx, root)

	// AC4: start is usable without a prior `init` — auto-initialize if missing.
	switch _, statErr := os.Stat(paths.Dir()); {
	case errors.Is(statErr, os.ErrNotExist):
		if err := a.Init(ctx); err != nil {
			return err
		}
	case statErr != nil:
		return fmt.Errorf("check session directory: %w", statErr)
	}

	// Acquire the active-Session lock (race-free). A second start fails here.
	if err := session.AcquireLock(paths.Lock()); err != nil {
		return err
	}
	// Release the lock if we fail before the session is fully started, so a
	// transient error can't permanently wedge the working tree. The lock
	// persists only on success — it marks the active Session.
	committed := false
	defer func() {
		if !committed {
			_ = session.ReleaseLock(paths.Lock())
		}
	}()

	state, err := session.LoadState(paths.State())
	if err != nil {
		return err
	}
	state.Goal = goal
	if gitErr == nil {
		state.Branch = snap.Branch
		state.Dirty = snap.Dirty
	}

	if err := session.SaveState(paths.State(), state); err != nil {
		return err
	}

	committed = true
	a.UI.Printf("Started session: %q\n", goal)
	if gitErr == nil {
		a.UI.Printf("  branch: %s (dirty: %v)\n", branchLabel(snap.Branch), snap.Dirty)
	}
	return nil
}

func branchLabel(b string) string {
	if b == "" {
		return "(detached HEAD)"
	}
	return b
}
