//go:build !windows

package self_check

import "github.com/0xJacky/Nginx-UI/internal/nginx"

func resolvePath(path ...string) string {
	return nginx.GetConfPath(path...)
}
