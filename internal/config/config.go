// Package config defines aictl's user-facing configuration contract
// (`.ai-session/config.yaml`) and its built-in defaults.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

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

// Load reads the Config at path. A missing file yields the built-in defaults so
// callers always get a usable config. A present config is normalized so an
// empty denylist or non-positive maxDiffChars can't silently disable the
// privacy/size guardrails (use a deliberate, documented opt-out instead).
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	c, err := Unmarshal(data)
	if err != nil {
		return Config{}, err
	}
	return c.normalized(), nil
}

// normalized fills in safe defaults for guardrail fields left empty/zero, so a
// partial config never silently disables secret redaction or the diff bound.
func (c Config) normalized() Config {
	if len(c.Denylist) == 0 {
		c.Denylist = append([]string(nil), DefaultDenylist...)
	}
	if c.Handoff.MaxDiffChars <= 0 {
		c.Handoff.MaxDiffChars = DefaultMaxDiffChars
	}
	return c
}
