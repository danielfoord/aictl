package shell

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunPassesInputMirrorsOutputAndPreservesExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix PTY story excludes Windows")
	}

	in, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := in.WriteString("hello from stdin\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := in.Seek(0, 0); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Command: os.Args[0],
		Args: []string{
			"-test.run=TestHelperProcess",
			"--",
			"echo-then-exit",
			"42",
		},
		Env:    append(os.Environ(), "GO_WANT_HELPER_PROCESS=1"),
		Stdin:  in,
		Stdout: &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 42 {
		t.Fatalf("ExitCode = %d, want 42", res.ExitCode)
	}
	got := normalizePTYOutput(out.String())
	for _, want := range []string{"helper ready", "echo: hello from stdin"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output missing %q:\n%s", want, got)
		}
	}
}

func TestRunMapsSignalExitTo128PlusSignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix signal semantics story excludes Windows")
	}

	var out bytes.Buffer
	res, err := Run(context.Background(), Options{
		Command: os.Args[0],
		Args: []string{
			"-test.run=TestHelperProcess",
			"--",
			"self-sigterm",
		},
		Env:    append(os.Environ(), "GO_WANT_HELPER_PROCESS=1"),
		Stdout: &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode != 143 {
		t.Fatalf("ExitCode = %d, want 143", res.ExitCode)
	}
}

func TestRunRestoresRawModeOnceOnStartError(t *testing.T) {
	restore := stubTerminal(t, true)
	startPTY = func(*exec.Cmd) (*os.File, error) {
		return nil, errors.New("start failed")
	}

	_, err := Run(context.Background(), Options{Command: os.Args[0]})
	if err == nil {
		t.Fatal("expected start error")
	}
	if restore.calls != 1 {
		t.Fatalf("restore calls = %d, want 1", restore.calls)
	}
}

func TestRunRestoresRawModeBeforeRepanic(t *testing.T) {
	restore := stubTerminal(t, true)
	startPTY = func(*exec.Cmd) (*os.File, error) {
		panic("boom")
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		if restore.calls != 1 {
			t.Fatalf("restore calls = %d, want 1", restore.calls)
		}
	}()

	_, _ = Run(context.Background(), Options{Command: os.Args[0]})
}

func TestRunSkipsRawModeForNonTerminalStdin(t *testing.T) {
	restore := stubTerminal(t, false)
	startPTY = func(*exec.Cmd) (*os.File, error) {
		return nil, errors.New("start failed")
	}

	_, err := Run(context.Background(), Options{Command: os.Args[0]})
	if err == nil {
		t.Fatal("expected start error")
	}
	if restore.makeRawCalls != 0 || restore.calls != 0 {
		t.Fatalf("raw mode calls = %d restore calls = %d, want both 0", restore.makeRawCalls, restore.calls)
	}
}

func TestRunRestoresRawModeOnContextCancellation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix PTY story excludes Windows")
	}
	restore := stubTerminal(t, true)

	in, err := os.CreateTemp(t.TempDir(), "stdin-*")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	var out bytes.Buffer
	res, err := Run(ctx, Options{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--", "sleep"},
		Env:     append(os.Environ(), "GO_WANT_HELPER_PROCESS=1"),
		Stdin:   in,
		Stdout:  &out,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.ExitCode == 0 {
		t.Fatal("ExitCode = 0, want non-zero after context-cancelled kill")
	}
	if restore.calls != 1 {
		t.Fatalf("restore calls = %d, want 1", restore.calls)
	}
}

func TestWatchTerminalRestoreRestoresOnSignal(t *testing.T) {
	restored := make(chan struct{}, 1)
	restore := func() {
		select {
		case restored <- struct{}{}:
		default:
		}
	}
	sigs := make(chan os.Signal, 1)
	stop := watchTerminalRestore(restore, sigs)
	defer stop()

	sigs <- os.Interrupt
	select {
	case <-restored:
	case <-time.After(time.Second):
		t.Fatal("terminal not restored after interrupt signal")
	}
}

func TestStartResizeWatcherInheritsInitialAndSignalSize(t *testing.T) {
	var calls atomic.Int32
	inheritPTYSize = func(_, _ *os.File) error {
		calls.Add(1)
		return nil
	}
	sig := make(chan os.Signal, 1)
	stop := startResizeWatcher(os.Stdin, os.Stdout, sig)
	defer stop()

	sig <- winchSignal
	time.Sleep(50 * time.Millisecond)

	if got := calls.Load(); got < 2 {
		t.Fatalf("inherit size calls = %d, want initial + signal", got)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}

	switch args[1] {
	case "echo-then-exit":
		code := 0
		if len(args) > 2 {
			code = atoi(args[2])
		}
		_, _ = os.Stdout.WriteString("helper ready\n")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		line = strings.TrimRight(line, "\r\n")
		_, _ = os.Stdout.WriteString("echo: " + line + "\n")
		os.Exit(code)
	case "self-sigterm":
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(sigtermSignal)
		time.Sleep(time.Second)
		os.Exit(2)
	case "sleep":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
