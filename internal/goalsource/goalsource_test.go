package goalsource

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseStructuredYAML(t *testing.T) {
	c := Parse("goal: Ship the goal source\nnextSteps:\n  - write the reader\n  - wire the command\ndecisions:\n  - reuse the verify shell-out pattern\n")
	if c.Goal != "Ship the goal source" {
		t.Fatalf("Goal = %q", c.Goal)
	}
	if len(c.NextSteps) != 2 || c.NextSteps[0] != "write the reader" {
		t.Fatalf("NextSteps = %v", c.NextSteps)
	}
	if len(c.Decisions) != 1 {
		t.Fatalf("Decisions = %v", c.Decisions)
	}
}

func TestParsePlainTextBecomesGoal(t *testing.T) {
	c := Parse("  Implement the thing  \n")
	if c.Goal != "Implement the thing" {
		t.Fatalf("Goal = %q, want trimmed plain text", c.Goal)
	}
	if len(c.NextSteps) != 0 {
		t.Fatalf("plain text should not yield structured fields, got %v", c.NextSteps)
	}
}

func TestParseStructuredWithoutGoalKeepsFields(t *testing.T) {
	c := Parse("nextSteps:\n  - first step\n  - second step\n")
	if c.Goal != "" {
		t.Fatalf("Goal = %q, want empty (no goal: key), not the raw YAML", c.Goal)
	}
	if len(c.NextSteps) != 2 || c.NextSteps[0] != "first step" {
		t.Fatalf("NextSteps = %v, want the structured steps preserved", c.NextSteps)
	}
}

func TestParseEmpty(t *testing.T) {
	if c := Parse("   \n  "); c.Goal != "" {
		t.Fatalf("empty input should yield empty goal, got %q", c.Goal)
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "TASK.md"), []byte("goal: from file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, ok, err := Read(context.Background(), dir, "TASK.md", "")
	if err != nil || !ok {
		t.Fatalf("Read file = (%q, %v, %v)", text, ok, err)
	}
	if Parse(text).Goal != "from file" {
		t.Fatalf("parsed goal = %q", Parse(text).Goal)
	}
}

func TestReadCommand(t *testing.T) {
	text, ok, err := Read(context.Background(), t.TempDir(), "", "printf 'goal: from command\\n'")
	if err != nil || !ok {
		t.Fatalf("Read command = (%q, %v, %v)", text, ok, err)
	}
	if Parse(text).Goal != "from command" {
		t.Fatalf("parsed goal = %q", Parse(text).Goal)
	}
}

func TestReadCommandFailureIncludesStderr(t *testing.T) {
	_, ok, err := Read(context.Background(), t.TempDir(), "", "echo 'boom detail' >&2; exit 3")
	if ok || err == nil {
		t.Fatalf("expected command failure, got (ok=%v, err=%v)", ok, err)
	}
	if !contains(err.Error(), "boom detail") {
		t.Fatalf("error should include stderr detail, got %q", err.Error())
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestReadFilePrecedesCommand(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "g.txt"), []byte("file wins"), 0o644); err != nil {
		t.Fatal(err)
	}
	text, ok, _ := Read(context.Background(), dir, "g.txt", "echo command")
	if !ok || Parse(text).Goal != "file wins" {
		t.Fatalf("file should take precedence, got %q", text)
	}
}

func TestReadUnset(t *testing.T) {
	_, ok, err := Read(context.Background(), t.TempDir(), "", "")
	if err != nil || ok {
		t.Fatalf("unset source should be (false, nil), got (%v, %v)", ok, err)
	}
}
