package _github

import "embed"

//go:embed build/build_info.json
var DistFS embed.FS
