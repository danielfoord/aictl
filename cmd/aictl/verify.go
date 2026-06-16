package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newVerifyCmd builds `aictl verify`, which runs the configured verification
// commands and captures their output for the next handoff.
func newVerifyCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Run configured verification commands and capture their output",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.Verify(cmd.Context())
		},
	}
}
