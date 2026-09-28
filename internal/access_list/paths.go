package access_list

import (
	"regexp"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// IncludeDir is where rendered access lists live, relative to the Nginx
// configuration directory. It must not be a directory that nginx.conf already
// includes with a wildcard at http level (conf.d is), or every list would
// restrict every site.
const IncludeDir = "nginx-ui/access"

// FileExt is the extension of a rendered access list.
const FileExt = ".conf"

var (
	slugPattern    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	includePattern = regexp.MustCompile(`(?:^|/)nginx-ui/access/([a-z0-9][a-z0-9-]{0,62})\.conf$`)
)

// Dir returns the absolute directory that holds the rendered access lists.
func Dir() string {
	return nginx.GetConfPath(IncludeDir)
}

// FilePath returns the absolute path of the rendered file of a list.
func FilePath(slug string) string {
	return nginx.GetConfPath(IncludeDir, slug+FileExt)
}

// IncludePath returns the path written into include directives. It is
// relative, so Nginx resolves it against its own configuration prefix and the
// sandbox used by `nginx -t` resolves it against the sandbox copy.
func IncludePath(slug string) string {
	return IncludeDir + "/" + slug + FileExt
}

// SlugFromInclude returns the slug an include parameter points at, accepting
// relative and absolute spellings of the access list directory.
func SlugFromInclude(param string) (string, bool) {
	param = strings.ReplaceAll(strings.TrimSpace(param), `\`, "/")
	match := includePattern.FindStringSubmatch(param)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// ValidSlug reports whether slug can name an access list file.
func ValidSlug(slug string) bool {
	return slugPattern.MatchString(slug)
}
