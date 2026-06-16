package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/shell"
)

// ExitError carries a process exit code that main should preserve.
type ExitError struct {
	Code int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("provider exited with code %d", e.Code)
}

// Run launches a provider executable through the shell PTY runner.
func (a *App) Run(ctx context.Context, provider string, args []string) (shell.Result, error) {
	path, err := exec.LookPath(provider)
	if err != nil {
		return shell.Result{ExitCode: 1}, fmt.Errorf("provider executable not found %q: %w", provider, err)
	}

	transcript, closeTranscript, err := openTranscript()
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}
	defer closeTranscript()

	a.UI.Printf("Launching %s\n", provider)
	// Stay silent while the provider owns the screen: buffer any aictl output
	// produced during the run and flush it once the provider exits (NFR-2/NFR-5).
	a.UI.Mute()
	// Deferred as a safety net so a panic or early return never leaves the UI
	// muted. Flush is idempotent, so the inline Flush on the normal path wins.
	defer a.UI.Flush()
	res, err := a.runProvider(ctx, shell.Options{
		Command:    path,
		Args:       args,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Transcript: transcript,
	})
	a.UI.Flush()
	if err != nil {
		return res, err
	}
	a.UI.Printf("Provider %s exited with code %d\n", provider, res.ExitCode)
	if res.ExitCode != 0 {
		return res, ExitError{Code: res.ExitCode}
	}
	return res, nil
}

// openTranscript creates the per-run transcript stream at
// `<repo>/.ai-session/transcript.ansi`, creating the Session Directory if it
// does not exist yet (without requiring `aictl init`). The transcript is a live
// output stream, so it is opened directly rather than through
// session.WriteAtomic, which is for atomic whole-buffer snapshots, not streams.
func openTranscript() (io.Writer, func(), error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, func() {}, fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		return nil, func() {}, fmt.Errorf("create session directory: %w", err)
	}
	// If the Session Directory was created lazily (run without a prior `init`),
	// write the ignore file so the secret-bearing transcript is never committed.
	if _, statErr := os.Stat(paths.GitIgnore()); errors.Is(statErr, os.ErrNotExist) {
		if err := session.WriteAtomic(paths.GitIgnore(), []byte(gitignoreContents), 0o644); err != nil {
			return nil, func() {}, fmt.Errorf("write session .gitignore: %w", err)
		}
	}
	// 0o600: the transcript is raw provider output that may contain secrets.
	f, err := os.OpenFile(paths.Transcript(), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create transcript: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}
