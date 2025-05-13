//go:build windows

package self_check

import "path/filepath"

// fix #1046
// include conf.d/*.conf
// inclde sites-enabled/*.conf

func resolvePath(path ...string) string {
	return filepath.ToSlash(filepath.Join(path...) + ".conf")
}
