//go:build windows

package nginx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// TestExistsHelpersAcceptInvalidNameOnWindows pins the platform behavior
// behind issue #2067: Stat of a "*" name fails with ERROR_INVALID_NAME, which
// os.IsNotExist does not match, so the exists helpers must handle it.
func TestExistsHelpersAcceptInvalidNameOnWindows(t *testing.T) {
	useLocalTargetFilesystem(t)
	sitesEnabled := filepath.Join(t.TempDir(), "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesEnabled, 0o755))
	path := filepath.Join(sitesEnabled, "*.example.com.conf")

	_, err := Stat(path)
	require.ErrorIs(t, err, windows.ERROR_INVALID_NAME)
	require.False(t, os.IsNotExist(err))

	_, err = Lstat(path)
	require.ErrorIs(t, err, windows.ERROR_INVALID_NAME)

	assertExistsHelpers(t, path, false, false, false)
}
