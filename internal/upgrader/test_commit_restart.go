package upgrader

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"code.pfad.fr/risefront"
	"github.com/minio/selfupdate"
	"github.com/pkg/errors"
)

func (u *Upgrader) TestCommitAndRestart() error {
	// Get the directory of the current executable
	exDir := filepath.Dir(u.ExPath)
	testBinaryPath := filepath.Join(exDir, "nginx-ui.test")

	// Create temporary old file path
	oldExe := filepath.Join(exDir, ".nginx-ui.old."+strconv.FormatInt(time.Now().Unix(), 10))

	// Setup update options
	opts := selfupdate.Options{
		OldSavePath: oldExe,
	}

	// Check permissions
	if err := opts.CheckPermissions(); err != nil {
		return err
	}

	// Copy current executable to test file
	srcFile, err := os.Open(u.ExPath)
	if err != nil {
		return errors.Wrap(err, "failed to open source executable")
	}
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.Create(testBinaryPath)
	if err != nil {
		return errors.Wrap(err, "failed to create test executable")
	}
	defer destFile.Close()

	// Copy file content
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return errors.Wrap(err, "failed to copy executable content")
	}

	// Set executable permissions
	if err = destFile.Chmod(0755); err != nil {
		return errors.Wrap(err, "failed to set executable permission")
	}

	// Reopen file for selfupdate
	srcFile.Close()
	srcFile, err = os.Open(testBinaryPath)
	if err != nil {
		return errors.Wrap(err, "failed to open test executable for update")
	}
	defer srcFile.Close()

	// Prepare and check binary
	if err = selfupdate.PrepareAndCheckBinary(srcFile, opts); err != nil {
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return pathErr.Err
		}
		return err
	}

	// Commit binary update
	if err = selfupdate.CommitBinary(opts); err != nil {
		if rerr := selfupdate.RollbackError(err); rerr != nil {
			return rerr
		}
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return pathErr.Err
		}
		return err
	}

	if runtime.GOOS != "windows" {
		_ = os.Remove(oldExe)
	}

	// Wait for file to be written
	time.Sleep(1 * time.Second)
	
	// Gracefully restart
	risefront.Restart()
	return nil
}
