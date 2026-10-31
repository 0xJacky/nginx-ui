package plugin

import (
	"strconv"
	"strings"
)

// The repo carries no semver dependency, so the plugin manager implements the
// small subset the manifest needs: comparing two versions and matching a
// version against a dependency range.

// semver is a parsed semantic version. Build metadata is dropped because it
// never takes part in precedence.
type semver struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

// ParseVersion reports whether v is a semantic version and returns its parts.
func parseVersion(v string) (semver, bool) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return semver{}, false
	}
	// Build metadata never affects precedence.
	if idx := strings.IndexByte(v, '+'); idx >= 0 {
		v = v[:idx]
	}
	var prerelease string
	if idx := strings.IndexByte(v, '-'); idx >= 0 {
		prerelease = v[idx+1:]
		v = v[:idx]
	}

	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return semver{}, false
	}
	numbers := make([]int, 3)
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return semver{}, false
		}
		numbers[i] = n
	}
	return semver{major: numbers[0], minor: numbers[1], patch: numbers[2], prerelease: prerelease}, true
}

// HostVersionSatisfies reports whether a host of version current meets a
// plugin's min_nginx_ui_version. A minimum without a pre-release part also
// accepts the pre-releases of its own version, so a plugin that needs 3.0.0
// runs on 3.0.0-beta.1; a minimum that names a pre-release is compared as is.
func HostVersionSatisfies(current, required string) bool {
	if have, ok := parseVersion(current); ok {
		if need, ok := parseVersion(required); ok && need.prerelease == "" {
			have.prerelease = ""
			return compareSemver(have, need) >= 0
		}
	}
	return CompareVersions(current, required) >= 0
}

// CompareVersions orders two semantic versions, returning -1, 0 or 1. An
// unparsable version sorts before a parsable one.
func CompareVersions(a, b string) int {
	left, leftOK := parseVersion(a)
	right, rightOK := parseVersion(b)
	switch {
	case !leftOK && !rightOK:
		return strings.Compare(a, b)
	case !leftOK:
		return -1
	case !rightOK:
		return 1
	}
	return compareSemver(left, right)
}

func compareSemver(a, b semver) int {
	for _, pair := range [][2]int{{a.major, b.major}, {a.minor, b.minor}, {a.patch, b.patch}} {
		if pair[0] != pair[1] {
			if pair[0] < pair[1] {
				return -1
			}
			return 1
		}
	}
	return comparePrerelease(a.prerelease, b.prerelease)
}

// comparePrerelease implements the semver 2.0.0 precedence rules for the
// pre-release part: a release outranks any pre-release, identifiers are
// compared left to right, numeric ones numerically.
func comparePrerelease(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}

	left := strings.Split(a, ".")
	right := strings.Split(b, ".")
	for i := 0; i < len(left) && i < len(right); i++ {
		if left[i] == right[i] {
			continue
		}
		leftNum, leftErr := strconv.Atoi(left[i])
		rightNum, rightErr := strconv.Atoi(right[i])
		switch {
		case leftErr == nil && rightErr == nil:
			if leftNum < rightNum {
				return -1
			}
			return 1
		case leftErr == nil:
			// Numeric identifiers always have lower precedence.
			return -1
		case rightErr == nil:
			return 1
		default:
			return strings.Compare(left[i], right[i])
		}
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return 0
}

// VersionSatisfies matches a version against a dependency range. The grammar
// is the common subset of the npm one: "||" separated alternatives, each a
// whitespace or comma separated list of comparators that must all hold.
// A comparator is an optional operator (=, >, >=, <, <=, ^, ~) and a version,
// an empty string or "*" match everything.
func VersionSatisfies(version, versionRange string) bool {
	versionRange = strings.TrimSpace(versionRange)
	if versionRange == "" || versionRange == "*" {
		return true
	}
	target, ok := parseVersion(version)
	if !ok {
		return false
	}
	for _, alternative := range strings.Split(versionRange, "||") {
		if matchAllComparators(target, alternative) {
			return true
		}
	}
	return false
}

func matchAllComparators(target semver, alternative string) bool {
	fields := strings.FieldsFunc(alternative, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ','
	})
	if len(fields) == 0 {
		return false
	}
	for _, comparator := range fields {
		if !matchComparator(target, comparator) {
			return false
		}
	}
	return true
}

func matchComparator(target semver, comparator string) bool {
	comparator = strings.TrimSpace(comparator)
	if comparator == "" || comparator == "*" {
		return true
	}

	operator := "="
	for _, candidate := range []string{">=", "<=", "^", "~", ">", "<", "="} {
		if rest, found := strings.CutPrefix(comparator, candidate); found {
			operator = candidate
			comparator = rest
			break
		}
	}
	bound, ok := parseVersion(comparator)
	if !ok {
		return false
	}

	order := compareSemver(target, bound)
	switch operator {
	case ">":
		return order > 0
	case ">=":
		return order >= 0
	case "<":
		return order < 0
	case "<=":
		return order <= 0
	case "^":
		// Compatible with the left-most non-zero component.
		if order < 0 {
			return false
		}
		switch {
		case bound.major > 0:
			return target.major == bound.major
		case bound.minor > 0:
			return target.major == 0 && target.minor == bound.minor
		default:
			return target.major == 0 && target.minor == 0
		}
	case "~":
		// Patch level changes only.
		return order >= 0 && target.major == bound.major && target.minor == bound.minor
	default:
		return order == 0
	}
}
