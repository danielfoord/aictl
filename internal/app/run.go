package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/danielfoord/aictl/internal/checkpoint"
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

// PostRunError reports a post-provider checkpoint failure while preserving the
// provider result for CLI exit-code handling.
type PostRunError struct {
	Code          int
	ProviderErr   error
	CheckpointErr error
}

func (e PostRunError) Error() string {
	if e.ProviderErr != nil {
		return fmt.Sprintf("provider finished with error and post-run checkpoint failed: %v: %v", e.ProviderErr, e.CheckpointErr)
	}
	return fmt.Sprintf("post-run checkpoint failed after provider exit code %d: %v", e.Code, e.CheckpointErr)
}

func (e PostRunError) Unwrap() []error {
	var errs []error
	if e.CheckpointErr != nil {
		errs = append(errs, e.CheckpointErr)
	}
	if e.ProviderErr != nil {
		errs = append(errs, e.ProviderErr)
	}
	if e.Code != 0 {
		errs = append(errs, ExitError{Code: e.Code})
	}
	return errs
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

	prepared, err := a.prepareHandoff(ctx, root, paths, true)
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}
	if err := session.WriteAtomic(paths.Handoff(), []byte(prepared.Markdown), 0o644); err != nil {
		return shell.Result{ExitCode: 1}, fmt.Errorf("write handoff: %w", err)
	}
	preGit, err := checkpoint.CaptureGit(ctx, root, prepared.Config.Denylist, prepared.Config.Handoff.MaxDiffChars)
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}
	pre, err := checkpoint.CapturePre(ctx, checkpoint.PreOptions{
		Paths:           paths,
		RepoRoot:        root,
		ProviderName:    name,
		HandoffMarkdown: prepared.Markdown,
		State:           prepared.State,
		CommandLog:      prepared.CommandLog,
		Git:             preGit,
		Denylist:        prepared.Config.Denylist,
		MaxDiffChars:    prepared.Config.Handoff.MaxDiffChars,
	})
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}

	command, injection, err := resolveLaunch(providers.Resolve(prepared.Config), name, paths.Handoff(), userArgs)
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

	postDir := paths.CheckpointDir(fmt.Sprintf("%04d-after-%s", pre.Sequence, pre.ProviderName))
	transcriptPath := filepath.Join(postDir, "transcript.ansi")
	transcript, closeTranscript, err := openTranscript(paths, transcriptPath)
	if err != nil {
		return shell.Result{ExitCode: 1}, err
	}
	var closeTranscriptOnce sync.Once
	closeTranscriptSafely := func() {
		closeTranscriptOnce.Do(closeTranscript)
	}
	defer closeTranscriptSafely()

	a.UI.Printf("Prepared handoff and checkpoint %04d\n", pre.Sequence)
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
	closeTranscriptSafely()
	a.UI.Flush()
	post, postErr := checkpoint.CapturePost(ctx, checkpoint.PostOptions{
		Paths:        paths,
		RepoRoot:     root,
		Pre:          pre,
		ExitCode:     res.ExitCode,
		Denylist:     prepared.Config.Denylist,
		MaxDiffChars: prepared.Config.Handoff.MaxDiffChars,
	})
	if postErr != nil {
		return res, PostRunError{Code: res.ExitCode, ProviderErr: err, CheckpointErr: postErr}
	}
	if err != nil {
		return res, err
	}
	a.UI.Printf("Provider %s exited with code %d (files changed: %s)\n", name, res.ExitCode, post.FilesChanged)
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

// openTranscript creates the per-run transcript stream at transcriptPath,
// creating the Session Directory if it does not exist yet (without requiring
// `aictl init`). The transcript is a live output stream, so it is opened
// directly rather than through session.WriteAtomic, which is for atomic
// whole-buffer snapshots, not streams.
func openTranscript(paths session.Paths, transcriptPath string) (io.Writer, func(), error) {
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		return nil, func() {}, fmt.Errorf("create session directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(transcriptPath), 0o755); err != nil {
		return nil, func() {}, fmt.Errorf("create transcript directory: %w", err)
	}
	// If the Session Directory was created lazily (run without a prior `init`),
	// write the ignore file so the secret-bearing transcript is never committed.
	if _, statErr := os.Stat(paths.GitIgnore()); errors.Is(statErr, os.ErrNotExist) {
		if err := session.WriteAtomic(paths.GitIgnore(), []byte(gitignoreContents), 0o644); err != nil {
			return nil, func() {}, fmt.Errorf("write session .gitignore: %w", err)
		}
	}
	// 0o600: the transcript is raw provider output that may contain secrets.
	f, err := os.OpenFile(transcriptPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create transcript: %w", err)
	}
	return f, func() { _ = f.Close() }, nil
}
