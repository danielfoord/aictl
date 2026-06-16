// Package providers resolves the provider a user asked to run into a concrete
// command + prompt-injection plan. Built-in adapters (Claude, Codex, Gemini)
// are overlaid by config into one resolved map; callers look up that map and
// never branch on "built-in vs config".
package providers

// Mode is how the handoff prompt is delivered to a launched provider.
type Mode string

const (
	// ModeFileRef injects a short instruction referencing .ai-session/handoff.md
	// as a command-line argument. It is the default.
	ModeFileRef Mode = "file-ref"
	// ModeArg injects the configured prompt text as a command-line argument.
	ModeArg Mode = "arg"
	// ModeStdin writes the prompt into the provider's PTY after launch.
	ModeStdin Mode = "stdin"
	// ModePaste writes the prompt into the PTY after launch (paste fallback for
	// providers that cannot take a prompt argument).
	ModePaste Mode = "paste"
)

// Valid reports whether m is a recognized injection mode. It is used to reject a
// mistyped config mode at resolve time rather than silently delivering the
// prompt as a file-ref argument.
func (m Mode) Valid() bool {
	switch m {
	case ModeFileRef, ModeArg, ModeStdin, ModePaste:
		return true
	default:
		return false
	}
}

// Provider is a fully resolved provider adapter: built-in defaults overlaid by
// config. UsageLimitPatterns are carried as data only in Story 3.3; detection
// that uses them arrives in Epic 4.
type Provider struct {
	Name               string
	Command            string
	Args               []string
	Mode               Mode
	PromptText         string
	UsageLimitPatterns []string
}
