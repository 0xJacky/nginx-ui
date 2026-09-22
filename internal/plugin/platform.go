package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
)

// Platform keys follow the "<goos>-<goarch>" form of server.executables. The
// helpers below are shared by the catalog selection, the offline package
// directory, cluster sync, inspect and lint.

// knownGOOS and knownGOARCH are the values "go tool dist list" knows. They make
// the file name parser strict enough that a prerelease suffix of a version is
// never mistaken for a platform.
var (
	knownGOOS = []string{
		"aix", "android", "darwin", "dragonfly", "freebsd", "illumos", "ios", "js",
		"linux", "netbsd", "openbsd", "plan9", "solaris", "wasip1", "windows",
	}
	knownGOARCH = []string{
		"386", "amd64", "arm", "arm64", "loong64", "mips", "mips64", "mips64le",
		"mipsle", "ppc64", "ppc64le", "riscv64", "s390x", "wasm",
	}
)

// HostPlatform is the "<goos>-<goarch>" key of the running binary.
func HostPlatform() string {
	return runtime.GOOS + "-" + runtime.GOARCH
}

// IsValidPlatform reports whether platform is a "<goos>-<goarch>" pair Go can
// target.
func IsValidPlatform(platform string) bool {
	goos, goarch, ok := strings.Cut(platform, "-")
	return ok && slices.Contains(knownGOOS, goos) && slices.Contains(knownGOARCH, goarch)
}

// PackageName is a parsed package file name.
type PackageName struct {
	ID      string
	Version string
	// Platform is empty for a portable package.
	Platform string
}

// PackageFileName is the canonical file name of a package: the portable form
// "<id>-<version>.tar.gz" when platform is empty or "any", the per-platform
// form "<id>-<version>-<goos>-<goarch>.tar.gz" otherwise.
func PackageFileName(id, version, platform string) string {
	if platform == "" || platform == anyPlatform {
		return id + "-" + version + packageSuffix
	}
	return id + "-" + version + "-" + platform + packageSuffix
}

// ParsePackageFileName splits a file name of either form. ok is false for a
// name that is neither, e.g. an upload saved as "package.tar.gz".
func ParsePackageFileName(name string) (PackageName, bool) {
	stem, ok := strings.CutSuffix(filepath.Base(name), packageSuffix)
	if !ok {
		return PackageName{}, false
	}

	// A per-platform name ends with two tokens that form a known platform.
	if parts := strings.Split(stem, "-"); len(parts) >= 4 {
		platform := parts[len(parts)-2] + "-" + parts[len(parts)-1]
		if IsValidPlatform(platform) {
			rest := strings.Join(parts[:len(parts)-2], "-")
			if id, version, ok := splitIDVersion(rest); ok {
				return PackageName{ID: id, Version: version, Platform: platform}, true
			}
		}
	}

	id, version, ok := splitIDVersion(stem)
	if !ok {
		return PackageName{}, false
	}
	return PackageName{ID: id, Version: version}, true
}

// splitIDVersion finds the first hyphen that leaves a valid id on the left and
// a semantic version on the right. Ids may contain hyphens, versions start
// with a digit, so the first valid split is the intended one.
func splitIDVersion(stem string) (string, string, bool) {
	for index := 0; index < len(stem); index++ {
		if stem[index] != '-' {
			continue
		}
		id, version := stem[:index], stem[index+1:]
		if IsValidID(id) && semverPattern.MatchString(version) {
			return id, version, true
		}
	}
	return "", "", false
}

// packagePlatforms lists the platforms an extracted plugin in dir can run on,
// see packagePlatformsWith.
func packagePlatforms(manifest *protocol.Manifest, dir string) []string {
	return packagePlatformsWith(manifest, func(rel string) bool {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
		return err == nil && info.Mode().IsRegular()
	})
}

// packagePlatformsWith lists the platforms a plugin can run on: every
// "<goos>-<goarch>" whose executable is present, plus "any" when the plugin
// needs no executable of its own (no server, or an interpreted command that
// ResolveExecutable falls back to). present reports whether a regular file
// ships at a manifest relative path.
func packagePlatformsWith(manifest *protocol.Manifest, present func(rel string) bool) []string {
	if manifest == nil {
		return []string{}
	}
	if manifest.Server == nil {
		return []string{anyPlatform}
	}

	platforms := make([]string, 0, len(manifest.Server.Executables)+1)
	for platform, rel := range manifest.Server.Executables {
		if !isSafeRelPath(rel) || !present(rel) {
			continue
		}
		platforms = append(platforms, platform)
	}
	sort.Strings(platforms)
	if len(manifest.Server.Command) > 0 {
		platforms = append(platforms, anyPlatform)
	}
	return platforms
}

// platformsCover reports whether a platform list includes platform, either
// by name or through "any".
func platformsCover(platforms []string, platform string) bool {
	return slices.Contains(platforms, platform) || slices.Contains(platforms, anyPlatform)
}
