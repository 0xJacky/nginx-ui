package nodeauth

import (
	"sync"
	"time"
)

const (
	replayWindow   = 2 * time.Minute
	maxReplayItems = 10000
)

type replayEntry struct {
	key       string
	expiresAt time.Time
}

type ReplayCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	order   []replayEntry
	limit   int
}

func NewReplayCache(limit int) *ReplayCache {
	if limit <= 0 {
		limit = maxReplayItems
	}
	return &ReplayCache{entries: make(map[string]time.Time), limit: limit}
}

func (cache *ReplayCache) Use(key string, now time.Time) bool {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.evict(now)
	if expiresAt, exists := cache.entries[key]; exists && expiresAt.After(now) {
		return false
	}
	if len(cache.entries) >= cache.limit {
		return false
	}

	expiresAt := now.Add(replayWindow)
	cache.entries[key] = expiresAt
	cache.order = append(cache.order, replayEntry{key: key, expiresAt: expiresAt})
	return true
}

func (cache *ReplayCache) evict(now time.Time) {
	for len(cache.order) > 0 {
		entry := cache.order[0]
		if entry.expiresAt.After(now) {
			break
		}
		cache.order = cache.order[1:]
		if current, exists := cache.entries[entry.key]; exists && current.Equal(entry.expiresAt) {
			delete(cache.entries, entry.key)
		}
	}
}
