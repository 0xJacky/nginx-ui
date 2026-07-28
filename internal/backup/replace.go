package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type stagedFileReplacement struct {
	destination string
	staged      string
}

type appliedReplacement interface {
	Commit() error
	Rollback() error
}

type appliedFileReplacement struct {
	parent      string
	rollbackDir string
	backups     map[string]string
	installed   []string
}

func stageBytes(destination string, content []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(destination), ".nginx-ui-restore-stage-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()

	if err := file.Chmod(mode.Perm()); err != nil {
		return "", err
	}
	if _, err := file.Write(content); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	remove = false
	return path, nil
}

func stageFile(source, destination string, mode os.FileMode) (string, error) {
	sourceFile, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	info, err := sourceFile.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("restore source %q is not a regular file", source)
	}

	destinationFile, err := os.CreateTemp(filepath.Dir(destination), ".nginx-ui-restore-stage-*")
	if err != nil {
		return "", err
	}
	path := destinationFile.Name()
	remove := true
	defer func() {
		_ = destinationFile.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()

	if err := destinationFile.Chmod(mode.Perm()); err != nil {
		return "", err
	}
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return "", err
	}
	if err := destinationFile.Sync(); err != nil {
		return "", err
	}
	if err := destinationFile.Close(); err != nil {
		return "", err
	}
	remove = false
	return path, nil
}

// replaceFilesWithRollback moves every current destination aside before
// installing staged files. A failed rename restores all original paths.
func replaceFilesWithRollback(replacements []stagedFileReplacement, removals []string) error {
	applied, err := applyFilesWithRollback(replacements, removals)
	if err != nil {
		return err
	}
	return applied.Commit()
}

func applyFilesWithRollback(replacements []stagedFileReplacement, removals []string) (*appliedFileReplacement, error) {
	if len(replacements) == 0 && len(removals) == 0 {
		return &appliedFileReplacement{}, nil
	}

	firstDestination := ""
	if len(replacements) != 0 {
		firstDestination = replacements[0].destination
	} else {
		firstDestination = removals[0]
	}
	parent := filepath.Dir(firstDestination)
	rollbackDir, err := os.MkdirTemp(parent, ".nginx-ui-restore-rollback-*")
	if err != nil {
		return nil, err
	}
	applied := &appliedFileReplacement{
		parent:      parent,
		rollbackDir: rollbackDir,
		backups:     make(map[string]string, len(replacements)+len(removals)),
		installed:   make([]string, 0, len(replacements)),
	}
	fail := func(cause error) (*appliedFileReplacement, error) {
		rollbackErr := applied.Rollback()
		if rollbackErr != nil {
			return nil, errors.Join(cause, rollbackErr)
		}
		return nil, cause
	}

	targets := make([]string, 0, len(replacements)+len(removals))
	for _, replacement := range replacements {
		if filepath.Dir(replacement.destination) != parent || filepath.Dir(replacement.staged) != parent {
			return fail(fmt.Errorf("restore file %q is not staged beside its destination", replacement.destination))
		}
		targets = append(targets, replacement.destination)
	}
	for _, removal := range removals {
		if filepath.Dir(removal) != parent {
			return fail(fmt.Errorf("restore removal %q is outside the transaction directory", removal))
		}
		targets = append(targets, removal)
	}

	for index, target := range targets {
		if _, err := os.Lstat(target); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fail(err)
		}
		backupPath := filepath.Join(rollbackDir, fmt.Sprintf("%d", index))
		if err := os.Rename(target, backupPath); err != nil {
			return fail(err)
		}
		applied.backups[target] = backupPath
	}

	for _, replacement := range replacements {
		if err := os.Rename(replacement.staged, replacement.destination); err != nil {
			return fail(err)
		}
		applied.installed = append(applied.installed, replacement.destination)
	}

	if err := syncDirectory(parent); err != nil {
		return fail(err)
	}
	return applied, nil
}

