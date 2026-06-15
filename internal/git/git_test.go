package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a fresh git repository in a temp dir and returns its path.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	return dir
}

func TestBranchReturnsCurrentBranch(t *testing.T) {
	dir := initRepo(t)
	branch, err := Branch(context.Background(), dir)
	if err != nil {
		t.Fatalf("Branch: %v", err)
	}
	if branch == "" {
		t.Fatal("expected a non-empty branch name for a fresh repo")
	}
}

func TestIsDirtyDetectsUntrackedFile(t *testing.T) {
	dir := initRepo(t)

	clean, err := IsDirty(context.Background(), dir)
	if err != nil {
		t.Fatalf("IsDirty (clean): %v", err)
	}
	if clean {
		t.Fatal("fresh empty repo should not be dirty")
	}

	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	dirty, err := IsDirty(context.Background(), dir)
	if err != nil {
		t.Fatalf("IsDirty (dirty): %v", err)
	}
	if !dirty {
		t.Fatal("repo with an untracked file should be dirty")
	}
}

func TestCaptureOnFreshRepo(t *testing.T) {
	dir := initRepo(t)
	snap, err := Capture(context.Background(), dir)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if snap.Branch == "" {
		t.Error("expected a branch name")
	}
	if snap.Dirty {
		t.Error("fresh repo should be clean")
	}
}

func TestCaptureOutsideRepoErrors(t *testing.T) {
	dir := t.TempDir() // not a git repo
	if _, err := Capture(context.Background(), dir); err == nil {
		t.Fatal("expected an error outside a git repository")
	}
}
