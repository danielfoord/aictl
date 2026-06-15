package session

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLockSucceedsOnFreshTree(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	if err := AcquireLock(path); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lockfile not created: %v", err)
	}
}

func TestAcquireLockRefusesWhenHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	if err := AcquireLock(path); err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	err := AcquireLock(path)
	if !errors.Is(err, ErrSessionLocked) {
		t.Fatalf("second AcquireLock error = %v, want ErrSessionLocked", err)
	}
}

func TestReleaseLockAllowsReacquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	if err := AcquireLock(path); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}
	if err := ReleaseLock(path); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}
	if err := AcquireLock(path); err != nil {
		t.Fatalf("re-AcquireLock after release: %v", err)
	}
}

func TestReleaseLockMissingIsNoError(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	if err := ReleaseLock(path); err != nil {
		t.Fatalf("ReleaseLock on missing lock: %v", err)
	}
}
