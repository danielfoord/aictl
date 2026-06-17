package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newSyncCmd builds `aictl sync`, which populates the session goal from the
// configured goalSource (a file or command) when the session has no goal yet.
func newSyncCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Populate the session goal from the configured goalSource",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.Sync(cmd.Context())
		},
	}
}
