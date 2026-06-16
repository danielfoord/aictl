// Package checkpoint captures durable before/after run artifacts under
// `.ai-session/checkpoints`.
package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/danielfoord/aictl/internal/git"
	"github.com/danielfoord/aictl/internal/session"
)

const (
	prePhase    = "before"
	postPhase   = "after"
	manualPhase = "checkpoint"
)

// FilesChanged is the evidence-backed verdict comparing before/after git state.
type FilesChanged string

const (
	FilesChangedYes     FilesChanged = "yes"
	FilesChangedNo      FilesChanged = "no"
	FilesChangedUnknown FilesChanged = "unknown"
)

// GitState is the git evidence persisted into checkpoints.
type GitState struct {
	Status        string
	Diff          string
	RecentCommits string
	Available     bool
}

// Pre is the created pre-run checkpoint.
type Pre struct {
	Sequence     int
	ProviderName string
	Dir          string
	Git          GitState
}

// Post is the created post-run checkpoint.
type Post struct {
	Sequence     int
	ProviderName string
	Dir          string
	Git          GitState
	FilesChanged FilesChanged
}

// Manual is the created on-demand checkpoint.
type Manual struct {
	Sequence  int
	Label     string
	LabelName string
	Dir       string
	Git       GitState
}

// PreOptions describes one pre-run checkpoint capture.
type PreOptions struct {
	Paths           session.Paths
	RepoRoot        string
	Sequence        int
	ProviderName    string
	HandoffMarkdown string
	State           session.TaskState
	CommandLog      string
	Git             GitState
	Denylist        []string
	MaxDiffChars    int
}

// PostOptions describes one post-run checkpoint capture.
type PostOptions struct {
	Paths        session.Paths
	RepoRoot     string
	Pre          Pre
	ExitCode     int
	Transcript   []byte
	PostGit      GitState
	Denylist     []string
	MaxDiffChars int
}

// ManualOptions describes one user-requested checkpoint capture.
type ManualOptions struct {
	Paths        session.Paths
	RepoRoot     string
	Sequence     int
	Label        string
	State        session.TaskState
	CommandLog   string
	LatestVerify string
	Git          GitState
	Denylist     []string
	MaxDiffChars int
}

// SafeProviderName returns a filesystem-safe provider name for checkpoint dirs.
func SafeProviderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "provider"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '.', r == '_', r == '-':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "provider"
	}
	return out
}

// NextSequence returns the next zero-padded checkpoint sequence number.
func NextSequence(paths session.Paths) (int, error) {
	entries, err := os.ReadDir(paths.Checkpoints())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 1, nil
		}
		return 0, fmt.Errorf("read checkpoints directory: %w", err)
	}
	maxSeq := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) < 11 {
			continue
		}
		if !strings.Contains(name, "-"+prePhase+"-") &&
			!strings.Contains(name, "-"+postPhase+"-") &&
			!strings.Contains(name, "-"+manualPhase+"-") {
			continue
		}
		seq, ok := parseCheckpointSequence(name)
		if !ok {
			continue
		}
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq + 1, nil
}

// CaptureGit captures redacted, bounded git evidence. Outside a git repository
// is represented as unavailable rather than as a hard failure.
func CaptureGit(ctx context.Context, repoRoot string, denylist []string, maxDiffChars int) (GitState, error) {
	var state GitState
	status, err := git.Status(ctx, repoRoot)
	if err != nil {
		if errors.Is(err, git.ErrNotARepo) {
			return GitState{}, nil
		}
		return GitState{}, fmt.Errorf("capture git status: %w", err)
	}
	state.Available = true
	state.Status = git.RedactStatusPaths(status, denylist)

	diff, err := git.Diff(ctx, repoRoot, denylist, maxDiffChars)
	if err != nil {
		if errors.Is(err, git.ErrNotARepo) {
			return GitState{}, nil
		}
		return GitState{}, fmt.Errorf("capture git diff: %w", err)
	}
	state.Diff = diff

	commits, err := git.RecentCommits(ctx, repoRoot, 10)
	if err != nil {
		if errors.Is(err, git.ErrNotARepo) {
			return GitState{}, nil
		}
		return GitState{}, fmt.Errorf("capture recent commits: %w", err)
	}
	state.RecentCommits = commits
	return state, nil
}

