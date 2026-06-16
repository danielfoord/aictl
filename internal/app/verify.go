package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/verify"
)

// Verify runs the session's configured verification commands, captures their
// combined output to latest-verify.txt (for the next handoff), and appends the
// invocation to command-log.md. Output is captured even when a command fails;
// Verify's returned error (and thus the process exit code) reflects whether all
// commands passed.
func (a *App) Verify(ctx context.Context) error {
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
	if len(cfg.Verify) == 0 {
		a.UI.Printf("No verify commands configured. Add them under `verify:` in %s\n", paths.Config())
		return nil
	}

	res, err := verify.Run(ctx, root, cfg.Verify)
	if err != nil {
		return fmt.Errorf("run verify commands: %w", err)
	}

	// Capture output regardless of pass/fail (AC: test-aware handoff).
	if err := session.WriteAtomic(paths.LatestVerify(), []byte(res.Combined), 0o644); err != nil {
		return fmt.Errorf("write verify output: %w", err)
	}
	if err := appendCommandLog(paths.CommandLog(), res.Commands); err != nil {
		return fmt.Errorf("append command log: %w", err)
	}

	for _, c := range res.Commands {
		status := "ok"
		if c.ExitCode != 0 {
			status = fmt.Sprintf("FAILED (exit %d)", c.ExitCode)
		}
		a.UI.Printf("  %s — %s\n", c.Command, status)
	}

	if !res.AllPassed {
		return fmt.Errorf("verify failed: one or more commands did not pass (output captured to %s)", paths.LatestVerify())
	}
	a.UI.Printf("All verify commands passed; output captured to %s\n", paths.LatestVerify())
	return nil
}

// appendCommandLog appends one timestamped line per command to the command log
// (the one Session artifact that grows by append rather than full rewrite).
func appendCommandLog(path string, commands []verify.CommandResult) (err error) {
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if openErr != nil {
		return openErr
	}
	// Surface a failed final flush rather than dropping the last log line.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	ts := time.Now().UTC().Format(time.RFC3339)
	for _, c := range commands {
		if _, werr := fmt.Fprintf(f, "%s $ %s (exit %d)\n", ts, c.Command, c.ExitCode); werr != nil {
			return werr
		}
	}
	return nil
}
