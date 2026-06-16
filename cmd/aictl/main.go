// Command aictl is a provider-agnostic supervisor for agentic coding CLIs.
//
// It wraps real provider CLIs (Claude Code, Codex, Gemini, ...) and owns
// durable, portable session state so a coding task survives provider switches
// and quota limits. This file is the entry point; subcommands are registered
// by their own features.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/danielfoord/aictl/internal/app"
	"github.com/danielfoord/aictl/internal/ui"
)

// version is the build version. It defaults to "dev" and is overridden at
// release time via -ldflags "-X main.version=<tag>". When unset, the version
// is resolved from the build info embedded by `go install module@version`.
var version = "dev"

func main() {
	// A signal-aware context so the whole command tree can observe Ctrl-C /
	// SIGTERM. Cancellation flows from here into every use-case.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application := app.New(ui.New(os.Stdout, os.Stderr))
	root := newRootCmd(application, resolveVersion())

	if err := root.ExecuteContext(ctx); err != nil {
		if code, ok := exitCodeFromError(err); ok {
			os.Exit(code)
		}
		fmt.Fprintln(os.Stderr, "aictl:", err)
		os.Exit(1)
	}
}

func exitCodeFromError(err error) (int, bool) {
	var exitErr app.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code, true
	}
	return 1, false
}

// resolveVersion prefers the ldflags-injected version, then the module version
// recorded by `go install`, falling back to the default.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}
