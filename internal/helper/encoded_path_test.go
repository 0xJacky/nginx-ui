package helper

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodePathParamRoundTrip(t *testing.T) {
	paths := []string{
		"/var/log/nginx/access.log",
		"/etc/nginx/sites-available/example.com",
		"sites-available/example.com",
		"conf.d/gzip.conf",
		"/var/log/nginx/中文域名.log",
		"/etc/nginx/sites-available/name with spaces.conf",
		"/",
	}

	for _, path := range paths {
		got, encoded := DecodePathParam(EncodePathParam(path))
		assert.True(t, encoded, path)
		assert.Equal(t, path, got, path)
	}
}

func TestDecodePathParamPassesRawValuesThrough(t *testing.T) {
	// Every real path shape must come back byte-for-byte, so that existing
	// scripts and older frontends keep working unchanged.
	raw := []string{
		"/var/log/nginx/access.log",
		"/etc/nginx/nginx.conf",
		"sites-available/example.com",
		"conf.d/gzip.conf",
		"streams-available/tcp",
		"app.jwt_secret",
		"",
		"/",
		"../../../etc/passwd",
	}

	for _, value := range raw {
		got, encoded := DecodePathParam(value)
		assert.False(t, encoded, value)
		assert.Equal(t, value, got, value)
	}
}

func TestDecodePathParamRejectsMalformedPayloads(t *testing.T) {
	// Anything that fails a clause must fall back to the raw value rather than
	// producing garbage.
	cases := map[string]string{
		"empty remainder":     EncodedPathPrefix,
		"outside alphabet":    EncodedPathPrefix + "!!!!",
		"not valid base64":    EncodedPathPrefix + "a",
		"decodes to NUL byte": EncodedPathPrefix + base64.RawURLEncoding.EncodeToString([]byte("/etc\x00passwd")),
		"invalid utf-8":       EncodedPathPrefix + base64.RawURLEncoding.EncodeToString([]byte{0xff, 0xfe, 0xfd}),
	}

	for name, value := range cases {
		got, encoded := DecodePathParam(value)
		assert.False(t, encoded, name)
		assert.Equal(t, value, got, name)
	}
}

func TestDecodePathParamAcceptsPaddedInput(t *testing.T) {
	// A third-party client using the standard URL encoder emits padding.
	padded := EncodedPathPrefix + base64.URLEncoding.EncodeToString([]byte("/var/log/nginx/error.log"))

	got, encoded := DecodePathParam(padded)

	assert.True(t, encoded)
	assert.Equal(t, "/var/log/nginx/error.log", got)
}

func TestDecodePathParamNearCollision(t *testing.T) {
	// A config genuinely named "b64_defaultsite" is valid base64url after the
	// prefix, so it does decode — to bytes that are not a real path. That is
	// acceptable precisely because the caller still runs containment
	// validation on the result, which rejects it. What must NOT happen is a
	// silent read of some other file.
	got, encoded := DecodePathParam("b64_defaultsite")

	if encoded {
		assert.NotContains(t, got, "..", "a near-collision must never decode into a traversal")
		return
	}
	assert.Equal(t, "b64_defaultsite", got)
}

func TestDecodePathParamDoesNotHideTraversal(t *testing.T) {
	// The security property: encoding must not smuggle anything past the
	// caller's validation, because the caller validates the DECODED value.
	// Decoding is expected to succeed here — rejection is validation's job.
	got, encoded := DecodePathParam(EncodePathParam("../../../etc/passwd"))

	assert.True(t, encoded)
	assert.Equal(t, "../../../etc/passwd", got,
		"decode must surface the traversal so containment checks can reject it")
}

func TestEncodedValueSurvivesUnescapeURL(t *testing.T) {
	// Several handlers still run helper.UnescapeURL on raw values. Every
	// character the encoder emits is RFC 3986 unreserved, so an encoded value
	// must pass through that loop untouched.
	encoded := EncodePathParam("/etc/nginx/sites-available/example.com")

	assert.Equal(t, encoded, UnescapeURL(encoded))
}
