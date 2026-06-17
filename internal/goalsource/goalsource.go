// Package goalsource reads a tool-agnostic, user-configured source (a file or a
// command) and parses it into task context for `aictl sync`. aictl ships no
// knowledge of any specific tool: the integration (BMad, Linear, a plain file,
// anything) lives entirely in the user's config command/file.
package goalsource

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
)

// Context is the task context derived from a goal source. A source may provide a
// structured form (all fields) or plain text (Goal only).
type Context struct {
	Goal          string   `yaml:"goal"`
	NextSteps     []string `yaml:"nextSteps"`
	Decisions     []string `yaml:"decisions"`
	KnownFailures []string `yaml:"knownFailures"`
}

// Read returns the raw source text. If file is set it is read (resolved relative
// to dir); else if command is set it is run via `sh -c` in dir and its stdout
// used. If neither is set, the second return is false (source unconfigured).
func Read(ctx context.Context, dir, file, command string) (string, bool, error) {
	switch {
	case strings.TrimSpace(file) != "":
		path := file
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, file)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("read goal source file: %w", err)
		}
		return string(data), true, nil
	case strings.TrimSpace(command) != "":
		c := exec.CommandContext(ctx, "sh", "-c", command)
		c.Dir = dir
		out, err := c.Output()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
				return "", false, fmt.Errorf("run goal source command: %w: %s", err, strings.TrimSpace(string(exitErr.Stderr)))
			}
			return "", false, fmt.Errorf("run goal source command: %w", err)
		}
		return string(out), true, nil
	default:
		return "", false, nil
	}
}

// Parse interprets source text as structured YAML when it parses and carries a
// non-empty goal; otherwise the whole trimmed text becomes the goal.
func Parse(text string) Context {
	if strings.TrimSpace(text) == "" {
		return Context{}
	}
	var c Context
	// Accept the structured form when it parses and carries any field — so a
	// source that supplies nextSteps/decisions but no goal does not get its raw
	// YAML jammed into Goal (the caller no-ops cleanly on an empty goal instead).
	if err := yaml.Unmarshal([]byte(text), &c); err == nil &&
		(strings.TrimSpace(c.Goal) != "" || len(c.NextSteps) > 0 || len(c.Decisions) > 0 || len(c.KnownFailures) > 0) {
		c.Goal = strings.TrimSpace(c.Goal)
		return c
	}
	return Context{Goal: strings.TrimSpace(text)}
}
