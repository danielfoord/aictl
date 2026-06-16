package providers

import (
	"strings"
	"testing"

	"github.com/danielfoord/aictl/internal/config"
)

func TestBuiltinsHaveTrioDefaults(t *testing.T) {
	resolved := Resolve(config.Default())
	for _, name := range []string{"claude", "codex", "gemini"} {
		p, ok := Lookup(resolved, name)
		if !ok {
			t.Fatalf("built-in %q missing from resolved registry", name)
		}
		if p.Command != name {
			t.Errorf("%s Command = %q, want %q", name, p.Command, name)
		}
		if p.Mode != ModeFileRef {
			t.Errorf("%s Mode = %q, want file-ref", name, p.Mode)
		}
		if len(p.UsageLimitPatterns) == 0 {
			t.Errorf("%s has no default usage-limit patterns", name)
		}
	}
}

func TestResolveOverlayOverridesBuiltinField(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = map[string]config.Provider{
		"claude": {Command: "claude-canary"},
	}
	resolved := Resolve(cfg)
	p, _ := Lookup(resolved, "claude")
	if p.Command != "claude-canary" {
		t.Errorf("Command = %q, want claude-canary", p.Command)
	}
	// Untouched built-in fields persist.
	if len(p.UsageLimitPatterns) == 0 {
		t.Error("overriding command should not wipe the built-in usage-limit patterns")
	}
	if p.Mode != ModeFileRef {
		t.Errorf("Mode = %q, want file-ref (unchanged)", p.Mode)
	}
}

func TestResolveAddsNewProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Providers = map[string]config.Provider{
		"myai": {Command: "myai-cli", UsageLimitPatterns: []string{"limit"},
			PromptInjection: config.PromptInjection{Mode: "arg", Text: "go"}},
	}
	resolved := Resolve(cfg)
	p, ok := Lookup(resolved, "myai")
	if !ok {
		t.Fatal("config-defined provider not resolved")
	}
	if p.Command != "myai-cli" || p.Mode != ModeArg || p.PromptText != "go" {
		t.Errorf("resolved = %+v, want {myai-cli, arg, go}", p)
	}
	// Built-ins remain present alongside.
	if _, ok := Lookup(resolved, "claude"); !ok {
		t.Error("built-ins should remain after adding a config provider")
	}
}

func TestModeValid(t *testing.T) {
	for _, m := range []Mode{ModeFileRef, ModeArg, ModeStdin, ModePaste} {
		if !m.Valid() {
			t.Errorf("%q should be valid", m)
		}
	}
	for _, m := range []Mode{Mode(""), Mode("weird"), Mode("PASTE"), Mode("stdin ")} {
		if m.Valid() {
			t.Errorf("%q should be invalid", m)
		}
	}
}

func TestLookupMiss(t *testing.T) {
	if _, ok := Lookup(Resolve(config.Default()), "nope"); ok {
		t.Error("unknown provider should not resolve")
	}
}

func TestInjectFileRefAppendsReferenceArg(t *testing.T) {
	p := Provider{Name: "claude", Command: "claude", Mode: ModeFileRef}
	inj := p.Inject("/repo/.ai-session/handoff.md", []string{"--resume"})
	if len(inj.InitialInput) != 0 {
		t.Errorf("file-ref must not set InitialInput, got %q", inj.InitialInput)
	}
	if len(inj.Args) == 0 {
		t.Fatal("expected prompt arg")
	}
	last := inj.Args[len(inj.Args)-1]
	if !strings.Contains(last, "/repo/.ai-session/handoff.md") {
		t.Errorf("file-ref arg %q should reference the handoff path", last)
	}
	if inj.Args[0] != "--resume" {
		t.Errorf("user args should precede the prompt, got %v", inj.Args)
	}
}

func TestInjectArgUsesConfiguredText(t *testing.T) {
	p := Provider{Command: "x", Mode: ModeArg, PromptText: "custom prompt"}
	inj := p.Inject("/h.md", nil)
	if inj.Args[len(inj.Args)-1] != "custom prompt" {
		t.Errorf("arg mode should inject configured text, got %v", inj.Args)
	}
	if len(inj.InitialInput) != 0 {
		t.Error("arg mode must not set InitialInput")
	}
}

func TestInjectPasteAndStdinSetInitialInput(t *testing.T) {
	for _, mode := range []Mode{ModePaste, ModeStdin} {
		p := Provider{Command: "x", Mode: mode, PromptText: "typed", Args: []string{"-a"}}
		inj := p.Inject("/h.md", []string{"-b"})
		if string(inj.InitialInput) != "typed\r" {
			t.Errorf("%s InitialInput = %q, want \"typed\\r\"", mode, inj.InitialInput)
		}
		// No prompt appended as an arg; provider+user args preserved.
		if strings.Join(inj.Args, " ") != "-a -b" {
			t.Errorf("%s Args = %v, want [-a -b]", mode, inj.Args)
		}
	}
}

func TestInjectEmptyTextFallsBackToFileRef(t *testing.T) {
	p := Provider{Command: "x", Mode: ModeArg} // no PromptText
	inj := p.Inject("/repo/handoff.md", nil)
	last := inj.Args[len(inj.Args)-1]
	if !strings.Contains(last, "/repo/handoff.md") {
		t.Errorf("empty text should fall back to file-ref referencing the handoff, got %q", last)
	}
}

func TestInjectUnknownModeDefaultsFileRef(t *testing.T) {
	p := Provider{Command: "x", Mode: Mode("weird")}
	inj := p.Inject("/h.md", nil)
	if len(inj.InitialInput) != 0 {
		t.Error("unknown mode should behave like file-ref (no InitialInput)")
	}
	if len(inj.Args) != 1 {
		t.Errorf("unknown mode should append the prompt arg, got %v", inj.Args)
	}
}
