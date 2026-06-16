// Package verify runs a session's configured verification commands and captures
// their output so a handoff can carry the real build/test state. It shells out
// via `sh -c` and treats a non-zero command exit as a recorded outcome (the
// point of test-aware handoffs), not an execution error.
package verify

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// CommandResult is the outcome of a single verify command.
type CommandResult struct {
	Command  string
	Output   string // combined stdout+stderr
	ExitCode int
}

// Result aggregates a verify run.
type Result struct {
	Commands  []CommandResult
	Combined  string // human-readable, all commands concatenated
	AllPassed bool
}

// Run executes each command via `sh -c` in dir, capturing combined stdout+stderr.
// A non-zero exit is recorded in the corresponding CommandResult and flips
// AllPassed; Run returns an error only when a command cannot be executed at all
// (e.g. the shell is missing or ctx is canceled).
func Run(ctx context.Context, dir string, commands []string) (Result, error) {
	res := Result{AllPassed: true}
	blocks := make([]string, 0, len(commands))

	for _, command := range commands {
		if strings.TrimSpace(command) == "" {
			continue // skip blank/whitespace-only entries
		}
		if err := ctx.Err(); err != nil {
			return res, err // canceled/timed out before this command
		}

		c := exec.CommandContext(ctx, "sh", "-c", command)
		c.Dir = dir
		out, err := c.CombinedOutput()

		cr := CommandResult{Command: command, Output: string(out)}
		if err != nil {
			// A process killed by ctx cancellation/timeout reports exit -1; that's
			// a canceled run, not a test failure — stop and surface it as such.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return res, ctxErr
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				cr.ExitCode = exitErr.ExitCode()
			} else {
				// Could not execute (e.g. `sh` missing): record it as a failed
				// command rather than discarding everything captured so far.
				cr.ExitCode = -1
				if cr.Output != "" && !strings.HasSuffix(cr.Output, "\n") {
					cr.Output += "\n"
				}
				cr.Output += fmt.Sprintf("aictl: could not execute command: %v", err)
			}
			res.AllPassed = false
		}
		res.Commands = append(res.Commands, cr)

		body := cr.Output
		if body != "" && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		blocks = append(blocks, fmt.Sprintf("$ %s (exit %d)\n%s", command, cr.ExitCode, body))
	}

	res.Combined = strings.Join(blocks, "\n")
	return res, nil
}
