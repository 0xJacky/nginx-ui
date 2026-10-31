//go:build unix

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadStoreRefusesAnUntrustedDirectory(t *testing.T) {
	// A directory others can write to is not trusted.
	_, store := useUploadStore(t)
	require.NoError(t, os.Mkdir(store, 0o700))
	require.NoError(t, os.Chmod(store, 0o777))
	_, err := uploadStoreDir()
	assert.ErrorIs(t, err, errUploadStoreUnsafe)

	// Neither is a symlink to a directory elsewhere.
	_, store = useUploadStore(t)
	require.NoError(t, os.Symlink(t.TempDir(), store))
	_, err = uploadStoreDir()
	assert.ErrorIs(t, err, errUploadStoreUnsafe)

	// Inspect still works, the install then needs a second upload.
	setupManager(t)
	result := inspectUpload(t, buildTestPackage(t, webappManifest("official.alpha"), nil))
	assert.Empty(t, result.UploadID)
	entries, err := os.ReadDir(filepath.Dir(store))
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), "nginx-ui-plugin-upload-")
	}
}
