package handoff

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func sampleInput() Input {
	return Input{
		Goal:          "Refactor the classifier to support schema versioning",
		Branch:        "feature/schema",
		Status:        " M classifier.go\n?? notes.txt",
		Diff:          "diff --git a/classifier.go b/classifier.go\n@@ -1 +1 @@\n-old\n+new",
		RecentCommits: "abc123 add schema version\ndef456 wip",
		VerifyOutput:  "ok  classifier  0.2s",
		CommandLog:    "go test ./...",
		NextSteps:     []string{"update tests", "run go test"},
		Decisions:     []string{"schema versions are immutable"},
		KnownFailures: []string{"TestLatestSchema failing"},
	}
}

func TestGenerateDeterministic(t *testing.T) {
	a, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if a != b {
		t.Fatal("Generate is not deterministic for identical input")
	}
}

func TestGenerateGolden(t *testing.T) {
	got, err := Generate(sampleInput())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	golden := filepath.Join("testdata", "handoff_golden.md")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run `go test -run TestGenerateGolden -update` to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("handoff does not match golden:\n--- got ---\n%s", got)
	}
}

func TestGenerateHasAntiRedoInstructions(t *testing.T) {
	got, err := Generate(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"do not restart", "Inspect the changed files", "Run the verification"} {
		if !strings.Contains(got, phrase) {
			t.Errorf("handoff missing anti-redo phrase %q", phrase)
		}
	}
}

func TestGenerateSparseInputOmitsEmptySections(t *testing.T) {
	got, err := Generate(Input{Goal: "just a goal"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "just a goal") {
		t.Error("goal missing from sparse handoff")
	}
	for _, section := range []string{"Recent Commits", "Latest Verification Output", "Recent Commands", "Next Steps"} {
		if strings.Contains(got, section) {
			t.Errorf("empty section %q should be omitted from a sparse handoff", section)
		}
	}
}
