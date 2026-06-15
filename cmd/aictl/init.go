package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newInitCmd builds `aictl init`, which scaffolds the `.ai-session/` Session
// Directory in the current repo.
func newInitCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the .ai-session/ Session Directory in the current repo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.Init(cmd.Context())
		},
	}
}
