package session

import (
	"fmt"
	"os"

	"github.com/goccy/go-yaml"
)

// TaskState is the provider-independent, aictl-owned record of a Session. It is
// persisted as YAML and is human-readable and hand-editable (NFR-4). `aictl
// init` writes an empty TaskState; the Goal and initial repo context are set by
// `aictl start` (Story 1.3).
type TaskState struct {
	Goal           string   `yaml:"goal"`
	Branch         string   `yaml:"branch"` // branch captured at session start
	Dirty          bool     `yaml:"dirty"`  // working tree dirty at session start
	Completed      []string `yaml:"completed"`
	Decisions      []string `yaml:"decisions"`
	NextSteps      []string `yaml:"nextSteps"`
	KnownFailures  []string `yaml:"knownFailures"`
	VerifyCommands []string `yaml:"verifyCommands"`
}

// Marshal serializes the Task State to YAML.
func (s TaskState) Marshal() ([]byte, error) {
	return yaml.Marshal(s)
}

// UnmarshalTaskState parses Task State from YAML.
func UnmarshalTaskState(data []byte) (TaskState, error) {
	var s TaskState
	err := yaml.Unmarshal(data, &s)
	return s, err
}

// LoadState reads and parses the Task State at path.
func LoadState(path string) (TaskState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TaskState{}, fmt.Errorf("read task state: %w", err)
	}
	return UnmarshalTaskState(data)
}

// SaveState atomically persists the Task State to path.
func SaveState(path string, s TaskState) error {
	data, err := s.Marshal()
	if err != nil {
		return fmt.Errorf("marshal task state: %w", err)
	}
	return WriteAtomic(path, data, 0o644)
}
