package shell

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/term"
)

type terminalStub struct {
	calls        int
	makeRawCalls int
}

func stubTerminal(t *testing.T, terminal bool) *terminalStub {
	t.Helper()
	stub := &terminalStub{}

	oldIsTerminal := isTerminal
	oldMakeRaw := makeRaw
	oldRestore := restoreTerminal
	oldStartPTY := startPTY
	oldInherit := inheritPTYSize

	isTerminal = func(int) bool { return terminal }
	makeRaw = func(int) (*term.State, error) {
		stub.makeRawCalls++
		return &term.State{}, nil
	}
	restoreTerminal = func(int, *term.State) error {
		stub.calls++
		return nil
	}

	t.Cleanup(func() {
		isTerminal = oldIsTerminal
		makeRaw = oldMakeRaw
		restoreTerminal = oldRestore
		startPTY = oldStartPTY
		inheritPTYSize = oldInherit
	})
	return stub
}

func normalizePTYOutput(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func atoi(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}
