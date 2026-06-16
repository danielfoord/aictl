package verify

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunCapturesPassAndFail(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), []string{"echo hello", "false"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Commands) != 2 {
		t.Fatalf("expected 2 command results, got %d", len(res.Commands))
	}
	if res.AllPassed {
		t.Error("AllPassed should be false when a command exits non-zero")
	}
	if res.Commands[0].ExitCode != 0 {
		t.Errorf("`echo` exit = %d, want 0", res.Commands[0].ExitCode)
	}
	if res.Commands[1].ExitCode == 0 {
		t.Error("`false` should report a non-zero exit code")
	}
	if !strings.Contains(res.Combined, "hello") {
		t.Errorf("combined output should contain command output:\n%s", res.Combined)
	}
}

func TestRunAllPass(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), []string{"true", "echo ok"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.AllPassed {
		t.Error("AllPassed should be true when every command exits 0")
	}
}

func TestRunEmpty(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.AllPassed || len(res.Commands) != 0 {
		t.Errorf("empty run should pass with no commands, got %+v", res)
	}
}

func TestRunStopsOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled
	_, err := Run(ctx, t.TempDir(), []string{"echo hi"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRunSkipsBlankCommands(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), []string{"", "   ", "echo ok"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res.Commands) != 1 || res.Commands[0].Command != "echo ok" {
		t.Fatalf("blank commands should be skipped, got %+v", res.Commands)
	}
	if !res.AllPassed {
		t.Error("AllPassed should be true (blanks skipped, echo passes)")
	}
}

func TestRunPreservesOrder(t *testing.T) {
	res, err := Run(context.Background(), t.TempDir(), []string{"echo one", "echo two"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Commands[0].Command != "echo one" || res.Commands[1].Command != "echo two" {
		t.Errorf("commands should run in order, got %q then %q", res.Commands[0].Command, res.Commands[1].Command)
	}
}
