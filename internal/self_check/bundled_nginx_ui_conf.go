package self_check

import (
	"errors"
	"os"
	"regexp"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/uozi-tech/cosy"
)

// bundledNginxUIConfPath is a var so tests can redirect it.
var bundledNginxUIConfPath = "/etc/nginx/conf.d/nginx-ui.conf"

// Markers indicating the WS reverse-proxy fix is present.
var (
	reMapForwardedProto    = regexp.MustCompile(`(?m)^\s*map\s+\$http_x_forwarded_proto\s+\$forwarded_proto\b`)
	reMapForwardedHost     = regexp.MustCompile(`(?m)^\s*map\s+\$http_x_forwarded_host\s+\$forwarded_host\b`)
	reHeaderForwardedProto = regexp.MustCompile(`(?m)^\s*proxy_set_header\s+X-Forwarded-Proto\s+\$forwarded_proto\b`)
	reHeaderForwardedHost  = regexp.MustCompile(`(?m)^\s*proxy_set_header\s+X-Forwarded-Host\s+\$forwarded_host\b`)
)

// CheckBundledNginxUIConf returns nil if the bundled conf has all expected fix markers,
// or ErrBundledNginxUIConfOutdated if any marker is missing.
// Outside of the official docker image, returns nil unconditionally.
func CheckBundledNginxUIConf() error {
	if !helper.InNginxUIOfficialDocker() {
		return nil
	}
	data, err := os.ReadFile(bundledNginxUIConfPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // delegated to other tasks
		}
		return cosy.WrapErrorWithParams(ErrFailedToReadBundledNginxUIConf, err.Error())
	}
	if !reMapForwardedProto.Match(data) ||
		!reMapForwardedHost.Match(data) ||
		!reHeaderForwardedProto.Match(data) ||
		!reHeaderForwardedHost.Match(data) {
		return ErrBundledNginxUIConfOutdated
	}
	return nil
}
