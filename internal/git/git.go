// Package git is aictl's interface to the user's repository. It shells out to
// the real `git` binary (rather than a Go git library) so output is byte-for-
// byte what the developer sees, and git is treated as the primary source of
// truth.
//
// It captures the repo state handoffs and checkpoints rely on: current branch
// and clean/dirty status (`Capture`), short status (`Status`), recent commits
// (`RecentCommits`), and the working+staged diff (`Diff`) — the diff is
// secret-redacted and size-bounded by caller-supplied parameters. The package
// is config-agnostic: the denylist and size limit are passed in, not imported.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ErrNotARepo indicates the directory is not inside a git repository. Callers
// can treat it as benign (e.g. `aictl start`'s best-effort context capture).
var ErrNotARepo = errors.New("not a git repository")

// gitTimeout bounds each git subprocess so a hung/prompting git cannot block
// aictl indefinitely. The status/diff/log calls are local and fast.
const gitTimeout = 10 * time.Second

// Snapshot is a minimal capture of repository state.
type Snapshot struct {
	Branch string
	Dirty  bool
}

// run executes a git subcommand in dir (under a timeout) and returns trimmed
// stdout. A "not a git repository" failure is reported as ErrNotARepo.
func run(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	// `-c core.quotePath=false` keeps non-ASCII path bytes un-escaped (so path
	// parsing/redaction is reliable). LC_ALL=C forces English messages, which we
	// classify (ErrNotARepo, unborn-branch).
	full := append([]string{"-c", "core.quotePath=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "not a git repository") {
			return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), ErrNotARepo)
		}
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, msg)
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

// Status returns the short porcelain status (`git status --short`).
func Status(ctx context.Context, dir string) (string, error) {
	return run(ctx, dir, "status", "--short", "--untracked-files=normal")
}

// RecentCommits returns up to n recent commits (`git log -n <n> --oneline`).
// A repository with no commits yet (unborn branch) yields an empty string, not
// an error. n <= 0 defaults to 10.
func RecentCommits(ctx context.Context, dir string, n int) (string, error) {
	if n <= 0 {
		n = 10
	}
	// Detect an unborn branch (no commits yet) deterministically rather than by
	// matching localized git text: rev-parse --verify --quiet exits non-zero
	// with no output when HEAD doesn't resolve.
	if _, err := run(ctx, dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		if errors.Is(err, ErrNotARepo) {
			return "", err
		}
		return "", nil // unborn branch — no commits to report
	}
	return run(ctx, dir, "log", "-n", strconv.Itoa(n), "--oneline")
}

// Diff returns the combined staged + working-tree diff, with file sections
// matching the denylist redacted and the result truncated to maxChars (see
// redactDiff / truncate in diff.go). It uses `git diff --cached` + `git diff`
// rather than `git diff HEAD` so it also works before the first commit.
// Redaction is applied before truncation so a secret can never survive via the
// truncation boundary. maxChars <= 0 means no limit.
func Diff(ctx context.Context, dir string, denylist []string, maxChars int) (string, error) {
	// Verify we're inside a repo first: outside one, `git diff` falls back to
	// --no-index mode and emits a confusing usage error rather than a clean
	// "not a git repository". rev-parse reports it as ErrNotARepo.
	if _, err := run(ctx, dir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", err
	}

	// Force the a/…b/ prefixes so header parsing is immune to the user's
	// diff.noprefix / diff.mnemonicPrefix config.
	staged, err := run(ctx, dir, "diff", "--cached", "--src-prefix=a/", "--dst-prefix=b/")
	if err != nil {
		return "", err
	}
	unstaged, err := run(ctx, dir, "diff", "--src-prefix=a/", "--dst-prefix=b/")
	if err != nil {
		return "", err
	}

	combined := staged
	if combined != "" && unstaged != "" {
		combined += "\n"
	}
	combined += unstaged

	combined = redactDiff(combined, denylist)
	combined = truncate(combined, maxChars)
	return combined, nil
}
