package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/danielfoord/aictl/internal/session"
	"github.com/danielfoord/aictl/internal/ui"
)

func TestStateCommandsAppendToMappedFields(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := a.Note(context.Background(), " choose yaml "); err != nil {
		t.Fatalf("Note: %v", err)
	}
	if err := a.Done(context.Background(), " wrote tests "); err != nil {
		t.Fatalf("Done: %v", err)
	}
	if err := a.Next(context.Background(), " implement commands "); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if err := a.Fail(context.Background(), " bad state "); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got, want := state.Decisions, []string{"choose yaml"}; !sameStrings(got, want) {
		t.Errorf("Decisions = %v, want %v", got, want)
	}
	if got, want := state.Completed, []string{"wrote tests"}; !sameStrings(got, want) {
		t.Errorf("Completed = %v, want %v", got, want)
	}
	if got, want := state.NextSteps, []string{"implement commands"}; !sameStrings(got, want) {
		t.Errorf("NextSteps = %v, want %v", got, want)
	}
	if got, want := state.KnownFailures, []string{"bad state"}; !sameStrings(got, want) {
		t.Errorf("KnownFailures = %v, want %v", got, want)
	}
}

func TestStateCommandsAccumulateAndRoundTripThroughYAML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := a.Note(context.Background(), "first"); err != nil {
		t.Fatalf("first Note: %v", err)
	}
	if err := a.Note(context.Background(), "second"); err != nil {
		t.Fatalf("second Note: %v", err)
	}

	data, err := os.ReadFile(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if !strings.Contains(string(data), "decisions:") || !strings.Contains(string(data), "- first") {
		t.Fatalf("state.yaml does not look hand-editable:\n%s", data)
	}

	state, err := session.UnmarshalTaskState(data)
	if err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	if got, want := state.Decisions, []string{"first", "second"}; !sameStrings(got, want) {
		t.Errorf("Decisions = %v, want %v", got, want)
	}
}

func TestStateCommandsRequireSession(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	err := newTestApp().Note(context.Background(), "remember this")
	if !errors.Is(err, ErrNoSession) {
		t.Fatalf("Note error = %v, want ErrNoSession", err)
	}
}

func TestStateCommandsPreserveExistingEntries(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	paths := session.NewPaths(dir)
	if err := session.SaveState(paths.State(), session.TaskState{
		Completed:     []string{"provider completed"},
		Decisions:     []string{"provider decision"},
		NextSteps:     []string{"provider next"},
		KnownFailures: []string{"provider failure"},
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	if err := a.Done(context.Background(), "user completed"); err != nil {
		t.Fatalf("Done: %v", err)
	}

	state, err := session.LoadState(paths.State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if got, want := state.Completed, []string{"provider completed", "user completed"}; !sameStrings(got, want) {
		t.Errorf("Completed = %v, want %v", got, want)
	}
	if got, want := state.Decisions, []string{"provider decision"}; !sameStrings(got, want) {
		t.Errorf("Decisions = %v, want %v", got, want)
	}
	if got, want := state.NextSteps, []string{"provider next"}; !sameStrings(got, want) {
		t.Errorf("NextSteps = %v, want %v", got, want)
	}
	if got, want := state.KnownFailures, []string{"provider failure"}; !sameStrings(got, want) {
		t.Errorf("KnownFailures = %v, want %v", got, want)
	}
}

func TestStateCommandsRejectEmptyValue(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := a.Next(context.Background(), " \t\n "); err == nil {
		t.Fatal("expected empty value error")
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.NextSteps) != 0 {
		t.Fatalf("NextSteps = %v, want empty", state.NextSteps)
	}
}

func TestStateCommandsDoNotOverwriteMalformedState(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	statePath := session.NewPaths(dir).State()
	bad := []byte("completed: [\n")
	if err := os.WriteFile(statePath, bad, 0o644); err != nil {
		t.Fatalf("seed malformed state: %v", err)
	}

	if err := a.Fail(context.Background(), "known failure"); err == nil {
		t.Fatal("expected malformed state error")
	}
	got, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if string(got) != string(bad) {
		t.Fatalf("malformed state was overwritten:\n%s", got)
	}
}

func TestStateCommandsPrintConfirmation(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	var out bytes.Buffer
	a := New(ui.New(&out, &bytes.Buffer{}))

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	out.Reset()

	if err := a.Note(context.Background(), "keep it"); err != nil {
		t.Fatalf("Note: %v", err)
	}
	if got, want := out.String(), "noted decision: \"keep it\"\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}

func TestStateCommandHonorsCanceledContext(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := a.Note(ctx, "should not persist")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Note error = %v, want context.Canceled", err)
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Decisions) != 0 {
		t.Fatalf("Decisions = %v, want empty", state.Decisions)
	}
}

func TestStateCommandsSerializeConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	a := newTestApp()

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	const count = 20
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- a.Note(context.Background(), string(rune('a'+i)))
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Note: %v", err)
		}
	}

	state, err := session.LoadState(session.NewPaths(dir).State())
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if len(state.Decisions) != count {
		t.Fatalf("Decisions length = %d, want %d: %v", len(state.Decisions), count, state.Decisions)
	}
	seen := make(map[string]bool, count)
	for _, decision := range state.Decisions {
		seen[decision] = true
	}
	for i := 0; i < count; i++ {
		want := string(rune('a' + i))
		if !seen[want] {
			t.Fatalf("Decisions missing %q: %v", want, state.Decisions)
		}
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
