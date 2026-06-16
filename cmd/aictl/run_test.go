package main

import (
	"errors"
	"os"
	"testing"

	"github.com/danielfoord/aictl/internal/app"
)

func TestRunCommandRequiresProvider(t *testing.T) {
	_, _, run := newTestCmd("dev")
	if err := run("run"); err == nil {
		t.Fatal("expected missing provider argument error")
	}
}

func TestRunCommandPassesProviderArgsAndReturnsExitError(t *testing.T) {
	// Run the real `run` command in a temp dir so the per-run transcript is not
	// written into the source tree.
	t.Chdir(t.TempDir())
	t.Setenv("GO_WANT_RUN_PROVIDER_HELPER", "1")
	_, _, run := newTestCmd("dev")

	err := run("run", os.Args[0], "--", "-test.run=TestRunProviderHelper", "--", "arg-exit", "37")
	if err == nil {
		t.Fatal("expected provider exit error")
	}
	var exitErr app.ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 37 {
		t.Fatalf("error = %#v, want ExitError{37}", err)
	}
}

func TestExitCodeFromError(t *testing.T) {
	if got, ok := exitCodeFromError(app.ExitError{Code: 42}); !ok || got != 42 {
		t.Fatalf("exitCodeFromError = %d, %v; want 42, true", got, ok)
	}
	if got, ok := exitCodeFromError(app.PostRunError{Code: 37, CheckpointErr: errors.New("checkpoint failed")}); !ok || got != 37 {
		t.Fatalf("post-run error exit = %d, %v; want 37, true", got, ok)
	}
	if got, ok := exitCodeFromError(errors.New("ordinary")); ok || got != 1 {
		t.Fatalf("ordinary error exit = %d, %v; want 1, false", got, ok)
	}
}

func TestRunProviderHelper(t *testing.T) {
	if os.Getenv("GO_WANT_RUN_PROVIDER_HELPER") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) == 3 && args[1] == "arg-exit" && args[2] == "37" {
		os.Exit(37)
	}
	os.Exit(2)
}
