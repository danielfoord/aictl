package handoff

import (
	"bytes"
	"fmt"
	"strings"
)

// RecoveryInput is the minimal deterministic input for a last-resort
// continuation prompt. Callers assemble it from Task State plus current git
// state; recovery rendering never reads the filesystem, clock, or environment.
type RecoveryInput struct {
	Goal   string
	Branch string
	Diff   string
}

// GenerateRecovery renders a minimal continuation prompt from only the original
// Goal, current branch, and current diff. Identical input yields byte-identical
// output.
func GenerateRecovery(in RecoveryInput) (string, error) {
	var b bytes.Buffer

	fmt.Fprintf(&b, "# Recover This Coding Task\n\n")
	fmt.Fprintf(&b, "You are continuing an in-progress coding task from minimal local state. Do not restart it from scratch.\n\n")
	fmt.Fprintf(&b, "## Goal\n%s\n\n", fencedBlock("", valueOrFallback(in.Goal, "(goal unavailable)")))
	fmt.Fprintf(&b, "## Current Repository State\n")
	fmt.Fprintf(&b, "Branch: %s\n\n", valueOrFallback(in.Branch, "(unknown / detached)"))
	fmt.Fprintf(&b, "### Diff\n")
	if in.Diff == "" {
		fmt.Fprintf(&b, "(no uncommitted changes)\n\n")
	} else {
		fmt.Fprintf(&b, "%s\n\n", fencedBlock("diff", in.Diff))
	}
	fmt.Fprintf(&b, "## Recovery Instructions\n")
	fmt.Fprintf(&b, "- Continue from the current repository state; do not restart the task from scratch.\n")
	fmt.Fprintf(&b, "- Inspect the changed files before editing.\n")
	fmt.Fprintf(&b, "- Preserve existing user work and avoid destructive git operations.\n")
	fmt.Fprintf(&b, "- Make the smallest safe change that advances the Goal.\n")
	fmt.Fprintf(&b, "- Run verification before finishing when verification commands are known.\n")

	return b.String(), nil
}

func valueOrFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func fencedBlock(info, content string) string {
	fence := strings.Repeat("`", longestBacktickRun(content)+1)
	if len(fence) < 3 {
		fence = "```"
	}
	if info != "" {
		return fmt.Sprintf("%s%s\n%s\n%s", fence, info, content, fence)
	}
	return fmt.Sprintf("%s\n%s\n%s", fence, content, fence)
}

func longestBacktickRun(s string) int {
	longest, current := 0, 0
	for _, r := range s {
		if r == '`' {
			current++
			if current > longest {
				longest = current
			}
			continue
		}
		current = 0
	}
	return longest
}
