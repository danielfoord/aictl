package config

// DefaultMaxDiffChars bounds the diff embedded in a handoff by default (FR-13).
const DefaultMaxDiffChars = 30000

// DefaultDenylist is the built-in set of path globs whose contents are stripped
// from generated artifacts to avoid leaking secrets.
var DefaultDenylist = []string{".env*", "*.pem", "*.key", "id_*"}

// Default returns the built-in configuration written by `aictl init`. Providers
// and verify commands start empty; they are populated by the user or by later
// stories.
func Default() Config {
	return Config{
		Providers: map[string]Provider{},
		Verify:    []string{},
		Handoff:   Handoff{MaxDiffChars: DefaultMaxDiffChars},
		Denylist:  append([]string(nil), DefaultDenylist...),
	}
}
