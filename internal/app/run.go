package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/providers"
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

// Run resolves the named provider through the registry (built-in Trio overlaid
// by config), injects the handoff prompt per the provider's mode, and launches
// it through the shell PTY runner. An unknown name runs as a bare executable
// with no injection (preserving "run any CLI").
func (a *App) Run(ctx context.Context, name string, userArgs []string) (shell.Result, error) {
	root, err := os.Getwd()
	if err != nil {
		return shell.Result{ExitCode: 1}, fmt.Errorf("determine working directory: %w", err)
	}
	paths := session.NewPaths(root)

	cfg, err := config.Load(paths.Config())
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}

	command, injection, err := resolveLaunch(providers.Resolve(cfg), name, paths.Handoff(), userArgs)
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}

	// Resolve and validate the executable before any side effects (Story 3.1
	// AC6): a missing command fails here, before raw mode and before the
	// transcript is created.
	resolvedPath, err := exec.LookPath(command)
	if err != nil {
		return shell.Result{ExitCode: 1}, fmt.Errorf("provider executable not found %q: %w", command, err)
	}

	transcript, closeTranscript, err := openTranscript(paths)
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}
	defer closeTranscript()

	a.UI.Printf("Launching %s\n", name)
	// Stay silent while the provider owns the screen: buffer any aictl output
	// produced during the run and flush it once the provider exits (NFR-2/NFR-5).
	a.UI.Mute()
	// Deferred as a safety net so a panic or early return never leaves the UI
	// muted. Flush is idempotent, so the inline Flush on the normal path wins.
	defer a.UI.Flush()
	res, err := a.runProvider(ctx, shell.Options{
		Command:      resolvedPath,
		Args:         injection.Args,
		Stdin:        os.Stdin,
		Stdout:       os.Stdout,
		Stderr:       os.Stderr,
		Transcript:   transcript,
		InitialInput: injection.InitialInput,
	})
	a.UI.Flush()
	if err != nil {
		return res, err
	}
	a.UI.Printf("Provider %s exited with code %d\n", name, res.ExitCode)
	if res.ExitCode != 0 {
		return res, ExitError{Code: res.ExitCode}
	}
	return res, nil
}

// resolveLaunch turns the requested name into a command and an injection plan.
// A name in the resolved registry uses its command + prompt injection; an
// unknown name falls back to a bare executable with no injection.
func resolveLaunch(resolved map[string]providers.Provider, name, handoffPath string, userArgs []string) (string, providers.Injection, error) {
	p, ok := providers.Lookup(resolved, name)
	if !ok {
		return name, providers.Injection{Args: userArgs}, nil
	}
	if p.Command == "" {
		return "", providers.Injection{}, fmt.Errorf("provider %q has no command configured", name)
	}
	if !p.Mode.Valid() {
		return "", providers.Injection{}, fmt.Errorf("provider %q has unknown injection mode %q (want file-ref, arg, stdin, or paste)", name, p.Mode)
	}
	return p.Command, p.Inject(handoffPath, userArgs), nil
}

// openTranscript creates the per-run transcript stream at
// `<repo>/.ai-session/transcript.ansi`, creating the Session Directory if it
// does not exist yet (without requiring `aictl init`). The transcript is a live
// output stream, so it is opened directly rather than through
// session.WriteAtomic, which is for atomic whole-buffer snapshots, not streams.
func openTranscript(paths session.Paths) (io.Writer, func(), error) {
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
