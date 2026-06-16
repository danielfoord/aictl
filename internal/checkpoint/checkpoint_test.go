package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/session"
)

func TestSafeProviderName(t *testing.T) {
	tests := map[string]string{
		"claude":               "claude",
		"../custom/provider":   "..-custom-provider",
		"codex beta":           "codex-beta",
		"gemini@nightly":       "gemini-nightly",
		"":                     "provider",
		"../../":               "..-..",
		"provider.with-dash_1": "provider.with-dash_1",
	}
	for in, want := range tests {
		if got := SafeProviderName(in); got != want {
			t.Fatalf("SafeProviderName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextSequenceScansExistingCheckpointDirs(t *testing.T) {
	paths := session.NewPaths(t.TempDir())
	for _, name := range []string{
		"0001-before-claude",
		"0001-after-claude",
		"0007-after-codex",
		"10000-before-gemini",
		"notes",
		"abcd-before-nope",
	} {
		if err := os.MkdirAll(paths.CheckpointDir(name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := NextSequence(paths)
	if err != nil {
		t.Fatalf("NextSequence: %v", err)
	}
	if got != 10001 {
		t.Fatalf("NextSequence = %d, want 10001", got)
	}
}

func TestCapturePreAvoidsExistingSequence(t *testing.T) {
	root := t.TempDir()
	paths := session.NewPaths(root)
	if err := os.MkdirAll(paths.CheckpointDir("0001-before-claude"), 0o755); err != nil {
		t.Fatal(err)
	}

	pre, err := CapturePre(context.Background(), PreOptions{
		Paths:        paths,
		ProviderName: "claude",
	})
	if err != nil {
		t.Fatalf("CapturePre: %v", err)
	}
	if pre.Sequence != 2 || filepath.Base(pre.Dir) != "0002-before-claude" {
		t.Fatalf("pre checkpoint = %d %q, want 2/0002-before-claude", pre.Sequence, filepath.Base(pre.Dir))
	}
}

func TestCapturePreWritesExpectedArtifacts(t *testing.T) {
	root := t.TempDir()
	paths := session.NewPaths(root)
	if err := os.MkdirAll(paths.Checkpoints(), 0o755); err != nil {
		t.Fatal(err)
	}

	pre, err := CapturePre(context.Background(), PreOptions{
		Paths:           paths,
		RepoRoot:        root,
		Sequence:        1,
		ProviderName:    "claude",
		HandoffMarkdown: "# Handoff\n",
		State:           session.TaskState{Goal: "Ship checkpointing", NextSteps: []string{"run tests"}},
		CommandLog:      "go test ./...\n",
	})
	if err != nil {
		t.Fatalf("CapturePre: %v", err)
	}
	if filepath.Base(pre.Dir) != "0001-before-claude" {
		t.Fatalf("pre dir = %q", pre.Dir)
	}
	for _, name := range []string{"handoff.md", "git-status.txt", "git-diff.patch", "recent-commits.txt", "command-log.md", "summary.md"} {
		if _, err := os.Stat(filepath.Join(pre.Dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	summary, err := os.ReadFile(filepath.Join(pre.Dir, "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(summary), "Ship checkpointing") {
		t.Fatalf("summary missing goal:\n%s", summary)
	}
}

func TestCapturePostWritesExpectedArtifactsAndVerdict(t *testing.T) {
	root := t.TempDir()
	paths := session.NewPaths(root)
	if err := os.MkdirAll(paths.Checkpoints(), 0o755); err != nil {
		t.Fatal(err)
	}
	pre := Pre{Sequence: 2, ProviderName: "codex", Git: GitState{Available: true, Status: " M a.go", Diff: "diff --git a/a.go b/a.go"}}

	post, err := CapturePost(context.Background(), PostOptions{
		Paths:      paths,
		RepoRoot:   root,
		Pre:        pre,
		ExitCode:   42,
		Transcript: []byte("raw ansi"),
		PostGit:    GitState{Available: true, Status: " M a.go\n M b.go", Diff: "diff --git a/b.go b/b.go"},
	})
	if err != nil {
		t.Fatalf("CapturePost: %v", err)
	}
	if filepath.Base(post.Dir) != "0002-after-codex" {
		t.Fatalf("post dir = %q", post.Dir)
	}
	for _, name := range []string{"exit-code.txt", "transcript.ansi", "git-status.txt", "git-diff.patch", "recent-commits.txt", "transcript-ref.txt", "files-changed.txt"} {
		if _, err := os.Stat(filepath.Join(post.Dir, name)); err != nil {
			t.Fatalf("expected %s: %v", name, err)
		}
	}
	if post.FilesChanged != FilesChangedYes {
		t.Fatalf("FilesChanged = %q, want %q", post.FilesChanged, FilesChangedYes)
	}
	data, err := os.ReadFile(filepath.Join(post.Dir, "files-changed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "yes" {
		t.Fatalf("files-changed.txt = %q", data)
	}
}

func TestCapturePostPersistsUnknownVerdictWhenGitCaptureFails(t *testing.T) {
	root := t.TempDir()
	paths := session.NewPaths(root)
	if err := os.MkdirAll(paths.Checkpoints(), 0o755); err != nil {
		t.Fatal(err)
	}
	notDir := filepath.Join(root, "not-dir")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	post, err := CapturePost(context.Background(), PostOptions{
		Paths:    paths,
		RepoRoot: notDir,
		Pre:      Pre{Sequence: 3, ProviderName: "codex", Git: GitState{Available: true}},
		ExitCode: 0,
	})
	if err != nil {
		t.Fatalf("CapturePost: %v", err)
	}
	if post.FilesChanged != FilesChangedUnknown {
		t.Fatalf("FilesChanged = %q, want %q", post.FilesChanged, FilesChangedUnknown)
	}
	data, err := os.ReadFile(filepath.Join(post.Dir, "files-changed.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(data)) != "unknown" {
		t.Fatalf("files-changed.txt = %q", data)
	}
}

func TestFilesChangedVerdict(t *testing.T) {
	tests := []struct {
		name string
		pre  GitState
		post GitState
		want FilesChanged
	}{
		{name: "same", pre: GitState{Available: true, Status: " M a", Diff: "d"}, post: GitState{Available: true, Status: " M a", Diff: "d"}, want: FilesChangedNo},
		{name: "different status", pre: GitState{Available: true}, post: GitState{Available: true, Status: " M a"}, want: FilesChangedYes},
		{name: "different commits", pre: GitState{Available: true, RecentCommits: "a before"}, post: GitState{Available: true, RecentCommits: "b after"}, want: FilesChangedYes},
		{name: "unknown", pre: GitState{Available: false}, post: GitState{Available: true}, want: FilesChangedUnknown},
		{name: "redacted marker unchanged", pre: GitState{Available: true, Status: " M [redacted]", Diff: "diff --git a/.env b/.env\n[redacted: matches secret denylist]"}, post: GitState{Available: true, Status: " M [redacted]", Diff: "diff --git a/.env b/.env\n[redacted: matches secret denylist]"}, want: FilesChangedUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineFilesChanged(tt.pre, tt.post); got != tt.want {
				t.Fatalf("DetermineFilesChanged = %q, want %q", got, tt.want)
			}
		})
	}
}
