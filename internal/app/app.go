// Package app wires aictl's dependencies and exposes the use-cases that the
// command layer invokes.
//
// Commands stay thin: they parse flags and call methods on App. Feature
// dependencies (config, session, git, handoff, providers, shell, ...) are
// added to App by the stories that introduce them. App is the seam that keeps
// business logic out of cmd/ and avoids global mutable state.
package app

import "github.com/danielfoord/aictl/internal/ui"

// App holds aictl's injected dependencies and orchestrates use-cases.
type App struct {
	UI *ui.UI
}

// New constructs an App with its dependencies.
func New(u *ui.UI) *App {
	return &App{UI: u}
}
