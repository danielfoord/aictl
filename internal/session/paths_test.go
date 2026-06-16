package session

import (
	"path/filepath"
	"testing"
)

func TestCheckpointPathHelpers(t *testing.T) {
	paths := NewPaths("/repo")

	dir := paths.CheckpointDir("0001-before-claude")
	wantDir := filepath.Join("/repo", ".ai-session", "checkpoints", "0001-before-claude")
	if dir != wantDir {
		t.Fatalf("CheckpointDir = %q, want %q", dir, wantDir)
	}

	file := paths.CheckpointFile("0001-before-claude", "git-status.txt")
	wantFile := filepath.Join(wantDir, "git-status.txt")
	if file != wantFile {
		t.Fatalf("CheckpointFile = %q, want %q", file, wantFile)
	}
}
