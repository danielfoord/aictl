package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/danielfoord/aictl/internal/shell"
)

// ExitError carries a process exit code that main should preserve.
type ExitError struct {
	Code int
}

func (e ExitError) Error() string {
	return fmt.Sprintf("provider exited with code %d", e.Code)
}

// Run launches a provider executable through the shell PTY runner.
func (a *App) Run(ctx context.Context, provider string, args []string) (shell.Result, error) {
	path, err := exec.LookPath(provider)
	if err != nil {
		return shell.Result{ExitCode: 1}, fmt.Errorf("provider executable not found %q: %w", provider, err)
	}

	a.UI.Printf("Launching %s\n", provider)
	res, err := a.runProvider(ctx, shell.Options{
		Command: path,
		Args:    args,
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})
	if err != nil {
		return res, err
	}
	a.UI.Printf("Provider %s exited with code %d\n", provider, res.ExitCode)
	if res.ExitCode != 0 {
		return res, ExitError{Code: res.ExitCode}
	}
	return res, nil
}
