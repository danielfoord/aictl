package guard

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// bannedImports are networking packages that aictl's own packages must never
// import. aictl is a supervisor of local CLIs: it shells out to git and
// provider binaries (os/exec) but makes no network/LLM calls itself (NFR-1).
// Keep this list in sync with the depguard rule in .golangci.yml.
var bannedImports = map[string]string{
	"net":        "aictl makes no network calls (NFR-1)",
	"net/http":   "aictl makes no network calls (NFR-1)",
	"net/url":    "aictl makes no network calls (NFR-1)",
	"net/rpc":    "aictl makes no network calls (NFR-1)",
	"crypto/tls": "aictl makes no network calls (NFR-1)",
}

// bannedPrefixes catches whole banned module trees (e.g. golang.org/x/net/...).
var bannedPrefixes = []string{"golang.org/x/net"}

func bannedReason(imp string) (string, bool) {
	if reason, ok := bannedImports[imp]; ok {
		return reason, true
	}
	for _, p := range bannedPrefixes {
		if imp == p || strings.HasPrefix(imp, p+"/") {
			return "aictl makes no network calls (NFR-1)", true
		}
	}
	return "", false
}

// TestNoNetworkImports asserts that no package in the aictl module directly
// imports a networking package. It complements the golangci-lint depguard rule
// and runs anywhere `go` is available (dev and CI).
func TestNoNetworkImports(t *testing.T) {
	cmd := exec.Command(
		"go", "list",
		"-f", "{{.ImportPath}} {{join .Imports \" \"}}",
		"github.com/danielfoord/aictl/...",
	)
	// Output() captures stdout only; stderr stays separate so go list errors
	// can't be mis-parsed as package/import lines.
	stdout, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			t.Fatalf("go list failed: %v\nstderr:\n%s", err, exitErr.Stderr)
		}
		t.Fatalf("go list failed: %v", err)
	}

	scanned := 0
	for _, line := range strings.Split(strings.TrimSpace(string(stdout)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		scanned++
		pkg := fields[0]
		for _, imp := range fields[1:] {
			if reason, banned := bannedReason(imp); banned {
				t.Errorf("package %s imports banned package %q: %s", pkg, imp, reason)
			}
		}
	}

	// Guard against a vacuously-green run (e.g. a pattern that matched nothing).
	if scanned == 0 {
		t.Fatal("no aictl packages were scanned — the no-network guard would pass vacuously")
	}
}
