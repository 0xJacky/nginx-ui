package user

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakePasskeyPreAuthSessionIsOneTime(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	session := &passkeyPreAuthSession{UserID: 7, SessionData: &webauthn.SessionData{}}
	cache.Set(buildPasskeyPreAuthKey("one-time"), session, time.Minute)

	actual, ok := takePasskeyPreAuthSession("one-time")
	require.True(t, ok)
	assert.Equal(t, uint64(7), actual.UserID)
	_, ok = takePasskeyPreAuthSession("one-time")
	assert.False(t, ok)
}

func TestTakePasskeyPreAuthSessionExpires(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	cache.Set(buildPasskeyPreAuthKey("expired"), &passkeyPreAuthSession{
		UserID:      9,
		SessionData: &webauthn.SessionData{},
	}, time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := takePasskeyPreAuthSession("expired")
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestTakePasskeyPreAuthSessionAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	cache.Set(buildPasskeyPreAuthKey("concurrent"), &passkeyPreAuthSession{
		UserID:      11,
		SessionData: &webauthn.SessionData{},
	}, time.Minute)

	var successful atomic.Int32
	var waitGroup sync.WaitGroup
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if _, ok := takePasskeyPreAuthSession("concurrent"); ok {
				successful.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	assert.Equal(t, int32(1), successful.Load())
}
