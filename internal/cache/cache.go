package cache

import (
	"github.com/uozi-tech/cosy/logger"
	"github.com/dgraph-io/ristretto"
	"time"
)

var cache *ristretto.Cache[string, any]

func Init() {
	var err error
	cache, err = ristretto.NewCache[string, any](&ristretto.Config[string, any]{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 30, // maximum cost of cache (1GB).
		BufferItems: 64,      // number of keys per Get buffer.
	})

	if err != nil {
		logger.Fatal("initializing local cache err", err)
	}
}

func Set(key string, value interface{}, ttl time.Duration) {
	cache.SetWithTTL(key, value, 0, ttl)
	cache.Wait()
}

func Get(key string) (value interface{}, ok bool) {
	return cache.Get(key)
}

func Del(key string) {
	cache.Del(key)
}
