package nginx

import (
	"regexp"
	"strconv"
)

// reCoreVersion finds the nginx core version in `nginx -V` output. Builds
// such as Tengine name the core version in parentheses ("nginx/1.24.0"), and
// OpenResty numbers its releases after the core version it bundles.
var reCoreVersion = regexp.MustCompile(`(?:nginx|openresty)/(\d+)\.(\d+)\.(\d+)`)

// ParseCoreVersion extracts the nginx core version from `nginx -V` output.
// ok is false when the output names no version.
func ParseCoreVersion(output string) (major, minor, patch int, ok bool) {
	m := reCoreVersion.FindStringSubmatch(output)
	if m == nil {
		return 0, 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return major, minor, patch, true
}

// CoreVersionAtLeast reports whether the target nginx core version is at
// least major.minor.patch. ok is false when the version cannot be determined.
func CoreVersionAtLeast(major, minor, patch int) (atLeast, ok bool) {
	gotMajor, gotMinor, gotPatch, ok := ParseCoreVersion(getNginxV())
	if !ok {
		return false, false
	}
	if gotMajor != major {
		return gotMajor > major, true
	}
	if gotMinor != minor {
		return gotMinor > minor, true
	}
	return gotPatch >= patch, true
}