// CapturePre persists a before-run checkpoint and returns its metadata.
func CapturePre(ctx context.Context, opts PreOptions) (Pre, error) {
	gitState := opts.Git
	if emptyGitState(gitState) && opts.RepoRoot != "" {
		var err error
		gitState, err = CaptureGit(ctx, opts.RepoRoot, opts.Denylist, opts.MaxDiffChars)
		if err != nil {
			// Fail closed before a run: a real pre-run git failure aborts the
			// checkpoint rather than launching with unknown state (NFR-3).
			return Pre{}, err
		}
	}

	provider := SafeProviderName(opts.ProviderName)
	seq, dir, err := reservePreCheckpointDir(opts.Paths, opts.Sequence, provider)
	if err != nil {
		return Pre{}, err
	}
	files := map[string][]byte{
		"handoff.md":         []byte(opts.HandoffMarkdown),
		"git-status.txt":     []byte(gitState.Status),
		"git-diff.patch":     []byte(gitState.Diff),
		"recent-commits.txt": []byte(gitState.RecentCommits),
		"command-log.md":     []byte(opts.CommandLog),
		"summary.md":         []byte(renderSummary(opts.ProviderName, opts.State, gitState)),
	}
	for name, data := range files {
		if err := session.WriteAtomic(filepath.Join(dir, name), data, filePerm(name)); err != nil {
			return Pre{}, fmt.Errorf("write pre-run checkpoint %s: %w", name, err)
		}
	}
	return Pre{Sequence: seq, ProviderName: provider, Dir: dir, Git: gitState}, nil
}

// CapturePost persists an after-run checkpoint and returns its metadata.
func CapturePost(ctx context.Context, opts PostOptions) (Post, error) {
	gitState := opts.PostGit
	if emptyGitState(gitState) && opts.RepoRoot != "" {
		var err error
		gitState, err = CaptureGit(ctx, opts.RepoRoot, opts.Denylist, opts.MaxDiffChars)
		if err != nil {
			gitState = GitState{
				Status: fmt.Sprintf("git capture failed: %v\n", err),
			}
		}
	}
	provider := opts.Pre.ProviderName
	if provider == "" {
		provider = SafeProviderName("provider")
	}
	dirName := checkpointDirName(opts.Pre.Sequence, postPhase, provider)
	dir := opts.Paths.CheckpointDir(dirName)
	if err := ensureDirDurable(dir); err != nil {
		return Post{}, fmt.Errorf("create post-run checkpoint directory: %w", err)
	}

	verdict := DetermineFilesChanged(opts.Pre.Git, gitState)
	files := map[string][]byte{
		"exit-code.txt":      []byte(fmt.Sprintf("%d\n", opts.ExitCode)),
		"git-status.txt":     []byte(gitState.Status),
		"git-diff.patch":     []byte(gitState.Diff),
		"recent-commits.txt": []byte(gitState.RecentCommits),
		"transcript-ref.txt": []byte("transcript.ansi\n"),
		"files-changed.txt":  []byte(string(verdict) + "\n"),
	}
	if opts.Transcript != nil {
		files["transcript.ansi"] = opts.Transcript
	} else if _, err := os.Stat(filepath.Join(dir, "transcript.ansi")); errors.Is(err, os.ErrNotExist) {
		files["transcript.ansi"] = nil
	}
	for name, data := range files {
		if err := session.WriteAtomic(filepath.Join(dir, name), data, filePerm(name)); err != nil {
			return Post{}, fmt.Errorf("write post-run checkpoint %s: %w", name, err)
		}
	}
	return Post{Sequence: opts.Pre.Sequence, ProviderName: provider, Dir: dir, Git: gitState, FilesChanged: verdict}, nil
}

