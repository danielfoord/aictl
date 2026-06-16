package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newRecoverCmd builds `aictl recover`, which writes a minimal last-resort
// continuation prompt to .ai-session/recovery.md.
func newRecoverCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "recover",
		Short: "Generate a minimal last-resort recovery prompt",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.Recover(cmd.Context())
		},
	}
}
