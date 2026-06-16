package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// mustGit runs a git subcommand in dir and fails the test on error.
func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestStatusShowsUntracked(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := Status(context.Background(), dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !strings.Contains(out, "f.txt") {
		t.Fatalf("status should mention the untracked file: %q", out)
	}
}

func TestDiffCapturesStagedOnUnbornRepo(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "main.go")

	out, err := Diff(context.Background(), dir, nil, 0)
	if err != nil {
		t.Fatalf("Diff on an unborn repo should not error: %v", err)
	}
	if !strings.Contains(out, "main.go") {
		t.Fatalf("diff should include the staged file: %q", out)
	}
}

func TestDiffRedactsTrackedSecret(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=topsecret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", ".env")

	out, err := Diff(context.Background(), dir, []string{".env*"}, 0)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if strings.Contains(out, "topsecret") {
		t.Fatalf("secret leaked into diff: %q", out)
	}
	if !strings.Contains(out, "[redacted") {
		t.Fatalf("expected redaction marker: %q", out)
	}
}

func TestDiffTruncatesAtMaxChars(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(strings.Repeat("x\n", 500)), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", "big.txt")

	out, err := Diff(context.Background(), dir, nil, 100)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !strings.Contains(out, "[truncated") {
		t.Fatalf("expected truncation marker for a large diff: len=%d", len(out))
	}
}

func TestRecentCommitsEmptyOnUnborn(t *testing.T) {
	dir := initRepo(t)
	out, err := RecentCommits(context.Background(), dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits on unborn repo should not error: %v", err)
	}
	if out != "" {
		t.Fatalf("expected no commits, got %q", out)
	}
}

func TestRecentCommitsAfterCommit(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, dir, "add", ".")
	mustGit(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-m", "initial commit")

	out, err := RecentCommits(context.Background(), dir, 5)
	if err != nil {
		t.Fatalf("RecentCommits: %v", err)
	}
	if !strings.Contains(out, "initial commit") {
		t.Fatalf("expected the commit message, got %q", out)
	}
}

func TestCaptureFunctionsClassifyNotARepo(t *testing.T) {
	dir := t.TempDir() // deliberately not a git repo
	ctx := context.Background()

	if _, err := Status(ctx, dir); !errors.Is(err, ErrNotARepo) {
		t.Errorf("Status outside a repo: want ErrNotARepo, got %v", err)
	}
	if _, err := Diff(ctx, dir, nil, 0); !errors.Is(err, ErrNotARepo) {
		t.Errorf("Diff outside a repo: want ErrNotARepo, got %v", err)
	}
	if _, err := RecentCommits(ctx, dir, 5); !errors.Is(err, ErrNotARepo) {
		t.Errorf("RecentCommits outside a repo: want ErrNotARepo, got %v", err)
	}
	if _, err := Capture(ctx, dir); !errors.Is(err, ErrNotARepo) {
		t.Errorf("Capture outside a repo: want ErrNotARepo, got %v", err)
	}
}
