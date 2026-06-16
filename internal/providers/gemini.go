package providers

// gemini is the built-in adapter for the Gemini CLI.
func gemini() Provider {
	return Provider{
		Name:               "gemini",
		Command:            "gemini",
		Mode:               ModeFileRef,
		UsageLimitPatterns: []string{"quota", "rate limit"},
	}
}
