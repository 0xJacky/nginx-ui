package helper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUnderDirectory(t *testing.T) {
	assert.Equal(t, true, IsUnderDirectory("/etc/nginx/nginx.conf", "/etc/nginx"))
	assert.Equal(t, false, IsUnderDirectory("../../root/nginx.conf", "/etc/nginx"))
	assert.Equal(t, false, IsUnderDirectory("/etc/nginx/../../root/nginx.conf", "/etc/nginx"))
	assert.Equal(t, false, IsUnderDirectory("/etc/nginx/../../etc/nginx/../../root/nginx.conf", "/etc/nginx"))
}

func TestIsUnderDirectoryRejectsExistingSymlinkEscape(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()

	linkPath := filepath.Join(baseDir, "escape")
	err := os.Symlink(outsideDir, linkPath)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	targetPath := filepath.Join(linkPath, "secret.txt")
	assert.False(t, IsUnderDirectory(targetPath, baseDir))
}

func TestIsUnderDirectoryRejectsNonExistingPathUnderSymlinkEscape(t *testing.T) {
	baseDir := t.TempDir()
	outsideDir := t.TempDir()

	linkPath := filepath.Join(baseDir, "escape")
	err := os.Symlink(outsideDir, linkPath)
	if err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	targetPath := filepath.Join(linkPath, "nested", "secret.txt")
	assert.False(t, IsUnderDirectory(targetPath, baseDir))
}

// TestIsUnderDirectoryAcceptsMissingWildcardName covers the wildcard
// certificate directory of issue #2067: a name that does not exist yet is
// joined onto its existing parent and stays inside the directory.
func TestIsUnderDirectoryAcceptsMissingWildcardName(t *testing.T) {
	baseDir := t.TempDir()

	assert.True(t, IsUnderDirectory(filepath.Join(baseDir, "ssl", "*.example.com_EC256"), baseDir))
	assert.True(t, IsUnderDirectory(filepath.Join(baseDir, "sites-available", "*.example.com"), baseDir))
}

func TestIsNonexistentPathError(t *testing.T) {
	_, err := os.Lstat(filepath.Join(t.TempDir(), "missing"))
	assert.True(t, IsNonexistentPathError(err))
	assert.True(t, IsNonexistentPathError(os.ErrNotExist))
	assert.False(t, IsNonexistentPathError(os.ErrPermission))
}
