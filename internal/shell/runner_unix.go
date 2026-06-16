//go:build !windows

package shell

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/creack/pty"
)

var startPTY = pty.Start

var winchSignal os.Signal = syscall.SIGWINCH

// interruptSignals are the signals that should proactively restore the terminal
// before aictl is terminated.
var interruptSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

func exitCode(state *os.ProcessState, waitErr error) (int, error) {
	if waitErr == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return 1, fmt.Errorf("wait for provider: %w", waitErr)
	}
	if state == nil {
		state = exitErr.ProcessState
	}
	if state == nil {
		return 1, fmt.Errorf("wait for provider: %w", waitErr)
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok {
		if status.Signaled() {
			return 128 + int(status.Signal()), nil
		}
		if status.Exited() {
			return status.ExitStatus(), nil
		}
	}
	return state.ExitCode(), nil
}
