package session

import "testing"

func TestTaskStateInProgress(t *testing.T) {
	tests := []struct {
		name  string
		state TaskState
		want  bool
	}{
		{"empty", TaskState{}, false},
		{"goal only", TaskState{Goal: "ship it"}, true},
		{"completed only", TaskState{Completed: []string{"x"}}, true},
		{"decisions only", TaskState{Decisions: []string{"x"}}, true},
		{"next steps only", TaskState{NextSteps: []string{"x"}}, true},
		{"known failures only", TaskState{KnownFailures: []string{"x"}}, true},
		// Branch/Dirty are captured at session start, VerifyCommands is config —
		// none of these count as task progress.
		{"branch only", TaskState{Branch: "main"}, false},
		{"dirty only", TaskState{Dirty: true}, false},
		{"verify commands only", TaskState{VerifyCommands: []string{"go test ./..."}}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.InProgress(); got != tc.want {
				t.Fatalf("InProgress() = %v, want %v", got, tc.want)
			}
		})
	}
}
