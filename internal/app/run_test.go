package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/shell"
	"github.com/danielfoord/aictl/internal/ui"
)

// seedGoal makes the session "in progress" so `run` injects the handoff.
func seedGoal(t *testing.T, dir, goal string) {
	t.Helper()
	paths := session.NewPaths(dir)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.SaveState(paths.State(), session.TaskState{Goal: goal}); err != nil {
		t.Fatal(err)
	}
}

func TestRunFreshSessionSkipsInjection(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "claude")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := newTestApp()
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}

	// Fresh session: no goal, no progress.
	if _, err := a.Run(context.Background(), "claude", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got.Args) != 0 {
		t.Fatalf("fresh session must not inject a prompt arg, got %v", got.Args)
	}
	if len(got.InitialInput) != 0 {
		t.Fatalf("fresh session must not set InitialInput, got %q", got.InitialInput)
	}
	// Durability is unchanged: handoff + before-checkpoint still written.
	paths := session.NewPaths(dir)
	if _, err := os.Stat(paths.Handoff()); err != nil {
		t.Fatalf("handoff.md should be written even without injection: %v", err)
	}
	if _, err := os.Stat(paths.CheckpointDir("0001-before-claude")); err != nil {
		t.Fatalf("pre-run checkpoint should be written even without injection: %v", err)
	}
	if _, err := os.Stat(paths.CheckpointDir("0001-after-claude")); err != nil {
		t.Fatalf("post-run checkpoint should be written even without injection: %v", err)
	}
}

func TestRunProgressWithoutGoalInjects(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "claude")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Progress recorded but no goal → still a task to continue.
	paths := session.NewPaths(dir)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.SaveState(paths.State(), session.TaskState{NextSteps: []string{"wire it up"}}); err != nil {
		t.Fatal(err)
	}

	a := newTestApp()
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}
	if _, err := a.Run(context.Background(), "claude", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got.Args) == 0 || !strings.Contains(got.Args[len(got.Args)-1], "handoff.md") {
		t.Fatalf("progress-only session should inject the handoff, got %v", got.Args)
	}
}

func TestRunResolvesBuiltinWithFileRefInjection(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "claude")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	seedGoal(t, dir, "resume the task")

	a := newTestApp()
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}

	if _, err := a.Run(context.Background(), "claude", []string{"--resume"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if filepath.Base(got.Command) != "claude" {
		t.Fatalf("Command = %q, want resolved claude", got.Command)
	}
	if len(got.Args) == 0 || !strings.Contains(got.Args[len(got.Args)-1], "handoff.md") {
		t.Fatalf("expected file-ref handoff prompt as final arg, got %v", got.Args)
	}
	if got.Args[0] != "--resume" {
		t.Fatalf("user arg should precede the prompt, got %v", got.Args)
	}
	if len(got.InitialInput) != 0 {
		t.Fatalf("file-ref must not set InitialInput, got %q", got.InitialInput)
	}
}

func TestRunConfigProviderResolvesThroughRegistry(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	bin := writeExecutable(t, dir, "myai-cli")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	cfg := config.Default()
	cfg.Providers = map[string]config.Provider{
		"myai": {Command: "myai-cli", PromptInjection: config.PromptInjection{Mode: "arg", Text: "do the thing"}},
	}
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	paths := session.NewPaths(dir)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Config(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	seedGoal(t, dir, "resume the task")

	a := newTestApp()
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}

	if _, err := a.Run(context.Background(), "myai", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Command != bin {
		t.Fatalf("Command = %q, want %q", got.Command, bin)
	}
	if got.Args[len(got.Args)-1] != "do the thing" {
		t.Fatalf("arg injection should pass configured text last, got %v", got.Args)
	}
}

func TestRunPasteProviderSetsInitialInput(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "pasteai")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeProviderConfig(t, dir, map[string]config.Provider{
		"pasteai": {Command: "pasteai", PromptInjection: config.PromptInjection{Mode: "paste", Text: "do it"}},
	})
	seedGoal(t, dir, "resume the task")

	a := newTestApp()
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}

	if _, err := a.Run(context.Background(), "pasteai", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if string(got.InitialInput) != "do it\r" {
		t.Fatalf("InitialInput = %q, want \"do it\\r\"", got.InitialInput)
	}
	if len(got.Args) != 0 {
		t.Fatalf("paste mode must not append a prompt arg, got %v", got.Args)
	}
}

func TestRunUnknownNameRunsBareWithoutInjection(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "randomtool")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := newTestApp()
	var got shell.Options
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		got = opts
		return shell.Result{ExitCode: 0}, nil
	}

	if _, err := a.Run(context.Background(), "randomtool", []string{"-x"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(got.InitialInput) != 0 {
		t.Fatalf("unknown name must not inject, got InitialInput %q", got.InitialInput)
	}
	if strings.Join(got.Args, " ") != "-x" {
		t.Fatalf("unknown name should pass only user args, got %v", got.Args)
	}
}

func TestRunInvalidConfigModeFailsBeforeRunner(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "typoai")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	writeProviderConfig(t, dir, map[string]config.Provider{
		"typoai": {Command: "typoai", PromptInjection: config.PromptInjection{Mode: "psate"}},
	})

	a := newTestApp()
	called := false
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		called = true
		return shell.Result{}, nil
	}

	_, err := a.Run(context.Background(), "typoai", nil)
	if err == nil || !strings.Contains(err.Error(), "unknown injection mode") {
		t.Fatalf("error = %v, want unknown injection mode", err)
	}
	if called {
		t.Fatal("runner must not be called when the injection mode is invalid")
	}
}

