package config

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// applyMutex serializes the write -> `nginx -t` -> reload sequence of every
// configuration mutation. `nginx -t` validates the whole configuration tree, so
// without this lock two concurrent saves would test each other's half applied
// files and roll back a change that is perfectly valid on its own.
var applyMutex sync.Mutex

// LockApply blocks until no other configuration mutation sits between its write
// and its reload, and returns the function that releases the lock.
func LockApply() (release func()) {
	applyMutex.Lock()
	return applyMutex.Unlock
}

// FileSnapshot records the on-disk state of a configuration file so a failed
// nginx test or reload can put the previous state back. A snapshot taken of a
// missing file restores by removing the file again, which is what a newly
// created configuration needs.
type FileSnapshot struct {
	exists  bool
	content []byte
	mode    os.FileMode
}

// CaptureFile snapshots the configuration file at path.
func CaptureFile(path string) (FileSnapshot, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return FileSnapshot{}, nil
	}
	if err != nil {
		return FileSnapshot{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return FileSnapshot{}, err
	}

	return FileSnapshot{
		exists:  true,
		content: content,
		mode:    info.Mode().Perm(),
	}, nil
}

// Restore puts the captured state back on disk. A snapshot of a file that did
// not exist removes the file the caller created.
func (snapshot FileSnapshot) Restore(path string) error {
	if !snapshot.exists {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	return WriteFile(path, snapshot.content, snapshot.mode)
}

// WriteFile replaces the content of a configuration file in place to preserve
// symlink targets and file-level bind mounts. The caller keeps a snapshot and
// restores it when validation or reload fails.
func WriteFile(path string, content []byte, mode os.FileMode) error {
	return os.WriteFile(path, content, mode)
}

// RollbackError runs the rollback and reports the original failure, adding the
// rollback failure when the previous state could not be put back.
func RollbackError(primary error, rollback func() error) error {
	if err := rollback(); err != nil {
		return errors.Join(primary, fmt.Errorf("rollback failed: %w", err))
	}
	return primary
}

// RestoreAndReload puts the previous content back and makes the running Nginx
// use it again.
func RestoreAndReload(path string, snapshot FileSnapshot) error {
	if err := snapshot.Restore(path); err != nil {
		return err
	}

	return reloadRestored()
}

// reloadRestored validates and loads the configuration a rollback put back in
// place.
func reloadRestored() error {
	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return fmt.Errorf("test restored configuration: %w", result.GetError())
	}
	if result := nginx.Control(nginx.Reload); result.IsError() {
		return fmt.Errorf("reload restored configuration: %w", result.GetError())
	}

	return nil
}

// FileTransaction snapshots every file it writes so a rejected `nginx -t` can
// restore all of them, including removing the files the transaction created.
type FileTransaction struct {
	entries []fileTransactionEntry
}

type fileTransactionEntry struct {
	path     string
	snapshot FileSnapshot
}

// Write snapshots path and then replaces its content.
func (t *FileTransaction) Write(path string, content []byte, mode os.FileMode) error {
	snapshot, err := CaptureFile(path)
	if err != nil {
		return err
	}

	t.entries = append(t.entries, fileTransactionEntry{path: path, snapshot: snapshot})

	return WriteFile(path, content, mode)
}

// Len reports how many files the transaction has written.
func (t *FileTransaction) Len() int {
	return len(t.entries)
}

// Rollback restores every written file in reverse order and reports every
// restore failure it hit.
func (t *FileTransaction) Rollback() error {
	errs := make([]error, 0, len(t.entries))
	for i := len(t.entries) - 1; i >= 0; i-- {
		if err := t.entries[i].snapshot.Restore(t.entries[i].path); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// RollbackAndReload restores every written file and makes the running Nginx use
// the restored configuration again.
func (t *FileTransaction) RollbackAndReload() error {
	if err := t.Rollback(); err != nil {
		return err
	}

	return reloadRestored()
}

// TestAndReload validates the configuration on disk before reloading Nginx.
// A rejected configuration is rolled back, because the running Nginx keeps
// serving its old in-memory configuration and invalid content left on disk
// would only surface on the next Nginx start, when every vhost goes down.
func (t *FileTransaction) TestAndReload() error {
	if result := nginx.Control(nginx.TestConfig); result.IsError() {
		return RollbackError(result.GetError(), t.Rollback)
	}

	if result := nginx.Control(nginx.Reload); result.IsError() {
		return RollbackError(result.GetError(), t.RollbackAndReload)
	}

	return nil
}
