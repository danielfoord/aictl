package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/shell"
	"github.com/danielfoord/aictl/internal/ui"
)

func TestRunWritesTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := newTestApp()
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		if opts.Transcript == nil {
			t.Error("runner did not receive a transcript writer")
			return shell.Result{ExitCode: 1}, nil
		}
		_, _ = opts.Transcript.Write([]byte("captured \x1b[1moutput\x1b[0m\n"))
		return shell.Result{ExitCode: 0}, nil
	}

	if _, err := a.Run(context.Background(), "fake-provider", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(session.NewPaths(dir).Transcript())
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if want := "captured \x1b[1moutput\x1b[0m\n"; string(data) != want {
		t.Fatalf("transcript = %q, want %q", data, want)
	}
}

func TestRunIsQuietDuringProviderRun(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	a := New(ui.New(&out, &out))
	a.runProvider = func(_ context.Context, _ shell.Options) (shell.Result, error) {
		// aictl attempting to speak mid-run must be buffered, not shown.
		a.UI.Printf("MIDRUN-NOTICE\n")
		if strings.Contains(out.String(), "MIDRUN-NOTICE") {
			t.Error("mid-run output leaked to the terminal while the provider owned the screen")
		}
		return shell.Result{ExitCode: 0}, nil
	}

	if _, err := a.Run(context.Background(), "fake-provider", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	final := out.String()
	last := -1
	for _, s := range []string{"Launching fake-provider", "MIDRUN-NOTICE", "exited with code 0"} {
		idx := strings.Index(final, s)
		if idx < 0 {
			t.Fatalf("missing %q in output:\n%s", s, final)
		}
		if idx < last {
			t.Fatalf("output out of order around %q:\n%s", s, final)
		}
		last = idx
	}
}

func TestRunFlushesUIEvenIfRunnerPanics(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	a := New(ui.New(&out, &out))
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		a.UI.Printf("buffered-before-panic\n")
		panic("boom")
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic to propagate")
		}
		// The deferred Flush must have run during unwinding: UI unmuted and the
		// buffered notice emitted, not lost.
		if !strings.Contains(out.String(), "buffered-before-panic") {
			t.Fatalf("buffered output not flushed after panic: %q", out.String())
		}
	}()

	_, _ = a.Run(context.Background(), "fake-provider", nil)
}

func TestRunWithoutInitWritesGitignore(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := newTestApp()
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		return shell.Result{ExitCode: 0}, nil
	}
	if _, err := a.Run(context.Background(), "fake-provider", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(session.NewPaths(dir).GitIgnore())
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), "transcript.ansi") {
		t.Fatalf("run-created .gitignore missing transcript.ansi:\n%s", data)
	}
}

func TestRunMissingProviderFailsBeforeRunner(t *testing.T) {
	a := newTestApp()
	called := false
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		called = true
		return shell.Result{}, nil
	}

	_, err := a.Run(context.Background(), "definitely-not-aictl-provider", nil)
	if err == nil {
		t.Fatal("expected missing provider error")
	}
	if called {
		t.Fatal("runner should not be called when provider is missing")
	}
	if !strings.Contains(err.Error(), "provider executable not found") {
		t.Fatalf("error = %v", err)
	}
}

func TestRunDelegatesResolvedProviderAndArgs(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	provider := writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out bytes.Buffer
	a := New(ui.New(&out, &out))
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}

	res, err := a.Run(context.Background(), "fake-provider", []string{"--one", "two"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("ExitCode = %d, want 0", res.ExitCode)
	}
	if got.Command != provider {
		t.Fatalf("Command = %q, want %q", got.Command, provider)
	}
	if strings.Join(got.Args, " ") != "--one two" {
		t.Fatalf("Args = %v", got.Args)
	}
	if !strings.Contains(out.String(), "Launching fake-provider") || !strings.Contains(out.String(), "Provider fake-provider exited with code 0") {
		t.Fatalf("missing pre/post output:\n%s", out.String())
	}
}

func TestRunReturnsTypedExitCodeError(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := newTestApp()
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		return shell.Result{ExitCode: 42}, nil
	}

	res, err := a.Run(context.Background(), "fake-provider", nil)
	if err == nil {
		t.Fatal("expected exit-code error")
	}
	if res.ExitCode != 42 {
		t.Fatalf("ExitCode = %d, want 42", res.ExitCode)
	}
	var exitErr ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 42 {
		t.Fatalf("error = %#v, want ExitError{42}", err)
	}
}

func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	body := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		path += ".bat"
		body = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
