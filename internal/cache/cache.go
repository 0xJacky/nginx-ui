package cache

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Global cache instance
var cache *ristretto.Cache[string, any]

// Init initializes the cache system with search indexing and config scanning
func Init(ctx context.Context) {
	// Force release any existing file system resources before initialization
	logger.Info("Initializing cache system - ensuring clean state...")
	ForceReleaseResources()

	// Initialize the main cache
	var err error
	cache, err = ristretto.NewCache(&ristretto.Config[string, any]{
		NumCounters: 1e7,     // Track frequency of 10M keys
		MaxCost:     1 << 30, // Maximum cache size: 1GB
		BufferItems: 64,      // Keys per Get buffer
	})
	if err != nil {
		logger.Fatal("Failed to initialize cache:", err)
	}

	// Initialize search index
	if err = InitSearchIndex(ctx); err != nil {
		logger.Error("Failed to initialize search index:", err)
	}

	// Initialize config file scanner
	logger.Info("Starting config scanner initialization...")
	InitScanner(ctx)
	logger.Info("Cache system initialization completed")

	go func() {
		<-ctx.Done()
		Shutdown()
	}()
}

// Set stores a value in cache with TTL
func Set(key string, value interface{}, ttl time.Duration) {
	cache.SetWithTTL(key, value, 0, ttl)
	cache.Wait()
}

// Get retrieves a value from cache
func Get(key string) (interface{}, bool) {
	return cache.Get(key)
}

// Del removes a value from cache
func Del(key string) {
	cache.Del(key)
}

// Shutdown gracefully shuts down the cache system and releases all resources
func Shutdown() {
	logger.Info("Shutting down cache system...")

	// Force release all file system resources
	ForceReleaseResources()

	// Close main cache
	if cache != nil {
		cache.Close()
		cache = nil
		logger.Info("Main cache closed")
	}

	logger.Info("Cache system shutdown completed")
}
