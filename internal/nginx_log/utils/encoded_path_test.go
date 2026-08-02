package utils

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /api/nginx_log/preflight?log_path=... is the request a managed WAF
// ruleset was measured blocking, so the log path is now sent base64url-encoded.
// The handler decodes before calling IsValidLogPath; these tests pin that
// ordering, because reversing it would let a path outside the whitelist hide
// inside the encoding.

func withLogWhitelist(t *testing.T, dirs ...string) {
	t.Helper()

	previous := settings.NginxSettings.LogDirWhiteList
	settings.NginxSettings.LogDirWhiteList = dirs
	t.Cleanup(func() { settings.NginxSettings.LogDirWhiteList = previous })
}

func TestEncodedLogPathOutsideWhitelistIsRejected(t *testing.T) {
	withLogWhitelist(t, "/var/log/nginx")

	for _, raw := range []string{
		"/etc/passwd",
		"/var/log/nginx/../../etc/shadow",
		"/root/.bash_history",
	} {
		wire := helper.EncodePathParam(raw)

		// Nothing traversal-shaped may remain in the wire value.
		assert.NotContains(t, wire, "/", raw)
		assert.NotContains(t, wire, "..", raw)

		decoded, encoded := helper.DecodePathParam(wire)
		require.True(t, encoded, raw)
		require.Equal(t, raw, decoded, raw)

		assert.False(t, IsValidLogPath(decoded),
			"whitelist must reject %q after decoding", raw)
	}
}

func TestEncodedWhitelistedLogPathIsAccepted(t *testing.T) {
	withLogWhitelist(t, "/var/log/nginx")

	raw := "/var/log/nginx/access.log"
	decoded, encoded := helper.DecodePathParam(helper.EncodePathParam(raw))

	require.True(t, encoded)
	require.Equal(t, raw, decoded)
	assert.True(t, IsValidLogPath(decoded))
}

func TestRawLogPathBehaviourUnchanged(t *testing.T) {
	withLogWhitelist(t, "/var/log/nginx")

	// An unencoded value must behave exactly as before the scheme existed.
	decoded, encoded := helper.DecodePathParam("/var/log/nginx/access.log")
	require.False(t, encoded)
	assert.True(t, IsValidLogPath(decoded))

	decoded, encoded = helper.DecodePathParam("/etc/passwd")
	require.False(t, encoded)
	assert.False(t, IsValidLogPath(decoded))
}
