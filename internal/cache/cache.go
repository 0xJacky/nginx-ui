package cache

import (
	"github.com/0xJacky/Nginx-UI/internal/logger"
	"github.com/dgraph-io/ristretto"
	"time"
)

var cache *ristretto.Cache

func Init() {
	var err error
	cache, err = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})

	if err != nil {
		logger.Fatal("initializing local cache err", err)
	}
}

func Set(key interface{}, value interface{}, ttl time.Duration) {
	cache.SetWithTTL(key, value, 0, ttl)
	cache.Wait()
}

func Get(key interface{}) (value interface{}, ok bool) {
	return cache.Get(key)
}

func Del(key interface{}) {
	cache.Del(key)
}
