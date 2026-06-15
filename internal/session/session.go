package session

import "github.com/goccy/go-yaml"

// TaskState is the provider-independent, aictl-owned record of a Session. It is
// persisted as YAML and is human-readable and hand-editable (NFR-4). `aictl
// init` writes an empty TaskState; the Goal is set by `aictl start` (Story 1.3).
type TaskState struct {
	Goal           string   `yaml:"goal"`
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
