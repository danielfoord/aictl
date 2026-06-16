package providers

import "fmt"

// defaultFileRefText is the built-in file-reference instruction. It points the
// provider at the handoff and forbids restarting from scratch (FR-17).
const defaultFileRefText = "Read %s and continue the in-progress task. Do not restart from scratch."

// Injection is a launch plan for a provider: the argv args to pass and, for
// paste/stdin modes, bytes to write into the PTY after the child starts.
type Injection struct {
	Args         []string
	InitialInput []byte
}

// Inject builds the launch plan for the provider, given the handoff path and the
// user's extra args. file-ref/arg deliver the prompt as the final argument;
// stdin/paste deliver it by writing into the PTY after launch.
func (p Provider) Inject(handoffPath string, userArgs []string) Injection {
	text := p.effectiveText(handoffPath)
	args := append(append([]string(nil), p.Args...), userArgs...)

	switch p.Mode {
	case ModeStdin, ModePaste:
		return Injection{Args: args, InitialInput: []byte(text + "\r")}
	default: // file-ref, arg, and any unknown/empty mode
		return Injection{Args: append(args, text)}
	}
}

// effectiveText is the prompt to inject: the configured text if set, otherwise
// the default file-reference instruction pointing at the handoff path.
func (p Provider) effectiveText(handoffPath string) string {
	if p.PromptText != "" {
		return p.PromptText
	}
	return fmt.Sprintf(defaultFileRefText, handoffPath)
}
