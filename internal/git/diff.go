package git

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

// redactDiff strips the body of any per-file section in a unified diff whose
// a-side OR b-side path matches the denylist, preserving the `diff --git`
// header so the file is visibly present-but-redacted. This is the single
// secret-stripping point; everything downstream consumes the result.
func redactDiff(diff string, denylist []string) string {
	if diff == "" || len(denylist) == 0 {
		return diff
	}

	var b strings.Builder
	for i, sec := range splitDiffSections(diff) {
		if i > 0 {
			b.WriteByte('\n')
		}
		aPath, bPath := sectionPaths(sec)
		if matchesDenylist(aPath, denylist) || matchesDenylist(bPath, denylist) {
			header := sec
			if nl := strings.IndexByte(sec, '\n'); nl >= 0 {
				header = sec[:nl]
			}
			b.WriteString(header)
			b.WriteString("\n[redacted: matches secret denylist]")
			continue
		}
		b.WriteString(sec)
	}
	return b.String()
}

// splitDiffSections splits a unified diff into per-file sections, each starting
// at a `diff --git ` line. (Real diff body lines are prefixed with space/+/-, so
// only genuine headers begin with `diff --git ` at column 0.)
func splitDiffSections(diff string) []string {
	lines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	var sections []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			sections = append(sections, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, ln := range lines {
		if strings.HasPrefix(ln, "diff --git ") {
			flush()
		}
		cur = append(cur, ln)
	}
	flush()
	return sections
}

// sectionPaths extracts the (a-side, b-side) repo-relative paths from a
// section's `diff --git <a> <b>` header, handling git's quoted form for paths
// with spaces/special characters. Either may be "" if unparseable.
func sectionPaths(section string) (string, string) {
	first := section
	if nl := strings.IndexByte(section, '\n'); nl >= 0 {
		first = section[:nl]
	}
	const prefix = "diff --git "
	if !strings.HasPrefix(first, prefix) {
		return "", ""
	}
	rest := first[len(prefix):]
	a, rest := scanPath(rest)
	b, _ := scanPath(strings.TrimLeft(rest, " "))
	return stripSidePrefix(a), stripSidePrefix(b)
}

// scanPath reads one path token (git double-quotes paths with special chars)
// from the front of s and returns it plus the remainder.
func scanPath(s string) (string, string) {
	if s == "" {
		return "", ""
	}
	if s[0] == '"' {
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++ // skip the escaped char
				continue
			}
			if s[i] == '"' {
				tok := s[:i+1]
				if uq, err := strconv.Unquote(tok); err == nil {
					return uq, s[i+1:]
				}
				return strings.Trim(tok, `"`), s[i+1:]
			}
		}
		return strings.Trim(s, `"`), "" // no closing quote — take the rest
	}
	if sp := strings.IndexByte(s, ' '); sp >= 0 {
		return s[:sp], s[sp:]
	}
	return s, ""
}

// stripSidePrefix removes a leading a/ or b/ diff prefix.
func stripSidePrefix(p string) string {
	if strings.HasPrefix(p, "a/") || strings.HasPrefix(p, "b/") {
		return p[2:]
	}
	return p
}

// RedactStatusPaths redacts the path of any `git status --short` line whose
// file matches the denylist, keeping the two-column status code so the change
// is still visible. This mirrors the diff redaction so a secret *filename* isn't
// disclosed via the status section. Rename lines (`old -> new`) match either path.
func RedactStatusPaths(status string, denylist []string) string {
	if status == "" || len(denylist) == 0 {
		return status
	}
	lines := strings.Split(status, "\n")
	for i, ln := range lines {
		if len(ln) < 4 { // "XY p"
			continue
		}
		code, path := ln[:3], ln[3:] // ln[:2] status, ln[2] = ' '
		redact := false
		if oldP, newP, ok := strings.Cut(path, " -> "); ok {
			redact = matchesDenylist(strings.TrimSpace(oldP), denylist) ||
				matchesDenylist(strings.TrimSpace(newP), denylist)
		} else {
			redact = matchesDenylist(strings.TrimSpace(path), denylist)
		}
		if redact {
			lines[i] = code + "[redacted]"
		}
	}
	return strings.Join(lines, "\n")
}

// matchesDenylist reports whether path matches any denylist glob, tested against
// both the full repo-relative path (so directory-scoped globs like `secrets/*`
// work) and the basename (so `*.pem` works regardless of directory).
func matchesDenylist(path string, denylist []string) bool {
	if path == "" {
		return false
	}
	base := filepath.Base(path)
	for _, glob := range denylist {
		if ok, _ := filepath.Match(glob, path); ok {
			return true
		}
		if ok, _ := filepath.Match(glob, base); ok {
			return true
		}
	}
	return false
}

// truncate caps s at maxChars bytes, backing off to a UTF-8 rune boundary so it
// never emits an invalid byte sequence, and appends a marker noting how many
// bytes were dropped. maxChars <= 0 means no limit.
func truncate(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	cut := maxChars
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	dropped := len(s) - cut
	return s[:cut] + fmt.Sprintf("\n… [truncated %d chars]", dropped)
}
