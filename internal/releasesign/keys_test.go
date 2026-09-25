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

func TestTrustedPublicKeysReturnsIsolatedCopy(t *testing.T) {
	keys := TrustedPublicKeys()
	keys[0] = "modified"
	assert.NotEqual(t, "modified", TrustedPublicKeys()[0])
}
