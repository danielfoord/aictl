package handoff

import (
	"strings"
	"testing"
)

func sampleRecoveryInput() RecoveryInput {
	return RecoveryInput{
		Goal:   "Refactor the classifier to support schema versioning",
		Branch: "feature/schema",
		Diff:   "diff --git a/classifier.go b/classifier.go\n@@ -1 +1 @@\n-old\n+new",
	}
}

func TestGenerateRecoveryDeterministic(t *testing.T) {
	a, err := GenerateRecovery(sampleRecoveryInput())
	if err != nil {
		t.Fatalf("GenerateRecovery: %v", err)
	}
	b, err := GenerateRecovery(sampleRecoveryInput())
	if err != nil {
		t.Fatalf("GenerateRecovery: %v", err)
	}
	if a != b {
		t.Fatal("GenerateRecovery is not deterministic for identical input")
	}
}

func TestGenerateRecoveryIncludesMinimalEvidence(t *testing.T) {
	got, err := GenerateRecovery(sampleRecoveryInput())
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"# Recover This Coding Task",
		"Refactor the classifier",
		"feature/schema",
		"classifier.go",
		"do not restart",
		"Inspect the changed files",
		"Preserve existing user work",
		"Run verification",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("recovery prompt missing %q:\n%s", want, got)
		}
	}
}

func TestGenerateRecoveryOmitsFullHandoffSections(t *testing.T) {
	got, err := GenerateRecovery(sampleRecoveryInput())
	if err != nil {
		t.Fatal(err)
	}

	for _, forbidden := range []string{
		"Latest Verification Output",
		"Recent Commands",
		"Recent Commits",
		"Decisions",
		"Known Failures",
		"Next Steps",
	} {
		if strings.Contains(got, forbidden) {
			t.Errorf("recovery prompt should not include full-handoff-only section %q:\n%s", forbidden, got)
		}
	}
}

func TestGenerateRecoveryShowsEmptyDiffMarker(t *testing.T) {
	got, err := GenerateRecovery(RecoveryInput{Goal: "goal", Branch: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "(no uncommitted changes)") {
		t.Fatalf("empty diff marker missing:\n%s", got)
	}
}

func TestGenerateRecoveryUsesFenceLongerThanContent(t *testing.T) {
	got, err := GenerateRecovery(RecoveryInput{
		Goal:   "goal with ``` fence",
		Branch: "main",
		Diff:   "diff --git a/f b/f\n+```",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "````\ngoal with ``` fence\n````") {
		t.Fatalf("goal should be contained in a longer fence:\n%s", got)
	}
	if !strings.Contains(got, "````diff\ndiff --git a/f b/f\n+```\n````") {
		t.Fatalf("diff should be contained in a longer diff fence:\n%s", got)
	}
}
