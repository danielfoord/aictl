package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newRunCmd builds `aictl run`, which launches a provider CLI in a PTY.
func newRunCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "run <provider> [-- provider-args...]",
		Short: "Run a provider CLI in a pseudo-terminal",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := a.Run(cmd.Context(), args[0], args[1:])
			return err
		},
	}
}
