package providers

// claude is the built-in adapter for Anthropic's Claude Code CLI.
func claude() Provider {
	return Provider{
		Name:               "claude",
		Command:            "claude",
		Mode:               ModeFileRef,
		UsageLimitPatterns: []string{"usage limit", "try again"},
	}
}
