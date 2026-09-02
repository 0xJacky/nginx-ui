package helper

import (
	"encoding/base64"
	"slices"
	"strings"
	"unicode/utf8"
)

// EncodedPathPrefix marks a query value as a base64url-encoded filesystem path.
//
// Filesystem paths in query strings look exactly like directory-traversal
// attacks, and managed WAF rulesets block them: a request carrying
// `?log_path=/var/log/nginx/access.log` is refused at the edge before it ever
// reaches this application. Percent-encoding does not help — WAFs normalise it
// before matching, and the blocked request that prompted this was already
// percent-encoded.
//
// base64url output is drawn from [A-Za-z0-9_-], so an encoded path contains no
// slash, no dot and no recognisable path fragment for a signature to match.
const EncodedPathPrefix = "b64_"

// EncodePathParam renders a path in the form DecodePathParam accepts. Provided
// mainly for tests and for Go clients; the frontend has its own implementation
// in app/src/lib/helper/encodePathParam.ts and the two must agree.
func EncodePathParam(path string) string {
	return EncodedPathPrefix + base64.RawURLEncoding.EncodeToString([]byte(path))
}

// DecodePathParam returns the real path behind a query value, and whether the
// value was encoded.
//
// A value is treated as encoded only when all of the following hold:
//
//  1. it starts with EncodedPathPrefix
//  2. the remainder decodes as base64url (padded input is tolerated, since
//     third-party scripts may produce it)
//  3. the decoded bytes are valid UTF-8 and contain no NUL
//
// Otherwise the input is returned byte-for-byte and callers behave exactly as
// they did before this existed, which is what keeps existing scripts working.
//
// The three clauses together make misreading a real path impossible. Absolute
// paths start with '/', so clause 1 rejects them. Relative config paths contain
// '/' or '.', neither of which is in the base64url alphabet, so clause 2
// rejects them. A "try to decode, fall back to raw" heuristic would NOT be
// safe here: a bare name like `defaultsite` is itself valid base64url and would
// silently decode to binary.
//
// Callers must decode BEFORE validating containment. Validating first would let
// `../../../etc/passwd` hide inside the encoding and slip past the check.
func DecodePathParam(value string) (path string, encoded bool) {
	remainder, ok := strings.CutPrefix(value, EncodedPathPrefix)
	if !ok || remainder == "" {
		return value, false
	}

	decoded, err := base64.RawURLEncoding.DecodeString(remainder)
	if err != nil {
		// Tolerate padded input from clients that used the standard encoder.
		decoded, err = base64.URLEncoding.DecodeString(remainder)
		if err != nil {
			return value, false
		}
	}

	if !utf8.Valid(decoded) || slices.Contains(decoded, 0) {
		return value, false
	}

	return string(decoded), true
}
