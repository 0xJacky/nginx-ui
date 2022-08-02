package frontend

import (
	"embed"
)

//go:embed dist/* dist/*/*
var DistFS embed.FS
