package self_check

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
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
// Outside of managed bundled Nginx Docker mode, returns nil unconditionally.
func CheckBundledNginxUIConf() error {
	if !helper.ShouldManageBundledNginx() {
		return nil
	}
	data, err := os.ReadFile(bundledNginxUIConfPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // delegated to other tasks
		}
		return cosy.WrapErrorWithParams(ErrFailedToReadBundledNginxUIConf, err.Error())
	}
	if !hasBundledConfWebSocketFix(data) {
		return ErrBundledNginxUIConfOutdated
	}
	return nil
}

func hasBundledConfWebSocketFix(data []byte) bool {
	return reMapForwardedProto.Match(data) &&
		reMapForwardedHost.Match(data) &&
		reHeaderForwardedProto.Match(data) &&
		reHeaderForwardedHost.Match(data)
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

// patchOnDiskWithBackup atomically writes the patched contents to bundledNginxUIConfPath.
// On any failure after the backup file exists, the target is restored from the backup
// and an error wrapping the backup path is returned.
//
// Restore errors are folded into the returned error message; ErrCriticalRecoveryFailed
// is reserved for the verify/reload layer (Phase 3.7), where a failed restore after
// nginx -t rejection is a distinct fault class operators need to triage specifically.
func patchOnDiskWithBackup(orig []byte, bak string) error {
	patched := applyBundledConfPatch(orig)
	if !hasBundledConfWebSocketFix(patched) {
		return cosy.WrapErrorWithParams(ErrFixedConfigInvalid,
			"unable to apply all required WebSocket reverse-proxy markers; backup at "+bak)
	}
	tmp := bundledNginxUIConfPath + ".tmp"
	if err := os.WriteFile(tmp, patched, 0o644); err != nil {
		_ = restoreFromBackup(bundledNginxUIConfPath, bak)
		return cosy.WrapErrorWithParams(ErrFixedConfigInvalid,
			"write failed: "+err.Error()+"; restored from "+bak)
	}
	if err := os.Rename(tmp, bundledNginxUIConfPath); err != nil {
		_ = os.Remove(tmp)
		_ = restoreFromBackup(bundledNginxUIConfPath, bak)
		return cosy.WrapErrorWithParams(ErrFixedConfigInvalid,
			"rename failed: "+err.Error()+"; restored from "+bak)
	}
	return nil
}

// restoreFromBackup copies the contents of bak over target.
func restoreFromBackup(target, bak string) error {
	data, err := os.ReadFile(bak)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

// FixBundledNginxUIConf is the FixFunc for the bundled nginx-ui.conf upgrade
// self_check task (registered in tasks.go).
// Flow: read -> backup -> patch -> atomic write -> nginx -t -> reload.
// On any failure between backup and verify the file is rolled back; the error
// always includes the backup path.
func FixBundledNginxUIConf() error {
	orig, err := os.ReadFile(bundledNginxUIConfPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToReadBundledNginxUIConf, err.Error())
	}

	bak := fmt.Sprintf("%s.bak.%s", bundledNginxUIConfPath, time.Now().Format("20060102150405"))
	if err := os.WriteFile(bak, orig, 0o644); err != nil {
		return cosy.WrapErrorWithParams(ErrFailedToCreateBackup, err.Error())
	}

	if err := patchOnDiskWithBackup(orig, bak); err != nil {
		return err
	}
	return verifyAndReload(bak)
}

// verifyAndReload runs `nginx -t` and rolls back on failure, then reloads.
// Reload failures do NOT trigger rollback because the file on disk is already valid.
func verifyAndReload(bak string) error {
	if out, err := nginx.TestConfig(); err != nil {
		if rerr := restoreFromBackup(bundledNginxUIConfPath, bak); rerr != nil {
			return cosy.WrapErrorWithParams(ErrCriticalRecoveryFailed,
				"validate failed: "+strings.TrimSpace(out)+
					"; restore failed: "+rerr.Error()+
					"; backup at "+bak)
		}
		return cosy.WrapErrorWithParams(ErrFixedConfigInvalid,
			strings.TrimSpace(out)+"; restored from "+bak)
	}
	if out, err := nginx.Reload(); err != nil {
		return cosy.WrapErrorWithParams(ErrReloadFailed,
			strings.TrimSpace(out)+"; backup at "+bak)
	}
	return nil
}
