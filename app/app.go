package app

import (
	"embed"
)

//go:embed dist/* dist/*/* src/language/* src/language/*/*
var DistFS embed.FS
