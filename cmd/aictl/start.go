package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newStartCmd builds `aictl start "<goal>"`, which opens a Session with an
// immutable goal in the current repo.
func newStartCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "start <goal>",
		Short: "Start a session with an immutable goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Start(cmd.Context(), args[0])
		},
	}
}
