package dns

import (
	"strings"

	"golang.org/x/net/idna"
)

// ToASCIIName converts a domain or record name to its canonical ASCII form,
// encoding internationalized labels as punycode so the Unicode and ASCII
// spellings of the same name compare equal.
//
// Conversion runs per label and leaves labels that are not valid IDN labels
// untouched, which preserves DNS constructs such as the "*" of a wildcard
// record or an "_acme-challenge" prefix.
func ToASCIIName(value string) string {
	return convertName(value, idna.Lookup.ToASCII)
}

// ToUnicodeName converts a domain or record name to its Unicode form, decoding
// punycode labels back into the spelling a user recognises.
func ToUnicodeName(value string) string {
	return convertName(value, idna.Lookup.ToUnicode)
}

func convertName(value string, convert func(string) (string, error)) string {
	name := strings.TrimSpace(value)
	if name == "" {
		return ""
	}

	// The whole name is converted first because IDNA mapping also folds separators
	// that are not an ASCII dot, such as the ideographic full stop (U+3002) a
	// Chinese or Japanese input method produces.
	if converted, err := convert(name); err == nil {
		return strings.ToLower(strings.Trim(converted, "."))
	}

	// Whole-name conversion fails as soon as a single label is not a valid IDN
	// label, which covers DNS constructs that are legal here (the "*" of a
	// wildcard record) as well as labels IDNA2008 rejects but this project has
	// always accepted (a "--" in the third and fourth position). Converting label
	// by label keeps those untouched instead of discarding the whole name, and
	// leaves the caller's own validation to report genuinely invalid input.
	labels := strings.Split(strings.Trim(name, "."), ".")
	for i, label := range labels {
		if converted, err := convert(label); err == nil {
			labels[i] = converted
		}
	}

	return strings.ToLower(strings.Join(labels, "."))
}
