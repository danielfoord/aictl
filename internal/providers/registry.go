package providers

import "github.com/danielfoord/aictl/internal/config"

// builtins returns a fresh map of the built-in Trio adapters.
func builtins() map[string]Provider {
	return map[string]Provider{
		"claude": claude(),
		"codex":  codex(),
		"gemini": gemini(),
	}
}

// Resolve overlays cfg.Providers onto the built-in Trio, producing the single
// resolved provider map. A config entry for an existing built-in overrides only
// the non-empty fields it sets (others keep their built-in default); an entry
// for a new name adds a provider. Callers use the resolved map and never branch
// on built-in vs config.
func Resolve(cfg config.Config) map[string]Provider {
	resolved := builtins()
	for name, cp := range cfg.Providers {
		base, ok := resolved[name]
		if !ok {
			base = Provider{Name: name, Mode: ModeFileRef}
		}
		resolved[name] = overlay(base, cp)
	}
	return resolved
}

// overlay applies a config Provider's non-empty fields onto a base provider.
func overlay(base Provider, cp config.Provider) Provider {
	if cp.Command != "" {
		base.Command = cp.Command
	}
	if len(cp.Args) > 0 {
		base.Args = append([]string(nil), cp.Args...)
	}
	if len(cp.UsageLimitPatterns) > 0 {
		base.UsageLimitPatterns = append([]string(nil), cp.UsageLimitPatterns...)
	}
	if cp.PromptInjection.Mode != "" {
		base.Mode = Mode(cp.PromptInjection.Mode)
	}
	if cp.PromptInjection.Text != "" {
		base.PromptText = cp.PromptInjection.Text
	}
	return base
}

// Lookup returns the resolved provider for name, if present.
func Lookup(resolved map[string]Provider, name string) (Provider, bool) {
	p, ok := resolved[name]
	return p, ok
}
