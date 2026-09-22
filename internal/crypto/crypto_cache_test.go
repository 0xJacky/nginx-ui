package crypto

import (
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCryptoParamsRecoversFromWrongCacheType(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	cache.Set(CacheKey, "unexpected", time.Minute)

	var params *Params
	var err error
	require.NotPanics(t, func() {
		params, err = GetCryptoParams()
	})
	require.NoError(t, err)
	require.NotNil(t, params)
	assert.NotEmpty(t, params.PrivateKey)
	assert.NotEmpty(t, params.PublicKey)
}

func TestDecryptRejectsWrongCacheTypeWithoutPanic(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	cache.Set(CacheKey, "unexpected", time.Minute)

	require.NotPanics(t, func() {
		_, err := Decrypt("unused")
		assert.ErrorIs(t, err, ErrTimeout)
	})
	_, found := cache.Get(CacheKey)
	assert.False(t, found, "an invalid typed entry should be discarded")
}
