package nginx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// useLocalTargetFilesystem points the nginx filesystem helpers at the local
// file system for the duration of the test.
func useLocalTargetFilesystem(t *testing.T) {
	t.Helper()
	original := resolveTargetFilesystem
	resolveTargetFilesystem = func() (targetFilesystem, error) { return localTargetFilesystem{}, nil }
	t.Cleanup(func() { resolveTargetFilesystem = original })
}

// assertExistsHelpers checks Exists, EntryExists and SymlinkExists for path.
func assertExistsHelpers(t *testing.T, path string, exists, entryExists, symlinkExists bool) {
	t.Helper()

	got, err := Exists(path)
	assert.NoError(t, err)
	assert.Equal(t, exists, got, "Exists")

	got, err = EntryExists(path)
	assert.NoError(t, err)
	assert.Equal(t, entryExists, got, "EntryExists")

	got, err = SymlinkExists(path)
	assert.NoError(t, err)
	assert.Equal(t, symlinkExists, got, "SymlinkExists")
}

func TestExistsHelpersReportMissingPathWithoutError(t *testing.T) {
	useLocalTargetFilesystem(t)

	assertExistsHelpers(t, filepath.Join(t.TempDir(), "sites-enabled", "missing.conf"), false, false, false)
}

func TestExistsHelpersReportExistingFile(t *testing.T) {
	useLocalTargetFilesystem(t)
	path := filepath.Join(t.TempDir(), "site.conf")
	require.NoError(t, os.WriteFile(path, []byte("server {}\n"), 0o644))

	assertExistsHelpers(t, path, true, true, false)
}

// TestExistsHelpersReportWildcardNameAsMissing covers issue #2067: wildcard
// certificate names such as "*.example.com" end up in site config paths. On
// Windows the file system rejects "*" with ERROR_INVALID_NAME, elsewhere the
// file simply does not exist; both must read as missing, not as an error.
func TestExistsHelpersReportWildcardNameAsMissing(t *testing.T) {
	useLocalTargetFilesystem(t)
	sitesEnabled := filepath.Join(t.TempDir(), "sites-enabled")
	require.NoError(t, os.MkdirAll(sitesEnabled, 0o755))

	assertExistsHelpers(t, filepath.Join(sitesEnabled, "*.example.com.conf"), false, false, false)
}

func TestExistsHelpersReturnTargetFilesystemError(t *testing.T) {
	original := resolveTargetFilesystem
	resolveTargetFilesystem = func() (targetFilesystem, error) { return nil, ErrInvalidTargetFilesystem }
	t.Cleanup(func() { resolveTargetFilesystem = original })

	_, err := Exists("/etc/nginx/nginx.conf")
	assert.ErrorIs(t, err, ErrInvalidTargetFilesystem)
	_, err = EntryExists("/etc/nginx/nginx.conf")
	assert.ErrorIs(t, err, ErrInvalidTargetFilesystem)
	_, err = SymlinkExists("/etc/nginx/nginx.conf")
	assert.ErrorIs(t, err, ErrInvalidTargetFilesystem)
}
