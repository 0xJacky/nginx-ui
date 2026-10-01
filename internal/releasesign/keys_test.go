package releasesign

import (
	"testing"

	"aead.dev/minisign"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustedPublicKeysAreValidAndUnique(t *testing.T) {
	keys := TrustedPublicKeys()
	require.NotEmpty(t, keys)

	keyIDs := make(map[uint64]struct{}, len(keys))
	for _, encodedKey := range keys {
		var publicKey minisign.PublicKey
		require.NoError(t, publicKey.UnmarshalText([]byte(encodedKey)))
		_, duplicate := keyIDs[publicKey.ID()]
		assert.False(t, duplicate)
		keyIDs[publicKey.ID()] = struct{}{}
	}

	_, hasPrimaryKey := keyIDs[0xE099146682BA5032]
	assert.True(t, hasPrimaryKey)
}

func TestPluginPublicKeysAreValidAndApartFromReleaseKeys(t *testing.T) {
	releaseIDs := make(map[uint64]struct{})
	for _, encodedKey := range TrustedPublicKeys() {
		var publicKey minisign.PublicKey
		require.NoError(t, publicKey.UnmarshalText([]byte(encodedKey)))
		releaseIDs[publicKey.ID()] = struct{}{}
	}

	keys := PluginPublicKeys()
	require.NotEmpty(t, keys)
	pluginIDs := make(map[uint64]struct{}, len(keys))
	for _, encodedKey := range keys {
		var publicKey minisign.PublicKey
		require.NoError(t, publicKey.UnmarshalText([]byte(encodedKey)))
		_, duplicate := pluginIDs[publicKey.ID()]
		assert.False(t, duplicate)
		pluginIDs[publicKey.ID()] = struct{}{}
		_, isReleaseKey := releaseIDs[publicKey.ID()]
		assert.False(t, isReleaseKey, "a plugin key must never verify an nginx-ui release")
	}

	_, hasPrimaryKey := pluginIDs[0x088ACCA5E13F459F]
	assert.True(t, hasPrimaryKey)
}

func TestTrustedPublicKeysReturnsIsolatedCopy(t *testing.T) {
	keys := TrustedPublicKeys()
	keys[0] = "modified"
	assert.NotEqual(t, "modified", TrustedPublicKeys()[0])
}
