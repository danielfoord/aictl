package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "config.yaml"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if cfg.Handoff.MaxDiffChars != DefaultMaxDiffChars || len(cfg.Denylist) == 0 {
		t.Fatalf("missing config should yield defaults, got %+v", cfg)
	}
}

func TestLoadNormalizesEmptyGuardrails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// A present config that zeroes the guardrails must not disable them.
	if err := os.WriteFile(path, []byte("denylist: []\nhandoff:\n  maxDiffChars: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Denylist) == 0 {
		t.Error("empty denylist should fall back to the default denylist")
	}
	if cfg.Handoff.MaxDiffChars != DefaultMaxDiffChars {
		t.Errorf("non-positive maxDiffChars should fall back to default, got %d", cfg.Handoff.MaxDiffChars)
	}
}

func TestLoadPreservesExplicitValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("denylist:\n  - \"*.secret\"\nhandoff:\n  maxDiffChars: 123\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Handoff.MaxDiffChars != 123 {
		t.Errorf("MaxDiffChars = %d, want 123", cfg.Handoff.MaxDiffChars)
	}
	if len(cfg.Denylist) != 1 || cfg.Denylist[0] != "*.secret" {
		t.Errorf("Denylist = %v, want [*.secret]", cfg.Denylist)
	}
}
