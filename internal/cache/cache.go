package cache

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto/v2"
	"github.com/uozi-tech/cosy/logger"
)

// Global cache instance
var cache *ristretto.Cache[string, any]

// InitInMemoryCache initializes just the in-memory cache system (Ristretto).
// This is suitable for unit tests that don't require search functionality.
func InitInMemoryCache() {
	if cache != nil {
		cache.Close()
	}

	var err error
	cache, err = ristretto.NewCache(&ristretto.Config[string, any]{
		NumCounters: 1e7,     // Track frequency of 10M keys
		MaxCost:     1 << 30, // Maximum cache size: 1GB
		BufferItems: 64,      // Keys per Get buffer
	})
	if err != nil {
		logger.Fatal("Failed to initialize in-memory cache:", err)
	}
	logger.Info("In-memory cache initialized successfully")
}

// Init initializes the full cache system including search indexing and config scanning.
// This should be used by the main application and integration tests.
func Init(ctx context.Context) {
	// Force release any existing file system resources before initialization
	logger.Info("Initializing full cache system - ensuring clean state...")
	ForceReleaseResources()

	// Initialize the main in-memory cache
	InitInMemoryCache()

	// Initialize search index
	if err := InitSearchIndex(ctx); err != nil {
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
	if cache == nil {
		logger.Warn("Cache not initialized, skipping Set operation.")
		return
	}
	cache.SetWithTTL(key, value, 0, ttl)
	cache.Wait()
}

// Get retrieves a value from cache
func Get(key string) (interface{}, bool) {
	if cache == nil {
		logger.Warn("Cache not initialized, returning not found.")
		return nil, false
	}
	return cache.Get(key)
}

// Del removes a value from cache
func Del(key string) {
	if cache == nil {
		logger.Warn("Cache not initialized, skipping Del operation.")
		return
	}
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
