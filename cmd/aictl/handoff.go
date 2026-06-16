package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newHandoffCmd builds `aictl handoff` (alias `aictl prepare`), which writes the
// deterministic handoff packet to .ai-session/handoff.md.
func newHandoffCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:     "handoff",
		Aliases: []string{"prepare"},
		Short:   "Generate the portable handoff packet (.ai-session/handoff.md)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.Handoff(cmd.Context())
		},
	}
}
