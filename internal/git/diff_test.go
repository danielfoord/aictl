package git

import (
	"strings"
	"testing"
	"unicode/utf8"
)

const sampleDiff = `diff --git a/.env b/.env
index 0000000..1111111 100644
--- a/.env
+++ b/.env
@@ -0,0 +1 @@
+SECRET=topsecret
diff --git a/main.go b/main.go
index 2222222..3333333 100644
--- a/main.go
+++ b/main.go
@@ -1 +1 @@
-old
+new
`

func TestRedactDiffStripsDenylistedFile(t *testing.T) {
	out := redactDiff(sampleDiff, []string{".env*", "*.pem"})

	if strings.Contains(out, "topsecret") {
		t.Fatalf("secret leaked through redaction:\n%s", out)
	}
	if !strings.Contains(out, "[redacted: matches secret denylist]") {
		t.Fatalf("expected redaction marker:\n%s", out)
	}
	if !strings.Contains(out, "diff --git a/.env b/.env") {
		t.Error("redaction should preserve the .env header line")
	}
	if !strings.Contains(out, "+new") || !strings.Contains(out, "main.go") {
		t.Errorf("non-denylisted file should be intact:\n%s", out)
	}
}

func TestRedactDiffNoDenylistIsUnchanged(t *testing.T) {
	if got := redactDiff(sampleDiff, nil); got != sampleDiff {
		t.Error("redactDiff with no denylist should return the diff unchanged")
	}
}

func TestMatchesDenylist(t *testing.T) {
	dl := []string{".env*", "*.pem", "*.key", "id_*"}
	for _, name := range []string{".env", ".env.local", "server.pem", "tls.key", "id_rsa"} {
		if !matchesDenylist(name, dl) {
			t.Errorf("%q should match the denylist", name)
		}
	}
	for _, name := range []string{"main.go", "README.md", "env.go"} {
		if matchesDenylist(name, dl) {
			t.Errorf("%q should NOT match the denylist", name)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc\n… [truncated 3 chars]" {
		t.Errorf("truncate over limit = %q", got)
	}
	if got := truncate("abc", 10); got != "abc" {
		t.Errorf("truncate under limit should be unchanged, got %q", got)
	}
	if got := truncate("abc", 0); got != "abc" {
		t.Errorf("truncate with 0 (no limit) should be unchanged, got %q", got)
	}
}

func TestRedactDiffQuotedPath(t *testing.T) {
	diff := `diff --git "a/my secret.pem" "b/my secret.pem"
--- "a/my secret.pem"
+++ "b/my secret.pem"
@@ -0,0 +1 @@
+KEY=leaked
`
	out := redactDiff(diff, []string{"*.pem"})
	if strings.Contains(out, "leaked") {
		t.Fatalf("secret in a space-containing path leaked:\n%s", out)
	}
	if !strings.Contains(out, "[redacted") {
		t.Fatalf("expected redaction of quoted path:\n%s", out)
	}
}

func TestRedactDiffDirectoryGlob(t *testing.T) {
	diff := `diff --git a/secrets/app.json b/secrets/app.json
@@ -0,0 +1 @@
+TOKEN=leaked
`
	out := redactDiff(diff, []string{"secrets/*"})
	if strings.Contains(out, "leaked") {
		t.Fatalf("directory-scoped denylist failed to redact:\n%s", out)
	}
}

func TestRedactDiffRenameMatchesASide(t *testing.T) {
	// A secret renamed to an innocuous path: the a-side must still match.
	diff := `diff --git a/secrets/old.env b/public/new.txt
similarity index 100%
rename from secrets/old.env
rename to public/new.txt
`
	out := redactDiff(diff, []string{"secrets/*"})
	if !strings.Contains(out, "[redacted") {
		t.Fatalf("rename should redact when the a-side path is denylisted:\n%s", out)
	}
}

func TestRedactStatusPaths(t *testing.T) {
	status := " M main.go\n?? .env\nA  keys/server.pem\nR  old.txt -> .env.local"
	out := RedactStatusPaths(status, []string{".env*", "*.pem"})

	if strings.Contains(out, ".env\n") || strings.Contains(out, "server.pem") || strings.Contains(out, ".env.local") {
		t.Fatalf("denylisted path leaked through status redaction:\n%s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("non-denylisted path should be intact:\n%s", out)
	}
	if strings.Count(out, "[redacted]") != 3 {
		t.Errorf("expected 3 redacted status lines, got:\n%s", out)
	}
}

func TestTruncateRuneBoundary(t *testing.T) {
	s := "aé" // 'a' + 'é' (2 bytes) => 3 bytes total
	got := truncate(s, 2)
	if !utf8.ValidString(got) {
		t.Fatalf("truncate produced invalid UTF-8: %q", got)
	}
	if !strings.HasPrefix(got, "a") || strings.Contains(got, "é") {
		t.Fatalf("expected truncation before the multibyte rune, got %q", got)
	}
}
