package searcher

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/dgraph-io/ristretto/v2"
)

// Cache provides high-performance caching using Ristretto
type Cache struct {
	cache *ristretto.Cache[string, *SearchResult]
}

// NewCache creates a new cache with Ristretto
func NewCache(maxSize int64) *Cache {
	cache, err := ristretto.NewCache(&ristretto.Config[string, *SearchResult]{
		NumCounters: maxSize * 10,
		MaxCost:     maxSize,
		BufferItems: 64,
		Metrics:     true,
	})
	if err != nil {
		panic(fmt.Sprintf("failed to create cache: %v", err))
	}

	return &Cache{cache: cache}
}

// CacheKeyData represents the normalized data used for cache key generation
type CacheKeyData struct {
	Query          string   `json:"query"`
	Limit          int      `json:"limit"`
	Offset         int      `json:"offset"`
	SortBy         string   `json:"sort_by"`
	SortOrder      string   `json:"sort_order"`
	StartTime      *int64   `json:"start_time"`
	EndTime        *int64   `json:"end_time"`
	UseMainLogPath bool     `json:"use_main_log_path"`
	LogPaths       []string `json:"log_paths"`
	Fields         []string `json:"fields"`
	IPAddresses    []string `json:"ip_addresses"`
	StatusCodes    []int    `json:"status_codes"`
	Methods        []string `json:"methods"`
	Paths          []string `json:"paths"`
	UserAgents     []string `json:"user_agents"`
	Referers       []string `json:"referers"`
	Countries      []string `json:"countries"`
	Browsers       []string `json:"browsers"`
	OSs            []string `json:"operating_systems"`
	Devices        []string `json:"devices"`
	MinBytes       *int64   `json:"min_bytes"`
	MaxBytes       *int64   `json:"max_bytes"`
	MinReqTime     *float64 `json:"min_request_time"`
	MaxReqTime     *float64 `json:"max_request_time"`
	IncludeFacets  bool     `json:"include_facets"`
	IncludeStats   bool     `json:"include_stats"`
	FacetFields    []string `json:"facet_fields"`
	FacetSize      int      `json:"facet_size"`
	UseCache       bool     `json:"use_cache"`
}

// sortedUniqueStrings returns a sorted, deduplicated copy of a string slice
func sortedUniqueStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(src))
	res := make([]string, 0, len(src))
	for _, s := range src {
		if _, exists := seen[s]; !exists {
			seen[s] = struct{}{}
			res = append(res, s)
		}
	}
	sort.Strings(res)
	return res
}

// sortedStrings returns a sorted copy of a string slice
func sortedStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	res := make([]string, len(src))
	copy(res, src)
	sort.Strings(res)
	return res
}

// sortedInts returns a sorted copy of an int slice
func sortedInts(src []int) []int {
	if len(src) == 0 {
		return nil
	}
	res := make([]int, len(src))
	copy(res, src)
	sort.Ints(res)
	return res
}

// GenerateKey generates an efficient cache key for a search request
func (c *Cache) GenerateKey(req *SearchRequest) string {
	keyData := CacheKeyData{
		Query:          req.Query,
		Limit:          req.Limit,
		Offset:         req.Offset,
		SortBy:         req.SortBy,
		SortOrder:      req.SortOrder,
		StartTime:      req.StartTime,
		EndTime:        req.EndTime,
		UseMainLogPath: req.UseMainLogPath,
		LogPaths:       sortedUniqueStrings(req.LogPaths),
		Fields:         sortedStrings(req.Fields),
		IPAddresses:    sortedUniqueStrings(req.IPAddresses),
		StatusCodes:    sortedInts(req.StatusCodes),
		Methods:        sortedUniqueStrings(req.Methods),
		Paths:          sortedUniqueStrings(req.Paths),
		UserAgents:     sortedUniqueStrings(req.UserAgents),
		Referers:       sortedUniqueStrings(req.Referers),
		Countries:      sortedUniqueStrings(req.Countries),
		Browsers:       sortedUniqueStrings(req.Browsers),
		OSs:            sortedUniqueStrings(req.OSs),
		Devices:        sortedUniqueStrings(req.Devices),
		MinBytes:       req.MinBytes,
		MaxBytes:       req.MaxBytes,
		MinReqTime:     req.MinReqTime,
		MaxReqTime:     req.MaxReqTime,
		IncludeFacets:  req.IncludeFacets,
		IncludeStats:   req.IncludeStats,
		FacetFields:    sortedUniqueStrings(req.FacetFields),
		FacetSize:      req.FacetSize,
		UseCache:       req.UseCache,
	}

	jsonData, err := json.Marshal(keyData)
	if err != nil {
		return c.generateFallbackKey(req)
	}

	hash := md5.Sum(jsonData)
	return hex.EncodeToString(hash[:])
}

// generateFallbackKey creates a basic cache key when JSON marshaling fails
func (c *Cache) generateFallbackKey(req *SearchRequest) string {
	keyBuf := make([]byte, 0, len(req.Query)+len(req.SortBy)+len(req.SortOrder)+32)
	keyBuf = append(keyBuf, "q:"...)
	keyBuf = append(keyBuf, req.Query...)
	keyBuf = append(keyBuf, "|l:"...)
	keyBuf = utils.AppendInt(keyBuf, req.Limit)
	keyBuf = append(keyBuf, "|o:"...)
	keyBuf = utils.AppendInt(keyBuf, req.Offset)
	keyBuf = append(keyBuf, "|s:"...)
	keyBuf = append(keyBuf, req.SortBy...)
	keyBuf = append(keyBuf, "|so:"...)
	keyBuf = append(keyBuf, req.SortOrder...)
	return utils.BytesToStringUnsafe(keyBuf)
}