// CaptureManual persists an on-demand labeled checkpoint and returns its metadata.
func CaptureManual(ctx context.Context, opts ManualOptions) (Manual, error) {
	label := strings.TrimSpace(opts.Label)
	if label == "" {
		return Manual{}, errors.New("checkpoint label must not be empty")
	}
	gitState := opts.Git
	if emptyGitState(gitState) && opts.RepoRoot != "" {
		var err error
		gitState, err = CaptureGit(ctx, opts.RepoRoot, opts.Denylist, opts.MaxDiffChars)
		if err != nil {
			gitState = GitState{
				Available: false,
				Status:    fmt.Sprintf("git capture failed: %v", err),
			}
		}
	}

	labelName := SafeLabelName(label)
	seq, dir, err := reserveManualCheckpointDir(opts.Paths, opts.Sequence, labelName)
	if err != nil {
		return Manual{}, err
	}
	files := map[string][]byte{
		"git-status.txt":     []byte(gitState.Status),
		"git-diff.patch":     []byte(gitState.Diff),
		"recent-commits.txt": []byte(gitState.RecentCommits),
		"command-log.md":     []byte(opts.CommandLog),
		"summary.md":         []byte(renderManualSummary(label, opts.State, gitState)),
	}
	if opts.LatestVerify != "" {
		files["latest-verify.txt"] = []byte(opts.LatestVerify)
	}
	for name, data := range files {
		if err := session.WriteAtomic(filepath.Join(dir, name), data, filePerm(name)); err != nil {
			return Manual{}, fmt.Errorf("write manual checkpoint %s: %w", name, err)
		}
	}
	return Manual{Sequence: seq, Label: label, LabelName: labelName, Dir: dir, Git: gitState}, nil
}

// maxLabelNameLen bounds the sanitized label so a checkpoint directory name
// stays well under the filesystem NAME_MAX (255) once the sequence + phase
// prefix is added.
const maxLabelNameLen = 80

// SafeLabelName returns a filesystem-safe, readable checkpoint label name,
// bounded to maxLabelNameLen so a pathological label cannot exceed NAME_MAX.
func SafeLabelName(label string) string {
	name := strings.Trim(SafeProviderName(label), ".-")
	if len(name) > maxLabelNameLen {
		name = strings.Trim(name[:maxLabelNameLen], ".-")
	}
	if name == "" {
		return "checkpoint"
	}
	return name
}

// DetermineFilesChanged compares persisted git evidence conservatively.
func DetermineFilesChanged(pre, post GitState) FilesChanged {
	if !pre.Available || !post.Available {
		return FilesChangedUnknown
	}
	if pre.Status == post.Status && pre.Diff == post.Diff && pre.RecentCommits == post.RecentCommits {
		if containsRedaction(pre) || containsRedaction(post) {
			return FilesChangedUnknown
		}
		return FilesChangedNo
	}
	return FilesChangedYes
}

func reservePreCheckpointDir(paths session.Paths, requestedSeq int, provider string) (int, string, error) {
	if err := ensureDirDurable(paths.Checkpoints()); err != nil {
		return 0, "", fmt.Errorf("create checkpoints directory: %w", err)
	}
	seq := requestedSeq
	if seq <= 0 {
		var err error
		seq, err = NextSequence(paths)
		if err != nil {
			return 0, "", err
		}
	}
	for {
		dirName := checkpointDirName(seq, prePhase, provider)
		dir := paths.CheckpointDir(dirName)
		if err := os.Mkdir(dir, 0o755); err != nil {
			if requestedSeq <= 0 && errors.Is(err, os.ErrExist) {
				seq++
				continue
			}
			return 0, "", fmt.Errorf("create pre-run checkpoint directory: %w", err)
		}
		if err := session.SyncDir(paths.Checkpoints()); err != nil {
			return 0, "", err
		}
		return seq, dir, nil
	}
}

