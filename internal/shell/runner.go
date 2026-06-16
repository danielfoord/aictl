// Package shell owns provider process execution through a pseudo-terminal.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// initialInputDelay gives an interactive TUI time to finish initializing before
// injected prompt bytes (paste/stdin modes) are written into the PTY.
const initialInputDelay = 250 * time.Millisecond

// Options describes one supervised interactive process run.
type Options struct {
	Command string
	Args    []string
	Dir     string
	Env     []string
	Stdin   *os.File
	Stdout  io.Writer
	// Stderr is currently unused: a pseudo-terminal merges the child's stdout
	// and stderr onto the single PTY master, which is mirrored to Stdout. The
	// field is retained for the documented Options shape and a possible future
	// non-PTY execution path.
	Stderr io.Writer
	// Transcript, when non-nil, receives a byte-identical copy of the PTY output
	// stream (raw ANSI) via the fan-out writer. A transcript write failure never
	// degrades the on-screen passthrough. Nil means terminal-only mirroring.
	Transcript io.Writer
	// InitialInput, when non-empty, is written into the PTY a short delay after
	// the child starts (used for paste/stdin prompt injection, Story 3.3). The
	// write is best-effort and never aborts the run. Empty means no injection.
	InitialInput []byte
}

// Result is the provider process result after the PTY has been cleaned up.
type Result struct {
	ExitCode int
}

// Run starts the configured command in a PTY, passes stdin through to it, and
// mirrors the PTY output to Stdout.
func Run(ctx context.Context, opts Options) (res Result, err error) {
	if opts.Command == "" {
		return Result{ExitCode: 1}, errors.New("run provider: command is required")
	}
	stdin := opts.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}

	restore := func() {}
	defer func() {
		restore()
		if r := recover(); r != nil {
			panic(r)
		}
	}()

	restore, err = enterRawMode(stdin)
	if err != nil {
		return Result{ExitCode: 1}, fmt.Errorf("enter raw mode: %w", err)
	}

	// Restore the terminal immediately on an interrupt/termination signal so a
	// hard-termination path (or a second interrupt) cannot leave it stuck in
	// raw mode. restore is idempotent, so this coexists with the deferred call.
	stopSignals := watchTerminalRestore(restore, nil)
	defer stopSignals()

	cmd := exec.CommandContext(ctx, opts.Command, opts.Args...)
	if opts.Dir != "" {
		cmd.Dir = opts.Dir
	}
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}

	ptmx, err := startPTY(cmd)
	if err != nil {
		return Result{ExitCode: 1}, fmt.Errorf("start provider pty: %w", err)
	}
	defer func() { _ = ptmx.Close() }()

	stopResize := startResizeWatcher(stdin, ptmx, nil)
	defer stopResize()

	// Single-write fan-out: PTY output is mirrored to the terminal once and teed
	// to the transcript (and, from Story 4.1, the usage detector) — no extra
	// buffering that could reorder bytes or degrade the TUI (NFR-2).
	out := newFanWriter(stdout, opts.Transcript)
	outputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(out, ptmx)
		outputDone <- copyErr
	}()

	// Prompt injection (paste/stdin): write the prompt into the PTY once, a short
	// delay after launch so the provider's TUI is ready. Best-effort — a write
	// error is ignored and never aborts the run. injectDone is closed when the
	// child exits so a fast-exiting provider doesn't leave this goroutine parked
	// on the timer or write into an already-closed PTY.
	injectDone := make(chan struct{})
	if len(opts.InitialInput) > 0 {
		go func() {
			select {
			case <-time.After(initialInputDelay):
				_, _ = ptmx.Write(opts.InitialInput)
			case <-ctx.Done():
			case <-injectDone:
			}
		}()
	}

	go func() {
		_, _ = io.Copy(ptmx, stdin)
	}()

	waitErr := cmd.Wait()
	close(injectDone)
	_ = ptmx.Close()
	<-outputDone

	code, mapErr := exitCode(cmd.ProcessState, waitErr)
	if mapErr != nil {
		return Result{ExitCode: 1}, mapErr
	}
	return Result{ExitCode: code}, nil
}
