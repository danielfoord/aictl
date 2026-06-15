// Package config defines aictl's user-facing configuration contract
// (`.ai-session/config.yaml`) and its built-in defaults.
package config

import "github.com/goccy/go-yaml"

// Config is aictl's configuration, persisted as `.ai-session/config.yaml`. It
// is a public contract: YAML keys are camelCase with explicit struct tags.
type Config struct {
	DefaultProvider string              `yaml:"defaultProvider"`
	Providers       map[string]Provider `yaml:"providers"`
	Verify          []string            `yaml:"verify"`
	Handoff         Handoff             `yaml:"handoff"`
	// Denylist are file path globs whose contents are stripped from generated
	// diffs/handoffs to avoid leaking secrets (used by the git layer, Story 2.1).
	Denylist []string `yaml:"denylist"`
}

// Provider is a provider adapter definition. It is expanded in later stories
// (prompt-injection modes, etc.); the fields here are the v1 essentials.
type Provider struct {
	Command            string   `yaml:"command"`
	Args               []string `yaml:"args"`
	UsageLimitPatterns []string `yaml:"usageLimitPatterns"`
}

// Handoff holds handoff-generation options.
type Handoff struct {
	// MaxDiffChars bounds the git diff embedded in a handoff (FR-13).
	MaxDiffChars int `yaml:"maxDiffChars"`
}

// Marshal serializes the Config to YAML.
func (c Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

// Unmarshal parses a Config from YAML.
func Unmarshal(data []byte) (Config, error) {
	var c Config
	err := yaml.Unmarshal(data, &c)
	return c, err
}