func writeProviderConfig(t *testing.T, dir string, provs map[string]config.Provider) {
	t.Helper()
	cfg := config.Default()
	cfg.Providers = provs
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	paths := session.NewPaths(dir)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Config(), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunWritesTranscriptFile(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := newTestApp().Start(context.Background(), "checkpoint the run"); err != nil {
		t.Fatalf("Start: %v", err)
	}

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

	data, err := os.ReadFile(filepath.Join(session.NewPaths(dir).Checkpoints(), "0001-after-fake-provider", "transcript.ansi"))
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	if want := "captured \x1b[1moutput\x1b[0m\n"; string(data) != want {
		t.Fatalf("transcript = %q, want %q", data, want)
	}
}

func TestRunCreatesPreAndPostCheckpoints(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := newTestApp().Start(context.Background(), "checkpoint the run"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	a := newTestApp()
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		_, _ = opts.Transcript.Write([]byte("transcript\n"))
		return shell.Result{ExitCode: 0}, nil
	}
	if _, err := a.Run(context.Background(), "fake-provider", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	checkpoints := session.NewPaths(dir).Checkpoints()
	for _, path := range []string{
		filepath.Join(checkpoints, "0001-before-fake-provider", "handoff.md"),
		filepath.Join(checkpoints, "0001-before-fake-provider", "git-status.txt"),
		filepath.Join(checkpoints, "0001-before-fake-provider", "git-diff.patch"),
		filepath.Join(checkpoints, "0001-before-fake-provider", "recent-commits.txt"),
		filepath.Join(checkpoints, "0001-before-fake-provider", "command-log.md"),
		filepath.Join(checkpoints, "0001-before-fake-provider", "summary.md"),
		filepath.Join(checkpoints, "0001-after-fake-provider", "exit-code.txt"),
		filepath.Join(checkpoints, "0001-after-fake-provider", "transcript.ansi"),
		filepath.Join(checkpoints, "0001-after-fake-provider", "recent-commits.txt"),
		filepath.Join(checkpoints, "0001-after-fake-provider", "files-changed.txt"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected checkpoint artifact %s: %v", path, err)
		}
	}
}

func TestRunNoopProviderDoesNotReportOwnHandoffWriteAsFilesChanged(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	if err := newTestApp().Start(context.Background(), "checkpoint cleanly"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	paths := session.NewPaths(dir)
	if err := session.WriteAtomic(paths.Handoff(), []byte("old handoff\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "add", ".ai-session/.gitignore", ".ai-session/config.yaml", ".ai-session/state.yaml", ".ai-session/handoff.md")
	runGit(t, dir, "commit", "-m", "track session")

	a := newTestApp()
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		return shell.Result{ExitCode: 0}, nil
	}
	if _, err := a.Run(context.Background(), "fake-provider", nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(paths.Checkpoints(), "0001-after-fake-provider", "files-changed.txt"))
	if err != nil {
		t.Fatalf("read files-changed.txt: %v", err)
	}
	if strings.TrimSpace(string(data)) != "no" {
		t.Fatalf("files-changed.txt = %q, want no", data)
	}
}

func TestRunPreRunPersistenceFailurePreventsRunner(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	paths := session.NewPaths(dir)
	if err := os.MkdirAll(paths.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := session.SaveState(paths.State(), session.TaskState{Goal: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Checkpoints(), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := newTestApp()
	called := false
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		called = true
		return shell.Result{}, nil
	}
	_, err := a.Run(context.Background(), "fake-provider", nil)
	if err == nil {
		t.Fatal("expected pre-run persistence failure")
	}
	if called {
		t.Fatal("runner must not be called after pre-run persistence failure")
	}
}

func TestRunNonzeroExitStillWritesPostCheckpoint(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := newTestApp().Start(context.Background(), "checkpoint the failure"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	a := newTestApp()
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		return shell.Result{ExitCode: 42}, nil
	}
	res, err := a.Run(context.Background(), "fake-provider", nil)
	if err == nil {
		t.Fatal("expected exit error")
	}
	if res.ExitCode != 42 {
		t.Fatalf("ExitCode = %d, want 42", res.ExitCode)
	}
	data, readErr := os.ReadFile(filepath.Join(session.NewPaths(dir).Checkpoints(), "0001-after-fake-provider", "exit-code.txt"))
	if readErr != nil {
		t.Fatalf("read exit-code.txt: %v", readErr)
	}
	if strings.TrimSpace(string(data)) != "42" {
		t.Fatalf("exit-code.txt = %q", data)
	}
}

func TestRunPostCheckpointFailurePreservesProviderExitCode(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := newTestApp().Start(context.Background(), "checkpoint the failure"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	providerErr := errors.New("provider transport failed")
	a := newTestApp()
	a.runProvider = func(context.Context, shell.Options) (shell.Result, error) {
		exitPath := filepath.Join(session.NewPaths(dir).Checkpoints(), "0001-after-fake-provider", "exit-code.txt")
		if err := os.Mkdir(exitPath, 0o755); err != nil {
			t.Fatalf("force post checkpoint failure: %v", err)
		}
		return shell.Result{ExitCode: 42}, providerErr
	}
	res, err := a.Run(context.Background(), "fake-provider", nil)
	if err == nil {
		t.Fatal("expected post checkpoint error")
	}
	if res.ExitCode != 42 {
		t.Fatalf("ExitCode = %d, want 42", res.ExitCode)
	}
	var exitErr ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 42 {
		t.Fatalf("error = %#v, want wrapped ExitError{42}", err)
	}
	if !errors.Is(err, providerErr) {
		t.Fatalf("error = %#v, want wrapped provider error", err)
	}
}

func TestRunIsQuietDuringProviderRun(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	writeExecutable(t, dir, "fake-provider")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := newTestApp().Start(context.Background(), "quiet checkpoint"); err != nil {
		t.Fatalf("Start: %v", err)
	}

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
	if err := newTestApp().Start(context.Background(), "panic checkpoint"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	var out bytes.Buffer
	a := New(ui.New(&out, &out))
	a.runProvider = func(_ context.Context, opts shell.Options) (shell.Result, error) {
		a.UI.Printf("buffered-before-panic\n")
		_, _ = opts.Transcript.Write([]byte("before panic\n"))
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
		data, err := os.ReadFile(filepath.Join(session.NewPaths(dir).Checkpoints(), "0001-after-fake-provider", "transcript.ansi"))
		if err != nil {
			t.Fatalf("read panic transcript: %v", err)
		}
		if string(data) != "before panic\n" {
			t.Fatalf("panic transcript = %q", data)
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
	paths := session.NewPaths(dir)
	if _, err := os.Stat(paths.Checkpoints()); err != nil {
		t.Fatalf("run-created checkpoints dir: %v", err)
	}
	if _, err := os.Stat(paths.State()); err != nil {
		t.Fatalf("run-created state file: %v", err)
	}
}

func TestRunMissingProviderFailsBeforeRunner(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
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
	if err := newTestApp().Start(context.Background(), "delegate checkpoint"); err != nil {
		t.Fatalf("Start: %v", err)
	}

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
	if err := newTestApp().Start(context.Background(), "exit checkpoint"); err != nil {
		t.Fatalf("Start: %v", err)
	}

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

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
