package user

import (
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	internalcrypto "github.com/0xJacky/Nginx-UI/internal/crypto"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifySecureSessionIDRequiresCanonicalUUID(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	sessionID := SetSecureSessionID(42)
	require.True(t, VerifySecureSessionID(sessionID, 42))
	assert.False(t, VerifySecureSessionID(strings.ToUpper(sessionID), 42))
	assert.False(t, VerifySecureSessionID(strings.ReplaceAll(sessionID, "-", ""), 42))
	assert.False(t, VerifySecureSessionID(" "+sessionID+" ", 42))
}

func TestVerifySecureSessionIDRejectsWrongCacheTypeWithoutPanic(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	sessionID := uuid.NewString()
	cache.Set(secureSessionIDCacheKey(sessionID), "unexpected", time.Minute)
	require.NotPanics(t, func() {
		assert.False(t, VerifySecureSessionID(sessionID, 42))
	})
	_, found := cache.Get(secureSessionIDCacheKey(sessionID))
	assert.False(t, found, "an invalid typed entry should be discarded")
}

func TestVerifySecureSessionIDDoesNotTouchSharedCryptoKey(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	cryptoParams := &internalcrypto.Params{PrivateKey: "private", PublicKey: "public"}
	cache.Set(internalcrypto.CacheKey, cryptoParams, time.Minute)

	assert.False(t, VerifySecureSessionID(internalcrypto.CacheKey, 42))
	actual, found := cache.Get(internalcrypto.CacheKey)
	require.True(t, found)
	assert.Same(t, cryptoParams, actual)
}
