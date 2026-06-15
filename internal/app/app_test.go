package app

import (
	"bytes"
	"testing"

	"github.com/danielfoord/aictl/internal/ui"
)

func TestNewWiresUI(t *testing.T) {
	u := ui.New(&bytes.Buffer{}, &bytes.Buffer{})
	a := New(u)

	if a == nil {
		t.Fatal("New returned nil App")
	}
	if a.UI != u {
		t.Fatal("New did not wire the provided UI")
	}
}