func reserveManualCheckpointDir(paths session.Paths, requestedSeq int, label string) (int, string, error) {
	if err := ensureDirDurable(paths.Checkpoints()); err != nil {
		return 0, "", fmt.Errorf("create checkpoints directory: %w", err)
	}
	seq := requestedSeq
	if seq <= 0 {
		var err error
		seq, err = NextSequence(paths)
		if err != nil {
			return 0, "", err
		}
	}
	for attempts := 0; attempts < 1000; attempts++ {
		dirName := checkpointDirName(seq, manualPhase, label)
		dir := paths.CheckpointDir(dirName)
		if err := os.Mkdir(dir, 0o755); err != nil {
			if requestedSeq <= 0 && errors.Is(err, os.ErrExist) {
				seq++
				continue
			}
			return 0, "", fmt.Errorf("create manual checkpoint directory: %w", err)
		}
		if err := session.SyncDir(paths.Checkpoints()); err != nil {
			return 0, "", err
		}
		return seq, dir, nil
	}
	return 0, "", errors.New("manual checkpoint directory reservation: too many collisions")
}

func ensureDirDurable(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return session.SyncDir(filepath.Dir(dir))
}

func parseCheckpointSequence(name string) (int, bool) {
	prefix, _, ok := strings.Cut(name, "-")
	if !ok || prefix == "" {
		return 0, false
	}
	seq, err := strconv.Atoi(prefix)
	if err != nil || seq <= 0 {
		return 0, false
	}
	return seq, true
}

func checkpointDirName(seq int, phase, provider string) string {
	return fmt.Sprintf("%04d-%s-%s", seq, phase, provider)
}

func filePerm(name string) os.FileMode {
	if name == "transcript.ansi" {
		return 0o600
	}
	return 0o644
}

func containsRedaction(state GitState) bool {
	return strings.Contains(state.Status, "[redacted]") ||
		strings.Contains(state.Diff, "[redacted:")
}

func emptyGitState(state GitState) bool {
	return !state.Available && state.Status == "" && state.Diff == "" && state.RecentCommits == ""
}

func renderManualSummary(label string, state session.TaskState, gitState GitState) string {
	var b strings.Builder
	b.WriteString("# Manual Checkpoint Summary\n\n")
	fmt.Fprintf(&b, "Label: %s\n", label)
	fmt.Fprintf(&b, "Goal: %s\n", state.Goal)
	fmt.Fprintf(&b, "Git available: %t\n", gitState.Available)
	if len(state.NextSteps) > 0 {
		b.WriteString("\nNext steps:\n")
		for _, step := range state.NextSteps {
			fmt.Fprintf(&b, "- %s\n", step)
		}
	}
	if len(state.Decisions) > 0 {
		b.WriteString("\nDecisions:\n")
		for _, decision := range state.Decisions {
			fmt.Fprintf(&b, "- %s\n", decision)
		}
	}
	if len(state.KnownFailures) > 0 {
		b.WriteString("\nKnown failures:\n")
		for _, failure := range state.KnownFailures {
			fmt.Fprintf(&b, "- %s\n", failure)
		}
	}
	return b.String()
}

func renderSummary(provider string, state session.TaskState, gitState GitState) string {
	var b strings.Builder
	b.WriteString("# Run Summary\n\n")
	fmt.Fprintf(&b, "Provider: %s\n", provider)
	fmt.Fprintf(&b, "Goal: %s\n", state.Goal)
	fmt.Fprintf(&b, "Git available: %t\n", gitState.Available)
	if len(state.NextSteps) > 0 {
		b.WriteString("\nNext steps:\n")
		for _, step := range state.NextSteps {
			fmt.Fprintf(&b, "- %s\n", step)
		}
	}
	if len(state.Decisions) > 0 {
		b.WriteString("\nDecisions:\n")
		for _, decision := range state.Decisions {
			fmt.Fprintf(&b, "- %s\n", decision)
		}
	}
	if len(state.KnownFailures) > 0 {
		b.WriteString("\nKnown failures:\n")
		for _, failure := range state.KnownFailures {
			fmt.Fprintf(&b, "- %s\n", failure)
		}
	}
	return b.String()
}
