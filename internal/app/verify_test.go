package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/config"
	"github.com/danielfoord/aictl/internal/session"
)

func writeConfigVerify(t *testing.T, dir string, cmds []string) {
	t.Helper()
	cfg := config.Default()
	cfg.Verify = cmds
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(session.NewPaths(dir).Config(), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFileT(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestVerifyWritesOutputAndLog(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()
	if err := a.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeConfigVerify(t, dir, []string{"echo building", "echo testing"})

	if err := a.Verify(context.Background()); err != nil {
		t.Fatalf("verify: %v", err)
	}

	paths := session.NewPaths(dir)
	out := readFileT(t, paths.LatestVerify())
	if !strings.Contains(out, "building") || !strings.Contains(out, "testing") {
		t.Errorf("latest-verify.txt missing output:\n%s", out)
	}
	logData := readFileT(t, paths.CommandLog())
	if !strings.Contains(logData, "echo building") {
		t.Errorf("command-log.md missing entry:\n%s", logData)
	}
}

func TestVerifyFailingCommandStillCapturesAndErrors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()
	if err := a.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeConfigVerify(t, dir, []string{"echo before", "false"})

	if err := a.Verify(context.Background()); err == nil {
		t.Fatal("expected an error when a verify command fails")
	}
	out := readFileT(t, session.NewPaths(dir).LatestVerify())
	if !strings.Contains(out, "before") {
		t.Errorf("output should still be captured even when a command fails:\n%s", out)
	}
}

func TestVerifyRequiresSession(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := newTestApp().Verify(context.Background()); !errors.Is(err, ErrNoSession) {
		t.Fatalf("want ErrNoSession, got %v", err)
	}
}

func TestVerifyNoCommandsConfigured(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()
	if err := a.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	// Default config has an empty verify list.
	if err := a.Verify(context.Background()); err != nil {
		t.Fatalf("verify with no commands should not error: %v", err)
	}
	if _, err := os.Stat(session.NewPaths(dir).LatestVerify()); !errors.Is(err, os.ErrNotExist) {
		t.Error("no verify output file should be written when nothing is configured")
	}
}

func TestVerifyOutputFeedsHandoff(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	t.Chdir(dir)
	a := newTestApp()
	if err := a.Start(context.Background(), "goal"); err != nil {
		t.Fatal(err)
	}
	writeConfigVerify(t, dir, []string{"echo VERIFYMARKER"})
	if err := a.Verify(context.Background()); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := a.Handoff(context.Background()); err != nil {
		t.Fatalf("handoff: %v", err)
	}
	h := readFileT(t, session.NewPaths(dir).Handoff())
	if !strings.Contains(h, "VERIFYMARKER") {
		t.Errorf("handoff should embed the verify output:\n%s", h)
	}
}
