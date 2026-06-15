package main

import (
	"github.com/danielfoord/aictl/internal/ui"
	"github.com/spf13/cobra"
)

// newRootCmd builds the `aictl` root command. This story establishes only the
// skeleton and --version; feature subcommands (init, start, run, checkpoint,
// handoff, verify, recover, ...) are registered by their own stories. Commands
// stay thin and call into internal/app — no business logic lives here.
func newRootCmd(out *ui.UI, version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "aictl",
		Short: "Provider-agnostic supervisor for agentic coding CLIs",
		Long: "aictl supervises agentic coding CLIs (Claude Code, Codex, Gemini, and others),\n" +
			"owning durable, portable session state so a coding task survives provider\n" +
			"switches and quota limits.",
		Version: version,
		// We print errors ourselves at the top level and don't want usage spam
		// on every runtime error.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Route Cobra's output through the single user-facing writer so it can be
	// centrally muted while a provider owns the screen (see NFR-2).
	rootCmd.SetOut(out.Out())
	rootCmd.SetErr(out.Err())

	return rootCmd
}
