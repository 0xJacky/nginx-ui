package middleware

import (
	"net/http"
	"strings"
)

// encodedPathSeparators are the escapes of the octets whose meaning in a path
// depends on who decodes it: a dot builds dot segments once decoded, and an
// encoded slash or backslash is a separator to some proxies and data to
// others. No client needs them: a dot is unreserved and never escaped, and
// both Gin and the plugin passthrough decode a path before routing, so an
// encoded slash means the same as a plain one on every route Proxy serves.
var encodedPathSeparators = []string{"%2e", "%2f", "%5c"}

// isCanonicalProxyPath reports whether the path of r means the same thing to
// every hop between the controller and a child node.
//
// The controller authorizes a request by the route it matched on the decoded
// path, then forwards the path as received. A proxy in front of the child may
// normalize the path first, resolving dot segments, decoding escaped dots and
// slashes, treating backslashes as separators or merging slashes, so the child
// can serve another route than the one authorized. A path is canonical when no
// such normalization can change it:
//   - the decoded path is absolute and has no "." or ".." segment, no empty
//     segment other than a trailing slash, and no backslash or NUL;
//   - the raw path has no escaped dot, slash or backslash.
//
// Dots inside a segment, such as "example.com.conf" or "a..b", are kept.
func isCanonicalProxyPath(r *http.Request) bool {
	decoded := r.URL.Path
	if !strings.HasPrefix(decoded, "/") || strings.ContainsAny(decoded, "\\\x00") {
		return false
	}
	segments := strings.Split(decoded[1:], "/")
	for i, segment := range segments {
		switch segment {
		case ".", "..":
			return false
		case "":
			if i != len(segments)-1 {
				return false
			}
		}
	}

	// The HTTP proxy forwards the escaped path, the WebSocket proxy the
	// request URI; check both raw forms.
	rawPaths := []string{r.URL.EscapedPath()}
	if r.RequestURI != "" {
		requestPath, _, _ := strings.Cut(r.RequestURI, "?")
		rawPaths = append(rawPaths, requestPath)
	}
	for _, raw := range rawPaths {
		raw = strings.ToLower(raw)
		for _, encoded := range encodedPathSeparators {
			if strings.Contains(raw, encoded) {
				return false
			}
		}
	}
	return true
}
