//go:build windows

package helper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/windows"
)

// TestIsNonexistentPathErrorAcceptsInvalidName covers issue #2067: Windows
// reports "*" in a name as ERROR_INVALID_NAME, which must count as missing.
func TestIsNonexistentPathErrorAcceptsInvalidName(t *testing.T) {
	_, err := os.Lstat(filepath.Join(t.TempDir(), "*.example.com_EC256"))
	assert.True(t, isNonexistentPathError(err), fmt.Sprint(err))
	assert.True(t, isNonexistentPathError(&os.PathError{Op: "lstat", Path: "*", Err: windows.ERROR_INVALID_NAME}))
}

// TestIsUnderDirectoryAcceptsInvalidNameOnWindows makes sure a wildcard name
// under the directory no longer fails the containment check, while a wildcard
// name outside it is still rejected.
func TestIsUnderDirectoryAcceptsInvalidNameOnWindows(t *testing.T) {
	baseDir := t.TempDir()

	assert.True(t, IsUnderDirectory(filepath.Join(baseDir, "ssl", "*.example.com_EC256"), baseDir))
	assert.False(t, IsUnderDirectory(filepath.Join(baseDir, "..", "*.example.com_EC256"), baseDir))
}
