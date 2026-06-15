// Package git is aictl's interface to the user's repository. It shells out to
// the real `git` binary (rather than a Go git library) so output is byte-for-
// byte what the developer sees, and git is treated as the primary source of
// truth.
//
// This file is the minimal slice needed by `aictl start` (current branch and
// clean/dirty status). The diff/commits/secret-denylist capture used by
// handoffs and checkpoints is added in Story 2.1.
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Snapshot is a minimal capture of repository state.
type Snapshot struct {
	Branch string
	Dirty  bool
}

// run executes a git subcommand in dir and returns trimmed stdout.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Branch returns the current branch name, or "" for a detached HEAD.
func Branch(ctx context.Context, dir string) (string, error) {
	return run(ctx, dir, "branch", "--show-current")
}

// IsDirty reports whether the working tree has uncommitted changes (including
// untracked files).
func IsDirty(ctx context.Context, dir string) (bool, error) {
	out, err := run(ctx, dir, "status", "--porcelain", "--untracked-files=normal")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// Capture returns a minimal Snapshot of repo state. It returns an error if dir
// is not a git repository or git is unavailable.
func Capture(ctx context.Context, dir string) (Snapshot, error) {
	branch, err := Branch(ctx, dir)
	if err != nil {
		return Snapshot{}, err
	}
	dirty, err := IsDirty(ctx, dir)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Branch: branch, Dirty: dirty}, nil
}
