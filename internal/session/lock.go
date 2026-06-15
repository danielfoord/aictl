package session

import (
	"errors"
	"fmt"
	"os"
	"time"
)

// ErrSessionLocked indicates a Session is already active in this working tree
// (the lockfile exists). aictl enforces exactly one active Session per working
// tree.
var ErrSessionLocked = errors.New("a session is already active in this working tree (.ai-session/.lock exists)")

// AcquireLock creates the lockfile with exclusive-create semantics (O_EXCL), so
// acquisition is atomic and race-free. It returns ErrSessionLocked if the lock
// already exists. The lock records the pid and acquisition time for diagnostics.
//
// NOTE: the lock deliberately does NOT go through WriteAtomic — it needs atomic
// exclusive *create* (O_EXCL), which a temp-file+rename cannot provide. In v1
// the lock is a persistent "active Session" marker and is not released at the
// end of a command; it is cleared when the Session is reset.
func AcquireLock(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrSessionLocked
		}
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer func() { _ = f.Close() }()

	_, _ = fmt.Fprintf(f, "pid: %d\nacquiredAt: %s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	return nil
}

// ReleaseLock removes the lockfile. Removing a non-existent lock is not an error.
func ReleaseLock(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("release lock: %w", err)
	}
	return nil
}
