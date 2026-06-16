package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/app"
	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/ui"
)

// newTestCmd builds the root command wired to in-memory buffers so tests can
// assert on output.
func newTestCmd(version string) (*bytes.Buffer, *bytes.Buffer, func(args ...string) error) {
	var out, errBuf bytes.Buffer
	cmd := newRootCmd(app.New(ui.New(&out, &errBuf)), version)
	run := func(args ...string) error {
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	return &out, &errBuf, run
}

func TestRootVersion(t *testing.T) {
	out, _, run := newTestCmd("1.2.3")
	if err := run("--version"); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "1.2.3") {
		t.Fatalf("--version output %q does not contain version %q", got, "1.2.3")
	}
}

func TestRootHelpMentionsProgram(t *testing.T) {
	out, _, run := newTestCmd("dev")
	if err := run("--help"); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "aictl") {
		t.Fatalf("--help output %q does not mention the program name", got)
	}
}

func TestResolveVersionPrefersLdflags(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "9.9.9"
	if got := resolveVersion(); got != "9.9.9" {
		t.Fatalf("resolveVersion() = %q, want ldflags value %q", got, "9.9.9")
	}
}

func TestStateCommandsPersistThroughRootCommand(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	out, _, run := newTestCmd("dev")

	if err := run("init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	out.Reset()

	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"note", "choose yaml"}, "noted decision: \"choose yaml\"\n"},
		{[]string{"done", "wrote tests"}, "recorded completed step: \"wrote tests\"\n"},
		{[]string{"next", "wire commands"}, "recorded next step: \"wire commands\"\n"},
		{[]string{"fail", "bad state"}, "recorded known failure: \"bad state\"\n"},
	} {
		out.Reset()
		if err := run(tc.args...); err != nil {
			t.Fatalf("%v: %v", tc.args, err)
		}
		if got := out.String(); got != tc.want {
			t.Fatalf("%v output = %q, want %q", tc.args, got, tc.want)
		}
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got, want := state.Decisions, []string{"choose yaml"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Decisions = %v, want %v", got, want)
	}
	if got, want := state.Completed, []string{"wrote tests"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Completed = %v, want %v", got, want)
	}
	if got, want := state.NextSteps, []string{"wire commands"}; !reflect.DeepEqual(got, want) {
		t.Errorf("NextSteps = %v, want %v", got, want)
	}
	if got, want := state.KnownFailures, []string{"bad state"}; !reflect.DeepEqual(got, want) {
		t.Errorf("KnownFailures = %v, want %v", got, want)
	}
}

func TestStateCommandsRequireExactlyOneArg(t *testing.T) {
	_, _, run := newTestCmd("dev")

	for _, args := range [][]string{
		{"note"},
		{"done", "one", "two"},
		{"next"},
		{"fail", "one", "two"},
	} {
		if err := run(args...); err == nil {
			t.Fatalf("%v: expected argument error", args)
		}
	}
}

func TestRootHelpMentionsCheckpointCommand(t *testing.T) {
	out, _, run := newTestCmd("dev")
	if err := run("--help"); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "checkpoint") {
		t.Fatalf("--help output %q does not mention checkpoint command", got)
	}
}

func TestCheckpointCommandRequiresExactlyOneLabel(t *testing.T) {
	_, _, run := newTestCmd("dev")
	for _, args := range [][]string{
		{"checkpoint"},
		{"checkpoint", "one", "two"},
	} {
		if err := run(args...); err == nil {
			t.Fatalf("%v: expected argument error", args)
		}
	}
}

func TestCheckpointCommandWritesManualCheckpoint(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	t.Chdir(dir)
	out, _, run := newTestCmd("dev")

	if err := run("start", "checkpoint through root"); err != nil {
		t.Fatalf("start: %v", err)
	}
	out.Reset()

	if err := run("checkpoint", "../Before risky refactor"); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}
	if !strings.Contains(out.String(), "Wrote checkpoint 0001-checkpoint-Before-risky-refactor") {
		t.Fatalf("checkpoint output missing success message: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(session.NewPaths(dir).Checkpoints(), "0001-checkpoint-Before-risky-refactor", "summary.md")); err != nil {
		t.Fatalf("expected manual checkpoint summary: %v", err)
	}
}

func TestRecoverCommandWritesRecoveryPrompt(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "-C", dir, "init").Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}
	t.Chdir(dir)
	out, _, run := newTestCmd("dev")

	if err := run("start", "recover this task"); err != nil {
		t.Fatalf("start: %v", err)
	}
	out.Reset()

	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if addOut, err := exec.Command("git", "-C", dir, "add", "main.go").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, addOut)
	}

	if err := run("recover"); err != nil {
		t.Fatalf("recover: %v", err)
	}
	if !strings.Contains(out.String(), "Wrote recovery prompt") {
		t.Fatalf("recover output missing success message: %q", out.String())
	}
	data, err := os.ReadFile(session.NewPaths(dir).Recovery())
	if err != nil {
		t.Fatalf("read recovery prompt: %v", err)
	}
	if !strings.Contains(string(data), "recover this task") {
		t.Fatalf("recovery prompt missing goal:\n%s", data)
	}
}
