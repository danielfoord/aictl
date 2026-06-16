//go:build windows

package shell

import (
	"errors"
	"os"
	"os/exec"
)

var startPTY = func(*exec.Cmd) (*os.File, error) {
	return nil, errors.New("pty runner is unsupported on windows")
}

var winchSignal os.Signal = os.Interrupt

// interruptSignals are the signals that should proactively restore the terminal
// before aictl is terminated.
var interruptSignals = []os.Signal{os.Interrupt}

func exitCode(state *os.ProcessState, waitErr error) (int, error) {
	if waitErr == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		return 1, waitErr
	}
	return exitErr.ExitCode(), nil
}
