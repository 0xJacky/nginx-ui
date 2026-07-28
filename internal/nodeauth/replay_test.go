package nodeauth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReplayCacheRejectsOverflowWithoutEvictingLiveNonces(t *testing.T) {
	now := time.Unix(2_000_000_000, 0)
	cache := NewReplayCache(2)
	assert.True(t, cache.Use("first", now))
	assert.True(t, cache.Use("second", now))
	assert.False(t, cache.Use("overflow", now))
	assert.False(t, cache.Use("first", now), "a live nonce must not become replayable after overflow")
	assert.True(t, cache.Use("after-expiry", now.Add(replayWindow)))
}
