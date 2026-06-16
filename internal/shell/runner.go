// Package shell owns provider process execution through a pseudo-terminal.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

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

	outputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stdout, ptmx)
		outputDone <- copyErr
	}()

	go func() {
		_, _ = io.Copy(ptmx, stdin)
	}()

	waitErr := cmd.Wait()
	_ = ptmx.Close()
	<-outputDone

	code, mapErr := exitCode(cmd.ProcessState, waitErr)
	if mapErr != nil {
		return Result{ExitCode: 1}, mapErr
	}
	return Result{ExitCode: code}, nil
}
