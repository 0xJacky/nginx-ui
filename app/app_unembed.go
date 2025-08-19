//go:build unembed

package app

import (
	"embed"
	"io/fs"
)

//go:embed i18n.json src/language/* src/language/*/*
var DistFS embed.FS

// GetDistFS returns the embedded filesystem for unembed build
func GetDistFS() (fs.FS, error) {
	return DistFS, nil
}
