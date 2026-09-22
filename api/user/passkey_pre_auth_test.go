package user

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/cache"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakePasskeyPreAuthSessionIsOneTime(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	sessionID := uuid.NewString()
	session := &passkeyPreAuthSession{UserID: 7, SessionData: &webauthn.SessionData{}}
	cache.Set(buildPasskeyPreAuthKey(sessionID), session, time.Minute)

	actual, ok := takePasskeyPreAuthSession(sessionID)
	require.True(t, ok)
	assert.Equal(t, uint64(7), actual.UserID)
	_, ok = takePasskeyPreAuthSession(sessionID)
	assert.False(t, ok)
}

func TestTakePasskeyPreAuthSessionExpires(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	sessionID := uuid.NewString()
	cache.Set(buildPasskeyPreAuthKey(sessionID), &passkeyPreAuthSession{
		UserID:      9,
		SessionData: &webauthn.SessionData{},
	}, time.Millisecond)

	require.Eventually(t, func() bool {
		_, ok := takePasskeyPreAuthSession(sessionID)
		return !ok
	}, time.Second, 10*time.Millisecond)
}

func TestTakePasskeyPreAuthSessionAllowsOnlyOneConcurrentConsumer(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)
	sessionID := uuid.NewString()
	cache.Set(buildPasskeyPreAuthKey(sessionID), &passkeyPreAuthSession{
		UserID:      11,
		SessionData: &webauthn.SessionData{},
	}, time.Minute)

	var successful atomic.Int32
	var waitGroup sync.WaitGroup
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if _, ok := takePasskeyPreAuthSession(sessionID); ok {
				successful.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	assert.Equal(t, int32(1), successful.Load())
}

func TestTakePasskeyPreAuthSessionRejectsMalformedAndWrongType(t *testing.T) {
	cache.InitInMemoryCache()
	t.Cleanup(cache.Shutdown)

	cache.Set(buildPasskeyPreAuthKey("crypto"), &passkeyPreAuthSession{
		UserID:      13,
		SessionData: &webauthn.SessionData{},
	}, time.Minute)
	_, ok := takePasskeyPreAuthSession("crypto")
	assert.False(t, ok)
	_, found := cache.Get(buildPasskeyPreAuthKey("crypto"))
	assert.True(t, found, "a malformed ID must be rejected before touching the cache")

	sessionID := uuid.NewString()
	cache.Set(buildPasskeyPreAuthKey(sessionID), "unexpected", time.Minute)
	assert.NotPanics(t, func() {
		_, ok = takePasskeyPreAuthSession(sessionID)
	})
	assert.False(t, ok)
}