// Get retrieves a search result from cache
func (c *Cache) Get(req *SearchRequest) *SearchResult {
	key := c.GenerateKey(req)

	result, found := c.cache.Get(key)
	if !found {
		return nil
	}

	cachedResult := *result
	cachedResult.FromCache = true
	return &cachedResult
}

// Put stores a search result in cache with automatic cost calculation
func (c *Cache) Put(req *SearchRequest, result *SearchResult, ttl time.Duration) {
	key := c.GenerateKey(req)

	cost := int64(1 + len(result.Hits)/10)
	if cost < 1 {
		cost = 1
	}

	c.cache.SetWithTTL(key, result, cost, ttl)
	c.cache.Wait()
}

// Clear clears all cached entries
func (c *Cache) Clear() {
	if c != nil && c.cache != nil {
		c.cache.Clear()
	}
}

// GetStats returns cache statistics
func (c *Cache) GetStats() *CacheStats {
	metrics := c.cache.Metrics

	return &CacheStats{
		Size:      int(metrics.KeysAdded() - metrics.KeysEvicted()),
		Capacity:  int(c.cache.MaxCost()),
		HitCount:  int64(metrics.Hits()),
		MissCount: int64(metrics.Misses()),
		HitRate:   metrics.Ratio(),
		Evictions: int64(metrics.KeysEvicted()),
		Additions: int64(metrics.KeysAdded()),
		Updates:   int64(metrics.KeysUpdated()),
		Cost:      int64(metrics.CostAdded() - metrics.CostEvicted()),
	}
}

// CacheStats provides detailed cache statistics
type CacheStats struct {
	Size      int     `json:"size"`
	Capacity  int     `json:"capacity"`
	HitCount  int64   `json:"hit_count"`
	MissCount int64   `json:"miss_count"`
	HitRate   float64 `json:"hit_rate"`
	Evictions int64   `json:"evictions"`
	Additions int64   `json:"additions"`
	Updates   int64   `json:"updates"`
	Cost      int64   `json:"cost"`
}

// Warmup pre-loads frequently used queries into cache
func (c *Cache) Warmup(queries []WarmupQuery) {
	for _, query := range queries {
		if query.Result != nil {
			key := c.GenerateKey(query.Request)
			c.cache.Set(key, query.Result, 1)
		}
	}

	c.cache.Wait()
}

// WarmupQuery represents a query and result pair for cache warmup
type WarmupQuery struct {
	Request *SearchRequest `json:"request"`
	Result  *SearchResult  `json:"result"`
}

// Close closes the cache and frees resources
func (c *Cache) Close() {
	c.cache.Close()
}

// KeyGen provides even faster key generation for hot paths
type KeyGen struct {
	buffer []byte
}

// NewKeyGen creates a key generator with pre-allocated buffer
func NewKeyGen() *KeyGen {
	return &KeyGen{
		buffer: make([]byte, 0, 256),
	}
}

// GenerateKey generates a key using pre-allocated buffer
func (kg *KeyGen) GenerateKey(req *SearchRequest) string {
	kg.buffer = kg.buffer[:0]

	kg.buffer = append(kg.buffer, "q:"...)
	kg.buffer = append(kg.buffer, req.Query...)
	kg.buffer = append(kg.buffer, "|l:"...)
	kg.buffer = strconv.AppendInt(kg.buffer, int64(req.Limit), 10)
	kg.buffer = append(kg.buffer, "|o:"...)
	kg.buffer = strconv.AppendInt(kg.buffer, int64(req.Offset), 10)
	kg.buffer = append(kg.buffer, "|s:"...)
	kg.buffer = append(kg.buffer, req.SortBy...)
	kg.buffer = append(kg.buffer, "|so:"...)
	kg.buffer = append(kg.buffer, req.SortOrder...)

	if req.StartTime != nil {
		kg.buffer = append(kg.buffer, "|st:"...)
		kg.buffer = strconv.AppendInt(kg.buffer, *req.StartTime, 10)
	}
	if req.EndTime != nil {
		kg.buffer = append(kg.buffer, "|et:"...)
		kg.buffer = strconv.AppendInt(kg.buffer, *req.EndTime, 10)
	}

	if len(req.StatusCodes) > 0 {
		kg.buffer = append(kg.buffer, "|sc:"...)
		for i, code := range req.StatusCodes {
			if i > 0 {
				kg.buffer = append(kg.buffer, ',')
			}
			kg.buffer = strconv.AppendInt(kg.buffer, int64(code), 10)
		}
	}

	return string(kg.buffer)
}

// Middleware provides middleware functionality for caching
type Middleware struct {
	cache      *Cache
	keyGen     *KeyGen
	enabled    bool
	defaultTTL time.Duration
}

// NewMiddleware creates a new cache middleware
func NewMiddleware(cache *Cache, defaultTTL time.Duration) *Middleware {
	return &Middleware{
		cache:      cache,
		keyGen:     NewKeyGen(),
		enabled:    true,
		defaultTTL: defaultTTL,
	}
}

// Enable enables caching
func (m *Middleware) Enable() {
	m.enabled = true
}

// Disable disables caching
func (m *Middleware) Disable() {
	m.enabled = false
}

// IsEnabled returns whether caching is enabled
func (m *Middleware) IsEnabled() bool {
	return m.enabled
}

// GetOrSet attempts to get from cache, or executes the provided function and caches the result
func (m *Middleware) GetOrSet(req *SearchRequest, fn func() (*SearchResult, error)) (*SearchResult, error) {
	if !m.enabled {
		return fn()
	}

	if cached := m.cache.Get(req); cached != nil {
		return cached, nil
	}

	result, err := fn()
	if err != nil {
		return nil, err
	}

	if result != nil {
		m.cache.Put(req, result, m.defaultTTL)
	}

	return result, nil
}
