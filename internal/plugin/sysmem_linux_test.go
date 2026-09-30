//go:build linux

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLimitAt(t *testing.T) {
	root := t.TempDir()
	write := func(dir, value string) {
		full := filepath.Join(root, dir)
		require.NoError(t, os.MkdirAll(full, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(full, "memory.max"), []byte(value), 0o644))
	}
	write("a", "1073741824\n")
	write("a/b", "max\n")
	write("a/b/c", "536870912\n")

	assert.Equal(t, int64(536870912), memoryLimitAt(root, "0::/a/b/c\n"))
	assert.Equal(t, int64(1073741824), memoryLimitAt(root, "0::/a/b\n"))
	assert.Equal(t, int64(0), memoryLimitAt(root, "0::/missing\n"))
	assert.Equal(t, int64(0), memoryLimitAt(root, "1:cpu:/a\n"))
}
