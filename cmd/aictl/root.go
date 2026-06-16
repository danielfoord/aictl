package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newRootCmd builds the `aictl` root command and registers its subcommands.
// Commands stay thin and call into internal/app — no business logic lives here.
func newRootCmd(a *app.App, version string) *cobra.Command {
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
	rootCmd.SetOut(a.UI.Out())
	rootCmd.SetErr(a.UI.Err())

	rootCmd.AddCommand(newInitCmd(a))
	rootCmd.AddCommand(newStartCmd(a))
	rootCmd.AddCommand(newNoteCmd(a))
	rootCmd.AddCommand(newDoneCmd(a))
	rootCmd.AddCommand(newNextCmd(a))
	rootCmd.AddCommand(newFailCmd(a))
	rootCmd.AddCommand(newHandoffCmd(a))
	rootCmd.AddCommand(newVerifyCmd(a))
	rootCmd.AddCommand(newRecoverCmd(a))
	rootCmd.AddCommand(newRunCmd(a))

	return rootCmd
}
