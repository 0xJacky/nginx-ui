package config

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Query parameters carrying a filesystem path may arrive base64url-encoded so a
// WAF does not read them as traversal attempts. Handlers decode first and then
// validate, and these tests pin that ordering down: decoding must expose a
// traversal to the containment check, never hide it from one.
//
// If someone ever moves the decode call after validation, these fail.

func TestEncodedTraversalIsStillRejectedByContainment(t *testing.T) {
	hostile := []string{
		"../../../etc/passwd",
		"/etc/passwd",
		"../../etc/shadow",
		"/root/.ssh/id_rsa",
	}

	for _, raw := range hostile {
		// What the handler receives on the wire.
		wire := helper.EncodePathParam(raw)

		// Nothing path-shaped may survive into the URL, or the WAF blocks it
		// and the whole exercise is pointless.
		assert.NotContains(t, wire, "/", raw)
		assert.NotContains(t, wire, "..", raw)

		// Step 1: decode. The traversal must come back intact — a decoder that
		// sanitised here would hide the attack from the check below.
		decoded, encoded := helper.DecodePathParam(wire)
		require.True(t, encoded, raw)
		require.Equal(t, raw, decoded, raw)

		// Step 2: validate the decoded value. This is what must reject it.
		_, err := ResolveAbsoluteOrRelativeConfPath(decoded)
		assert.Error(t, err, "containment must reject %q after decoding", raw)
	}
}

func TestEncodedLegitimatePathStillResolves(t *testing.T) {
	// The mirror of the above: encoding must not break ordinary use.
	for _, raw := range []string{"sites-available/example.com", "conf.d/gzip.conf"} {
		decoded, encoded := helper.DecodePathParam(helper.EncodePathParam(raw))
		require.True(t, encoded, raw)
		require.Equal(t, raw, decoded, raw)

		resolved, err := ResolveAbsoluteOrRelativeConfPath(decoded)
		assert.NoError(t, err, raw)
		assert.Contains(t, resolved, raw, raw)
	}
}

func TestRawTraversalRemainsRejected(t *testing.T) {
	// Unencoded input must behave exactly as it did before this scheme existed.
	decoded, encoded := helper.DecodePathParam("../../../etc/passwd")
	require.False(t, encoded)
	require.Equal(t, "../../../etc/passwd", decoded)

	_, err := ResolveAbsoluteOrRelativeConfPath(decoded)
	assert.Error(t, err)
}
