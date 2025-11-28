//go:build !unembed

package app

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed dist i18n.json src/language
var embeddedFS embed.FS

// GetDistFS returns the embedded filesystem with frontend assets
func GetDistFS() (fs.FS, error) {
	return embeddedFS, nil
}

// HTTPFileSystem returns an http.FileSystem that serves from the embedded filesystem
func HTTPFileSystem() (http.FileSystem, error) {
	fsys, err := GetDistFS()
	if err != nil {
		return nil, err
	}
	return http.FS(fsys), nil
}

// Open opens a file from the embedded filesystem
func Open(name string) (fs.File, error) {
	fsys, err := GetDistFS()
	if err != nil {
		return nil, err
	}

	name = strings.TrimPrefix(name, "/")
	return fsys.Open(name)
}
