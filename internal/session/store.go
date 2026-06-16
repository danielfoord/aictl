package session

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path atomically and durably: it writes to a temp
// file in the same directory, fsyncs it, then renames over the target (rename
// is atomic on POSIX). The parent directory is fsynced so the rename survives a
// crash. This is the single filesystem-write path for all Session artifacts —
// no other code should write Session files directly (NFR-3, NFR-4).
func WriteAtomic(path string, data []byte, perm os.FileMode) (err error) {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	// On any failure after creation, remove the temp file so we don't litter.
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err = tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp file over target: %w", err)
	}
	return SyncDir(dir)
}

// SyncDir flushes a directory entry so creates/renames within it are durable.
func SyncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("open dir for sync: %w", err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("sync dir: %w", err)
	}
	return nil
}
