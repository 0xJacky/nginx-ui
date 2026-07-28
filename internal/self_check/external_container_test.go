package self_check

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uozi-tech/cosy"
)

func TestCheckExternalContainerConfigShared(t *testing.T) {
	t.Run("shared path is visible and cleaned up", func(t *testing.T) {
		configDir := t.TempDir()
		var checkPath string

		err := checkExternalContainerConfigShared(configDir, func(path string) bool {
			checkPath = path
			content, readErr := os.ReadFile(path)
			require.NoError(t, readErr)
			assert.Equal(t, externalConfigCheckContent, string(content))
			return true
		})

		require.NoError(t, err)
		assert.NotEmpty(t, checkPath)
		_, statErr := os.Stat(checkPath)
		assert.True(t, errors.Is(statErr, os.ErrNotExist))
	})

	t.Run("unshared path returns a scoped error and cleans up", func(t *testing.T) {
		configDir := t.TempDir()
		var checkPath string

		err := checkExternalContainerConfigShared(configDir, func(path string) bool {
			checkPath = path
			return false
		})

		require.Error(t, err)
		var cosyErr *cosy.Error
		require.ErrorAs(t, err, &cosyErr)
		assert.Equal(t, int32(40422), cosyErr.Code)
		_, statErr := os.Stat(checkPath)
		assert.True(t, errors.Is(statErr, os.ErrNotExist))
	})

	t.Run("unwritable path reports verification failure", func(t *testing.T) {
		missingDir := filepath.Join(t.TempDir(), "missing")

		err := checkExternalContainerConfigShared(missingDir, func(string) bool {
			t.Fatal("statPath must not be called when the local probe cannot be created")
			return false
		})

		require.Error(t, err)
		var cosyErr *cosy.Error
		require.ErrorAs(t, err, &cosyErr)
		assert.Equal(t, int32(50011), cosyErr.Code)
	})
}
