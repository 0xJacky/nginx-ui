//go:build unembed

package app

import "embed"

//go:embed i18n.json src/language/* src/language/*/*
var DistFS embed.FS
