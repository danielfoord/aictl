package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newCheckpointCmd builds `aictl checkpoint`, which captures a labeled,
// provider-free checkpoint of the current session state and git evidence.
func newCheckpointCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "checkpoint <label>",
		Short: "Capture a labeled checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Checkpoint(cmd.Context(), args[0])
		},
	}
}