func (replacement *appliedFileReplacement) Rollback() error {
	if replacement == nil || replacement.rollbackDir == "" {
		return nil
	}
	var rollbackErrors []error
	for index := len(replacement.installed) - 1; index >= 0; index-- {
		if err := os.Remove(replacement.installed[index]); err != nil && !os.IsNotExist(err) {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	for destination, backupPath := range replacement.backups {
		if err := os.Rename(backupPath, destination); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if replacement.parent != "" {
		if err := syncDirectory(replacement.parent); err != nil {
			rollbackErrors = append(rollbackErrors, err)
		}
	}
	if err := os.RemoveAll(replacement.rollbackDir); err != nil {
		rollbackErrors = append(rollbackErrors, err)
	}
	replacement.rollbackDir = ""
	return errors.Join(rollbackErrors...)
}

func (replacement *appliedFileReplacement) Commit() error {
	if replacement == nil || replacement.rollbackDir == "" {
		return nil
	}
	err := os.RemoveAll(replacement.rollbackDir)
	replacement.rollbackDir = ""
	return err
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func replaceDirectory(candidate, destination string) error {
	applied, err := applyDirectoryWithRollback(candidate, destination)
	if err != nil {
		return err
	}
	return applied.Commit()
}

type appliedDirectoryReplacement struct {
	destination  string
	rollbackDir  string
	rollbackPath string
	mounted      bool
	hadOriginal  bool
}

func applyDirectoryWithRollback(candidate, destination string) (*appliedDirectoryReplacement, error) {
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}

	if _, err := os.Lstat(destination); os.IsNotExist(err) {
		if err := os.Rename(candidate, destination); err != nil {
			return nil, err
		}
		if err := syncDirectory(parent); err != nil {
			_ = os.RemoveAll(destination)
			return nil, err
		}
		return &appliedDirectoryReplacement{destination: destination}, nil
	} else if err != nil {
		return nil, err
	}

	if isMountPoint(destination) {
		return applyMountedDirectoryWithRollback(candidate, destination)
	}

	rollbackDir, err := os.MkdirTemp(parent, ".nginx-ui-nginx-rollback-*")
	if err != nil {
		return nil, err
	}
	rollbackPath := filepath.Join(rollbackDir, "original")

	if err := os.Rename(destination, rollbackPath); err != nil {
		if isDeviceBusyError(err) {
			_ = os.RemoveAll(rollbackDir)
			return applyMountedDirectoryWithRollback(candidate, destination)
		}
		_ = os.RemoveAll(rollbackDir)
		return nil, err
	}
	if err := os.Rename(candidate, destination); err != nil {
		rollbackErr := os.Rename(rollbackPath, destination)
		_ = os.RemoveAll(rollbackDir)
		if rollbackErr != nil {
			return nil, errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	if err := syncDirectory(parent); err != nil {
		rollbackErr := os.RemoveAll(destination)
		if rollbackErr == nil {
			rollbackErr = os.Rename(rollbackPath, destination)
		}
		_ = os.RemoveAll(rollbackDir)
		if rollbackErr != nil {
			return nil, errors.Join(err, rollbackErr)
		}
		return nil, err
	}
	return &appliedDirectoryReplacement{
		destination:  destination,
		rollbackDir:  rollbackDir,
		rollbackPath: rollbackPath,
		hadOriginal:  true,
	}, nil
}

func applyMountedDirectoryWithRollback(candidate, destination string) (*appliedDirectoryReplacement, error) {
	parent := filepath.Dir(destination)
	rollbackRoot, err := os.MkdirTemp(parent, ".nginx-ui-nginx-mounted-rollback-*")
	if err != nil {
		return nil, err
	}
	rollbackPath := filepath.Join(rollbackRoot, "original")

	if err := copyDirectory(destination, rollbackPath); err != nil {
		_ = os.RemoveAll(rollbackRoot)
		return nil, fmt.Errorf("snapshot mounted Nginx directory: %w", err)
	}
	if err := cleanDirectoryPreservingStructure(destination); err != nil {
		_ = os.RemoveAll(rollbackRoot)
		return nil, fmt.Errorf("clear mounted Nginx directory: %w", err)
	}
	if err := copyDirectory(candidate, destination); err == nil {
		return &appliedDirectoryReplacement{
			destination:  destination,
			rollbackDir:  rollbackRoot,
			rollbackPath: rollbackPath,
			mounted:      true,
			hadOriginal:  true,
		}, nil
	} else {
		applyErr := err
		rollbackErr := cleanDirectoryPreservingStructure(destination)
		if rollbackErr == nil {
			rollbackErr = copyDirectory(rollbackPath, destination)
		}
		if rollbackErr != nil {
			_ = os.RemoveAll(rollbackRoot)
			return nil, errors.Join(applyErr, fmt.Errorf("rollback mounted Nginx directory: %w", rollbackErr))
		}
		_ = os.RemoveAll(rollbackRoot)
		return nil, applyErr
	}
}

func (replacement *appliedDirectoryReplacement) Rollback() error {
	if replacement == nil || replacement.destination == "" {
		return nil
	}
	var err error
	if replacement.mounted {
		err = cleanDirectoryPreservingStructure(replacement.destination)
		if err == nil {
			err = copyDirectory(replacement.rollbackPath, replacement.destination)
		}
	} else {
		err = os.RemoveAll(replacement.destination)
		if err == nil && replacement.hadOriginal {
			err = os.Rename(replacement.rollbackPath, replacement.destination)
		}
	}
	cleanupErr := os.RemoveAll(replacement.rollbackDir)
	replacement.destination = ""
	if err != nil && cleanupErr != nil {
		return errors.Join(err, cleanupErr)
	}
	if err != nil {
		return err
	}
	return cleanupErr
}

func (replacement *appliedDirectoryReplacement) Commit() error {
	if replacement == nil || replacement.destination == "" {
		return nil
	}
	var err error
	if replacement.rollbackDir != "" {
		err = os.RemoveAll(replacement.rollbackDir)
	}
	replacement.destination = ""
	return err
}
