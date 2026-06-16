package providers

// codex is the built-in adapter for the Codex CLI.
func codex() Provider {
	return Provider{
		Name:               "codex",
		Command:            "codex",
		Mode:               ModeFileRef,
		UsageLimitPatterns: []string{"usage limit", "quota exceeded", "try again in", "billing"},
	}
}
