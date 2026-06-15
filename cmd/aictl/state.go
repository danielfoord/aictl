package main

import (
	"github.com/danielfoord/aictl/internal/app"
	"github.com/spf13/cobra"
)

// newNoteCmd builds `aictl note "<decision>"`.
func newNoteCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "note <decision>",
		Short: "Record a decision in the current session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Note(cmd.Context(), args[0])
		},
	}
}

// newDoneCmd builds `aictl done "<completed step>"`.
func newDoneCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "done <completed step>",
		Short: "Record a completed step in the current session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Done(cmd.Context(), args[0])
		},
	}
}

// newNextCmd builds `aictl next "<next step>"`.
func newNextCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "next <next step>",
		Short: "Record a next step in the current session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Next(cmd.Context(), args[0])
		},
	}
}

// newFailCmd builds `aictl fail "<known failure>"`.
func newFailCmd(a *app.App) *cobra.Command {
	return &cobra.Command{
		Use:   "fail <known failure>",
		Short: "Record a known failure in the current session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.Fail(cmd.Context(), args[0])
		},
	}
}
