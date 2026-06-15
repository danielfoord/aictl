package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/app"
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
