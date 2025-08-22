package searcher

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// OptimizedSearchCache provides high-performance caching using Ristretto
type OptimizedSearchCache struct {
	cache *ristretto.Cache[string, *SearchResult]
}

// NewOptimizedSearchCache creates a new optimized cache with Ristretto
func NewOptimizedSearchCache(maxSize int64) *OptimizedSearchCache {
	cache, err := ristretto.NewCache(&ristretto.Config[string, *SearchResult]{
		NumCounters: maxSize * 10, // Number of keys to track frequency of (10x cache size)
		MaxCost:     maxSize,      // Maximum cost of cache (number of items)
		BufferItems: 64,           // Number of keys per Get buffer
		Metrics:     true,         // Enable metrics collection
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create cache: %v", err))
	}

	return &OptimizedSearchCache{
		cache: cache,
	}
}

// GenerateOptimizedKey generates an efficient cache key for a search request
func (osc *OptimizedSearchCache) GenerateOptimizedKey(req *SearchRequest) string {
	// Create a unique key based on all search parameters
	keyData := struct {
		Query           string   `json:"query"`
		Limit           int      `json:"limit"`
		Offset          int      `json:"offset"`
		SortBy          string   `json:"sort_by"`
		SortOrder       string   `json:"sort_order"`
		StartTime       *int64   `json:"start_time"`
		EndTime         *int64   `json:"end_time"`
		IPAddresses     []string `json:"ip_addresses"`
		StatusCodes     []int    `json:"status_codes"`
		Methods         []string `json:"methods"`
		MinBytes        *int64   `json:"min_bytes"`
		MaxBytes        *int64   `json:"max_bytes"`
	}{
		Query:           req.Query,
		Limit:           req.Limit,
		Offset:          req.Offset,
		SortBy:          req.SortBy,
		SortOrder:       req.SortOrder,
		StartTime:       req.StartTime,
		EndTime:         req.EndTime,
		IPAddresses:     req.IPAddresses,
		StatusCodes:     req.StatusCodes,
		Methods:         req.Methods,
		MinBytes:        req.MinBytes,
		MaxBytes:        req.MaxBytes,
	}

	// Convert to JSON and hash for consistent key generation
	jsonData, err := json.Marshal(keyData)
	if err != nil {
		// Fallback to simple string concatenation if JSON marshal fails
		return fmt.Sprintf("q:%s|l:%d|o:%d|s:%s|so:%s", req.Query, req.Limit, req.Offset, req.SortBy, req.SortOrder)
	}

	// Use MD5 hash for compact key representation
	hash := md5.Sum(jsonData)
	return hex.EncodeToString(hash[:])
}

// Get retrieves a search result from cache
func (osc *OptimizedSearchCache) Get(req *SearchRequest) *SearchResult {
	key := osc.GenerateOptimizedKey(req)
	
	if result, found := osc.cache.Get(key); found {
		// Mark result as from cache
		cachedResult := *result // Create a copy
		cachedResult.FromCache = true
		return &cachedResult
	}
	
	return nil
}

// Put stores a search result in cache with automatic cost calculation
func (osc *OptimizedSearchCache) Put(req *SearchRequest, result *SearchResult, ttl time.Duration) {
	key := osc.GenerateOptimizedKey(req)
	
	// Calculate cost based on result size (number of hits + base cost)
	cost := int64(1 + len(result.Hits)/10) // Base cost of 1 plus hits/10
	if cost < 1 {
		cost = 1
	}
	
	// Set with TTL
	osc.cache.SetWithTTL(key, result, cost, ttl)
	// Wait for the value to pass through buffers to ensure it's cached
	osc.cache.Wait()
}

// Clear clears all cached entries
func (osc *OptimizedSearchCache) Clear() {
	osc.cache.Clear()
}

// GetStats returns cache statistics
func (osc *OptimizedSearchCache) GetStats() *CacheStats {
	metrics := osc.cache.Metrics
	
	return &CacheStats{
		Size:        int(metrics.KeysAdded() - metrics.KeysEvicted()),
		Capacity:    int(osc.cache.MaxCost()),
		HitCount:    int64(metrics.Hits()),
		MissCount:   int64(metrics.Misses()),
		HitRate:     metrics.Ratio(),
		Evictions:   int64(metrics.KeysEvicted()),
		Additions:   int64(metrics.KeysAdded()),
		Updates:     int64(metrics.KeysUpdated()),
		Cost:        int64(metrics.CostAdded() - metrics.CostEvicted()),
	}
}

// CacheStats provides detailed cache statistics
type CacheStats struct {
	Size      int     `json:"size"`        // Current number of items
	Capacity  int     `json:"capacity"`    // Maximum capacity
	HitCount  int64   `json:"hit_count"`   // Number of cache hits
	MissCount int64   `json:"miss_count"`  // Number of cache misses
	HitRate   float64 `json:"hit_rate"`    // Cache hit rate (0.0 to 1.0)
	Evictions int64   `json:"evictions"`   // Number of evicted items
	Additions int64   `json:"additions"`   // Number of items added
	Updates   int64   `json:"updates"`     // Number of items updated
	Cost      int64   `json:"cost"`        // Current cost
}

// WarmupCache pre-loads frequently used queries into cache
func (osc *OptimizedSearchCache) WarmupCache(queries []WarmupQuery) {
	for _, query := range queries {
		// Pre-generate keys to warm up the cache
		key := osc.GenerateOptimizedKey(query.Request)
		if query.Result != nil {
			osc.cache.Set(key, query.Result, 1) // Use cost of 1 for warmup
		}
	}
	
	// Wait for cache operations to complete
	osc.cache.Wait()
}

// WarmupQuery represents a query and result pair for cache warmup
type WarmupQuery struct {
	Request *SearchRequest `json:"request"`
	Result  *SearchResult  `json:"result"`
}

// Close closes the cache and frees resources
func (osc *OptimizedSearchCache) Close() {
	osc.cache.Close()
}

// FastKeyGenerator provides even faster key generation for hot paths
type FastKeyGenerator struct {
	buffer []byte
}

// NewFastKeyGenerator creates a key generator with pre-allocated buffer
func NewFastKeyGenerator() *FastKeyGenerator {
	return &FastKeyGenerator{
		buffer: make([]byte, 0, 256), // Pre-allocate 256 bytes
	}
}

// GenerateKey generates a key using pre-allocated buffer
func (fkg *FastKeyGenerator) GenerateKey(req *SearchRequest) string {
	fkg.buffer = fkg.buffer[:0] // Reset buffer
	
	// Build key using buffer to avoid allocations
	fkg.buffer = append(fkg.buffer, "q:"...)
	fkg.buffer = append(fkg.buffer, req.Query...)
	fkg.buffer = append(fkg.buffer, "|l:"...)
	fkg.buffer = strconv.AppendInt(fkg.buffer, int64(req.Limit), 10)
	fkg.buffer = append(fkg.buffer, "|o:"...)
	fkg.buffer = strconv.AppendInt(fkg.buffer, int64(req.Offset), 10)
	fkg.buffer = append(fkg.buffer, "|s:"...)
	fkg.buffer = append(fkg.buffer, req.SortBy...)
	fkg.buffer = append(fkg.buffer, "|so:"...)
	fkg.buffer = append(fkg.buffer, req.SortOrder...)
	
	// Add timestamps if present
	if req.StartTime != nil {
		fkg.buffer = append(fkg.buffer, "|st:"...)
		fkg.buffer = strconv.AppendInt(fkg.buffer, *req.StartTime, 10)
	}
	if req.EndTime != nil {
		fkg.buffer = append(fkg.buffer, "|et:"...)
		fkg.buffer = strconv.AppendInt(fkg.buffer, *req.EndTime, 10)
	}
	
	// Add arrays (simplified)
	if len(req.StatusCodes) > 0 {
		fkg.buffer = append(fkg.buffer, "|sc:"...)
		for i, code := range req.StatusCodes {
			if i > 0 {
				fkg.buffer = append(fkg.buffer, ',')
			}
			fkg.buffer = strconv.AppendInt(fkg.buffer, int64(code), 10)
		}
	}
	
	// Convert to string (this still allocates, but fewer allocations overall)
	return string(fkg.buffer)
}

// CacheMiddleware provides middleware functionality for caching
type CacheMiddleware struct {
	cache     *OptimizedSearchCache
	keyGen    *FastKeyGenerator
	enabled   bool
	defaultTTL time.Duration
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(cache *OptimizedSearchCache, defaultTTL time.Duration) *CacheMiddleware {
	return &CacheMiddleware{
		cache:      cache,
		keyGen:     NewFastKeyGenerator(),
		enabled:    true,
		defaultTTL: defaultTTL,
	}
}

// Enable enables caching
func (cm *CacheMiddleware) Enable() {
	cm.enabled = true
}

// Disable disables caching
func (cm *CacheMiddleware) Disable() {
	cm.enabled = false
}

// IsEnabled returns whether caching is enabled
func (cm *CacheMiddleware) IsEnabled() bool {
	return cm.enabled
}

// GetOrSet attempts to get from cache, or executes the provided function and caches the result
func (cm *CacheMiddleware) GetOrSet(req *SearchRequest, fn func() (*SearchResult, error)) (*SearchResult, error) {
	if !cm.enabled {
		return fn()
	}
	
	// Try cache first
	if cached := cm.cache.Get(req); cached != nil {
		return cached, nil
	}
	
	// Execute function
	result, err := fn()
	if err != nil {
		return nil, err
	}
	
	// Cache successful result
	if result != nil {
		cm.cache.Put(req, result, cm.defaultTTL)
	}
	
	return result, nil
}