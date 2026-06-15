package app

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
)

// ErrAlreadyInitialized is returned when `aictl init` runs in a working tree
// that already has a Session Directory. Init is non-destructive: it refuses
// rather than overwriting existing state.
var ErrAlreadyInitialized = errors.New("aictl is already initialized here (.ai-session/ exists)")

// gitignoreContents ignores the transient / potentially-sensitive parts of the
// Session Directory. config.yaml, state.yaml and handoff.md stay tracked so the
// portable session travels with the repo; the lockfile is machine-local and
// checkpoints may contain raw transcripts, so both are ignored.
const gitignoreContents = `# Transient / sensitive aictl artifacts — not committed
.lock
.tmp-*
checkpoints/
`

// Init creates the Session Directory in the current working directory with a
// default Config and an empty Task State. It is non-destructive and performs no
// network access.
func (a *App) Init(_ context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)

	// os.Mkdir is an atomic create-or-fail: it returns os.ErrExist if the path
	// already exists (as a directory OR a file), making the non-destructive
	// guard race-free — no TOCTOU window between a stat and the create.
	if err := os.Mkdir(paths.Dir(), 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrAlreadyInitialized
		}
		return fmt.Errorf("create session directory: %w", err)
	}

	// If any step below fails, roll back the directory we just created so the
	// user can re-run init rather than being permanently refused. This only
	// ever removes a directory created by this call (a pre-existing one would
	// have failed the os.Mkdir above).
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(paths.Dir())
		}
	}()

	if err := os.Mkdir(paths.Checkpoints(), 0o755); err != nil {
		return fmt.Errorf("create checkpoints directory: %w", err)
	}

	cfgData, err := config.Default().Marshal()
	if err != nil {
		return fmt.Errorf("marshal default config: %w", err)
	}
	if err := session.WriteAtomic(paths.Config(), cfgData, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	stateData, err := session.TaskState{}.Marshal()
	if err != nil {
		return fmt.Errorf("marshal task state: %w", err)
	}
	if err := session.WriteAtomic(paths.State(), stateData, 0o644); err != nil {
		return fmt.Errorf("write task state: %w", err)
	}

	if err := session.WriteAtomic(paths.GitIgnore(), []byte(gitignoreContents), 0o644); err != nil {
		return fmt.Errorf("write .gitignore: %w", err)
	}

	ok = true
	a.UI.Printf("Initialized aictl session in %s\n", paths.Dir())
	return nil
}
