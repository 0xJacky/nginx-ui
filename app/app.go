//go:build !unembed

package app

import (
	"archive/tar"
	"bytes"
	"embed"
	_ "embed"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	"github.com/ulikunitz/xz"
)

//go:embed dist.tar.xz
var compressedDist []byte

//go:embed i18n.json
var i18nJSON []byte

//go:embed src/language/* src/language/*/*
var languageFS embed.FS

var (
	DistFS  afero.Fs
	initErr error
)

func init() {
	DistFS, initErr = initDistFS()
}

// GetDistFS returns the initialized memory filesystem with decompressed frontend assets
func GetDistFS() (afero.Fs, error) {
	return DistFS, initErr
}

// initDistFS initializes the memory filesystem by decompressing the embedded assets
func initDistFS() (afero.Fs, error) {
	memFS := afero.NewMemMapFs()

	// Extract compressed dist archive
	if err := extractDistArchive(memFS); err != nil {
		return nil, err
	}

	// Copy i18n.json
	if err := afero.WriteFile(memFS, "i18n.json", i18nJSON, 0644); err != nil {
		return nil, err
	}

	// Copy language files from embed.FS to memory filesystem
	if err := copyLanguageFiles(memFS); err != nil {
		return nil, err
	}

	return memFS, nil
}

// extractDistArchive decompresses and extracts the dist.tar.xz archive
func extractDistArchive(memFS afero.Fs) error {
	if len(compressedDist) == 0 {
		return nil
	}

	xzReader, err := xz.NewReader(bytes.NewReader(compressedDist))
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(xzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Sanitize the file path to prevent directory traversal
		cleanPath := filepath.Clean(header.Name)

		// Ensure the path doesn't escape the target directory
		if strings.Contains(cleanPath, "..") || filepath.IsAbs(cleanPath) {
			// Skip entries with suspicious paths
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := memFS.MkdirAll(cleanPath, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			dir := filepath.Dir(cleanPath)
			if dir != "." {
				if err := memFS.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}

			file, err := memFS.Create(cleanPath)
			if err != nil {
				return err
			}

			if _, err := io.Copy(file, tarReader); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}
	}

	return nil
}

// copyLanguageFiles copies language files from embed.FS to memory filesystem
func copyLanguageFiles(memFS afero.Fs) error {
	return fs.WalkDir(languageFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return memFS.MkdirAll(path, 0755)
		}

		data, err := languageFS.ReadFile(path)
		if err != nil {
			return err
		}

		return afero.WriteFile(memFS, path, data, 0644)
	})
}

// HTTPFileSystem returns an http.FileSystem that serves from the memory filesystem
func HTTPFileSystem() (http.FileSystem, error) {
	fs, err := GetDistFS()
	if err != nil {
		return nil, err
	}
	return afero.NewHttpFs(fs), nil
}

// Open opens a file from the memory filesystem
func Open(name string) (afero.File, error) {
	fs, err := GetDistFS()
	if err != nil {
		return nil, err
	}

	name = strings.TrimPrefix(name, "/")
	return fs.Open(name)
}
