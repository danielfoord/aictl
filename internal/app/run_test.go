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

	"github.com/danielfoord/aictl/internal/shell"
	"github.com/danielfoord/aictl/internal/ui"
)

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
