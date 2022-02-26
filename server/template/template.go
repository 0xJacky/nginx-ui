package template

import "embed"

//go:embed http-conf https-conf
var DistFS embed.FS
