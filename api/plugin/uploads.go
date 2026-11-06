package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	plugin "github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/uozi-tech/cosy"
)

const (
	// uploadStoreName is the directory under os.TempDir() that keeps
	// inspected packages until their install.
	uploadStoreName = "nginx-ui-plugin-uploads"
	// uploadTTL is how long an inspected package waits for its install.
	uploadTTL    = 15 * time.Minute
	uploadSuffix = ".tar.gz"
)

// uploadIDPattern is the only id shape allowed to reach the filesystem.
var uploadIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// errUploadStoreUnsafe means the store directory is not private to this user.
var errUploadStoreUnsafe = errors.New("plugin upload store is not private")

// uploadStoreDir returns the store directory, creating it on first use. The
// temp directory is shared, so a symlink or a directory another user planted
// there is refused.
func uploadStoreDir() (string, error) {
	dir := filepath.Join(os.TempDir(), uploadStoreName)
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || !isPrivateDir(info) {
		return "", errUploadStoreUnsafe
	}
	return dir, nil
}

// keepUpload moves an inspected package into the store and returns its id.
func keepUpload(archivePath string) (string, error) {
	dir, err := uploadStoreDir()
	if err != nil {
		return "", err
	}
	raw := make([]byte, 16)
	// Read never fails since Go 1.24.
	_, _ = rand.Read(raw)
	id := hex.EncodeToString(raw)
	if err = os.Rename(archivePath, filepath.Join(dir, id+uploadSuffix)); err != nil {
		return "", err
	}
	return id, nil
}

// takeUpload claims a kept package for one install. The file leaves the store
// right away, so the id cannot be used twice, and cleanup deletes it.
func takeUpload(id string) (string, func(), error) {
	noop := func() {}
	expired := cosy.WrapErrorWithParams(plugin.ErrPackageInvalid, "upload expired")
	if !uploadIDPattern.MatchString(id) {
		return "", noop, expired
	}
	dir, err := uploadStoreDir()
	if err != nil {
		return "", noop, err
	}

	claim, err := os.MkdirTemp("", "nginx-ui-plugin-upload-")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { _ = os.RemoveAll(claim) }

	target := filepath.Join(claim, "package.tar.gz")
	if err = os.Rename(filepath.Join(dir, id+uploadSuffix), target); err != nil {
		cleanup()
		if errors.Is(err, fs.ErrNotExist) {
			return "", noop, expired
		}
		return "", noop, err
	}
	return target, cleanup, nil
}

// sweepUploads deletes kept packages older than the TTL.
func sweepUploads() {
	dir, err := uploadStoreDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-uploadTTL)
	for _, item := range entries {
		id, ok := strings.CutSuffix(item.Name(), uploadSuffix)
		if !ok || !uploadIDPattern.MatchString(id) {
			continue
		}
		info, err := item.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, item.Name()))
	}
}
