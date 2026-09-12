package backup

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateZipArchiveExcludesOutput(t *testing.T) {
	for _, location := range []string{"source", "nested", "outside", "hard_link"} {
		t.Run(location, func(t *testing.T) {
			source := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(source, "config.txt"), []byte("configuration"), 0600))
			// Existing archives are still user data and must not be excluded.
			require.NoError(t, os.WriteFile(filepath.Join(source, "previous.zip"), []byte("previous backup"), 0600))
			destination := source
			switch location {
			case "nested":
				destination = filepath.Join(source, "backups")
				require.NoError(t, os.Mkdir(destination, 0700))
			case "outside":
				destination = t.TempDir()
			}
			output := filepath.Join(destination, "current.zip")
			if location == "hard_link" {
				require.NoError(t, os.WriteFile(output, nil, 0600))
				if err := os.Link(output, filepath.Join(source, "alias.zip")); err != nil {
					t.Skipf("hard links unavailable: %v", err)
				}
			}
			require.NoError(t, createZipArchive(output, source))
			archive, err := zip.OpenReader(output)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, archive.Close()) })
			files := make(map[string]string)
			for _, entry := range archive.File {
				if entry.FileInfo().IsDir() {
					continue
				}
				reader, err := entry.Open()
				require.NoError(t, err)
				content, err := io.ReadAll(reader)
				require.NoError(t, err)
				require.NoError(t, reader.Close())
				files[entry.Name] = string(content)
			}
			require.Equal(t, map[string]string{
				"config.txt":   "configuration",
				"previous.zip": "previous backup",
			}, files)
		})
	}
}
