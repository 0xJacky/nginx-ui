package self_check

import (
	"errors"
	"os"
	"regexp"
	"strings"

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

// Patterns to rewrite when an old default is still in place.
var (
	reHeaderForwardedProtoLegacy = regexp.MustCompile(`(?m)^(\s*proxy_set_header\s+X-Forwarded-Proto\s+)\$scheme(\s*;)`)
	reHeaderForwardedHostLegacy  = regexp.MustCompile(`(?m)^(\s*proxy_set_header\s+X-Forwarded-Host\s+)\$http_host(\s*;)`)
)

const mapForwardedProtoBlock = `# Preserve X-Forwarded-Proto from an outer reverse proxy (e.g. host nginx
# terminating TLS in front of this container). Only fall back to $scheme
# when the inbound request did not carry the header.
map $http_x_forwarded_proto $forwarded_proto {
    default $http_x_forwarded_proto;
    ''      $scheme;
}

`

const mapForwardedHostBlock = `# Same for X-Forwarded-Host: keep what the outer proxy stamped, otherwise
# use the inbound Host header.
map $http_x_forwarded_host $forwarded_host {
    default $http_x_forwarded_host;
    ''      $http_host;
}

`

// applyBundledConfPatch returns a patched copy of in. Idempotent.
func applyBundledConfPatch(in []byte) []byte {
	out := in
	out = reHeaderForwardedProtoLegacy.ReplaceAll(out, []byte("${1}$$forwarded_proto${2}"))
	out = reHeaderForwardedHostLegacy.ReplaceAll(out, []byte("${1}$$forwarded_host${2}"))

	var injection strings.Builder
	if !reMapForwardedProto.Match(out) {
		injection.WriteString(mapForwardedProtoBlock)
	}
	if !reMapForwardedHost.Match(out) {
		injection.WriteString(mapForwardedHostBlock)
	}
	if injection.Len() > 0 {
		out = injectBeforeFirstServer(out, injection.String())
	}
	return out
}

// reFirstServer matches the first top-level `server {` for map injection.
var reFirstServer = regexp.MustCompile(`(?m)^server\s*\{`)

// injectBeforeFirstServer inserts s before the first top-level `server {`.
// Falls back to prepend if no server block is found.
func injectBeforeFirstServer(in []byte, s string) []byte {
	idx := reFirstServer.FindIndex(in)
	if idx == nil {
		return append([]byte(s), in...)
	}
	out := make([]byte, 0, len(in)+len(s))
	out = append(out, in[:idx[0]]...)
	out = append(out, s...)
	out = append(out, in[idx[0]:]...)
	return out
}
