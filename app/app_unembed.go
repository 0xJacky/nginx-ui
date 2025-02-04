//go:build unembed

package app

import "embed"

//go:embed i18n.json src/language/* src/language/*/* src/version.json
var DistFS embed.FS

var VersionPath = "src/version.json"
