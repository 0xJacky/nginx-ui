package parser

import (
	"regexp"
	"sync"
	"time"
)

// RegexCache provides high-performance compiled regex caching
type RegexCache struct {
	cache         map[string]*CachedRegex
	mutex         sync.RWMutex
	maxSize       int
	ttl           time.Duration
	hits          int64
	misses        int64
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// CachedRegex represents a compiled regex with metadata
type CachedRegex struct {
	regex      *regexp.Regexp
	pattern    string
	compiledAt time.Time
	lastUsed   time.Time
	useCount   int64
}

// RegexCacheStats provides cache statistics
type RegexCacheStats struct {
	Size    int     `json:"size"`
	MaxSize int     `json:"max_size"`
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitRate float64 `json:"hit_rate"`
	TTL     string  `json:"ttl"`
}

// Common nginx log parsing patterns - pre-compiled for performance
var commonPatterns = map[string]string{
	"ipv4":            `(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`,
	"ipv6":            `([0-9a-fA-F:]+:+[0-9a-fA-F:]*[0-9a-fA-F]+)`,
	"timestamp":       `\[([^\]]+)\]`,
	"method":          `"([A-Z]+)\s+`,
	"path":            `\s+([^\s?"]+)`,
	"protocol":        `\s+(HTTP/[0-9.]+)"`,
	"status":          `"\s+(\d{3})\s+`,
	"size":            `\s+(\d+|-)`,
	"referer":         `"([^"]*)"`,
	"user_agent":      `"([^"]*)"`,
	"request_time":    `\s+([\d.]+)$`,
	"upstream_time":   `\s+([\d.]+)\s*$`,
	"combined_format": `^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"([^"]+)"\s+(\d+)\s+(\d+|-)(?:\s+"([^"]*)")?(?:\s+"([^"]*)")?(?:\s+([\d.]+))?(?:\s+([\d.]+))?`,
	"main_format":     `^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"([^"]+)"\s+(\d+)\s+(\d+|-)`,
}

// Global regex cache instance
var globalRegexCache *RegexCache
var regexCacheOnce sync.Once

// GetGlobalRegexCache returns the global regex cache instance
func GetGlobalRegexCache() *RegexCache {
	regexCacheOnce.Do(func() {
		globalRegexCache = NewRegexCache(1000, 24*time.Hour) // 1000 patterns, 24h TTL
		globalRegexCache.PrecompileCommonPatterns()
	})
	return globalRegexCache
}

// NewRegexCache creates a new regex cache with the specified parameters
func NewRegexCache(maxSize int, ttl time.Duration) *RegexCache {
	cache := &RegexCache{
		cache:       make(map[string]*CachedRegex),
		maxSize:     maxSize,
		ttl:         ttl,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup routine
	cache.cleanupTicker = time.NewTicker(ttl / 4) // Clean every quarter of TTL
	go cache.cleanupRoutine()

	return cache
}

// PrecompileCommonPatterns pre-compiles common nginx log parsing patterns
func (rc *RegexCache) PrecompileCommonPatterns() {
	for name, pattern := range commonPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			continue // Skip invalid patterns
		}

		rc.mutex.Lock()
		rc.cache[name] = &CachedRegex{
			regex:      regex,
			pattern:    pattern,
			compiledAt: time.Now(),
			lastUsed:   time.Now(),
			useCount:   0,
		}
		rc.mutex.Unlock()
	}
}

// GetRegex retrieves or compiles a regex pattern
func (rc *RegexCache) GetRegex(pattern string) (*regexp.Regexp, error) {
	// Try to get from cache first
	rc.mutex.RLock()
	cached, exists := rc.cache[pattern]
	if exists {
		// Check if not expired
		if time.Since(cached.compiledAt) < rc.ttl {
			cached.lastUsed = time.Now()
			cached.useCount++
			rc.hits++
			rc.mutex.RUnlock()
			return cached.regex, nil
		}
	}
	rc.mutex.RUnlock()

	// Cache miss or expired - compile new regex
	regex, err := regexp.Compile(pattern)
	if err != nil {
		rc.mutex.Lock()
		rc.misses++
		rc.mutex.Unlock()
		return nil, err
	}

	// Store in cache
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	// Check if cache is full
	if len(rc.cache) >= rc.maxSize {
		rc.evictLeastUsed()
	}

	rc.cache[pattern] = &CachedRegex{
		regex:      regex,
		pattern:    pattern,
		compiledAt: time.Now(),
		lastUsed:   time.Now(),
		useCount:   1,
	}
	rc.misses++

	return regex, nil
}

// GetCommonRegex retrieves a pre-compiled common pattern
func (rc *RegexCache) GetCommonRegex(patternName string) (*regexp.Regexp, bool) {
	pattern, exists := commonPatterns[patternName]
	if !exists {
		return nil, false
	}

	regex, err := rc.GetRegex(pattern)
	if err != nil {
		return nil, false
	}

	return regex, true
}

// evictLeastUsed removes the least recently used entry from cache
func (rc *RegexCache) evictLeastUsed() {
	var oldestKey string
	var oldestTime time.Time
	var lowestCount int64 = -1

	for key, cached := range rc.cache {
		if lowestCount == -1 || cached.useCount < lowestCount {
			lowestCount = cached.useCount
			oldestKey = key
			oldestTime = cached.lastUsed
		} else if cached.useCount == lowestCount && cached.lastUsed.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.lastUsed
		}
	}

	if oldestKey != "" {
		delete(rc.cache, oldestKey)
	}
}

// cleanupRoutine periodically removes expired entries
func (rc *RegexCache) cleanupRoutine() {
	for {
		select {
		case <-rc.cleanupTicker.C:
			rc.cleanup()
		case <-rc.stopCleanup:
			rc.cleanupTicker.Stop()
			return
		}
	}
}

// cleanup removes expired entries from the cache
func (rc *RegexCache) cleanup() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	now := time.Now()
	for key, cached := range rc.cache {
		if now.Sub(cached.compiledAt) > rc.ttl {
			delete(rc.cache, key)
		}
	}
}

// GetStats returns cache statistics
func (rc *RegexCache) GetStats() RegexCacheStats {
	rc.mutex.RLock()
	defer rc.mutex.RUnlock()

	total := rc.hits + rc.misses
	var hitRate float64
	if total > 0 {
		hitRate = float64(rc.hits) / float64(total)
	}

	return RegexCacheStats{
		Size:    len(rc.cache),
		MaxSize: rc.maxSize,
		Hits:    rc.hits,
		Misses:  rc.misses,
		HitRate: hitRate,
		TTL:     rc.ttl.String(),
	}
}

// Clear clears all cached regexes
func (rc *RegexCache) Clear() {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.cache = make(map[string]*CachedRegex)
	rc.hits = 0
	rc.misses = 0
}

// Close stops the cleanup routine and clears the cache
func (rc *RegexCache) Close() {
	close(rc.stopCleanup)
	rc.Clear()
}

// RegexMatcher provides optimized regex matching for log parsing
type RegexMatcher struct {
	cache *RegexCache
	// Pre-compiled common patterns for fastest access
	ipv4Regex      *regexp.Regexp
	timestampRegex *regexp.Regexp
	methodRegex    *regexp.Regexp
	statusRegex    *regexp.Regexp
	combinedRegex  *regexp.Regexp
	mainRegex      *regexp.Regexp
}

// NewRegexMatcher creates a new optimized regex matcher
func NewRegexMatcher() *RegexMatcher {
	cache := GetGlobalRegexCache()

	matcher := &RegexMatcher{
		cache: cache,
	}

	// Pre-compile most common patterns for direct access
	matcher.ipv4Regex, _ = cache.GetCommonRegex("ipv4")
	matcher.timestampRegex, _ = cache.GetCommonRegex("timestamp")
	matcher.methodRegex, _ = cache.GetCommonRegex("method")
	matcher.statusRegex, _ = cache.GetCommonRegex("status")
	matcher.combinedRegex, _ = cache.GetCommonRegex("combined_format")
	matcher.mainRegex, _ = cache.GetCommonRegex("main_format")

	return matcher
}

// MatchIPv4 matches IPv4 addresses using cached regex
func (orm *RegexMatcher) MatchIPv4(text string) []string {
	if orm.ipv4Regex != nil {
		return orm.ipv4Regex.FindStringSubmatch(text)
	}
	return nil
}

// MatchTimestamp matches timestamp patterns using cached regex
func (orm *RegexMatcher) MatchTimestamp(text string) []string {
	if orm.timestampRegex != nil {
		return orm.timestampRegex.FindStringSubmatch(text)
	}
	return nil
}

// MatchCombinedFormat matches complete combined log format
func (orm *RegexMatcher) MatchCombinedFormat(text string) []string {
	if orm.combinedRegex != nil {
		return orm.combinedRegex.FindStringSubmatch(text)
	}
	return nil
}

// MatchMainFormat matches main log format
func (orm *RegexMatcher) MatchMainFormat(text string) []string {
	if orm.mainRegex != nil {
		return orm.mainRegex.FindStringSubmatch(text)
	}
	return nil
}

// MatchPattern matches any pattern using the regex cache
func (orm *RegexMatcher) MatchPattern(pattern, text string) ([]string, error) {
	regex, err := orm.cache.GetRegex(pattern)
	if err != nil {
		return nil, err
	}

	return regex.FindStringSubmatch(text), nil
}

// DetectLogFormat detects nginx log format using cached patterns
func (orm *RegexMatcher) DetectLogFormat(logLine string) string {
	// Try combined format first (most common)
	if orm.combinedRegex != nil && orm.combinedRegex.MatchString(logLine) {
		return "combined"
	}

	// Try main format
	if orm.mainRegex != nil && orm.mainRegex.MatchString(logLine) {
		return "main"
	}

	return "unknown"
}

// GetCacheStats returns regex cache statistics
func (orm *RegexMatcher) GetCacheStats() RegexCacheStats {
	return orm.cache.GetStats()
}

// FastLogFormatDetector provides ultra-fast log format detection
type FastLogFormatDetector struct {
	combinedRegex *regexp.Regexp
	mainRegex     *regexp.Regexp
	// Pre-computed patterns for fastest detection
	combinedPatternBytes []byte
	mainPatternBytes     []byte
}

// NewFastLogFormatDetector creates a new fast log format detector
func NewFastLogFormatDetector() *FastLogFormatDetector {
	cache := GetGlobalRegexCache()

	detector := &FastLogFormatDetector{}
	detector.combinedRegex, _ = cache.GetCommonRegex("combined_format")
	detector.mainRegex, _ = cache.GetCommonRegex("main_format")

	// Pre-compute pattern signatures for ultra-fast detection
	detector.combinedPatternBytes = []byte(`" `) // Look for quotes and spaces
	detector.mainPatternBytes = []byte(`[`)      // Look for bracket patterns

	return detector
}

// DetectFormat detects log format with minimal overhead
func (flfd *FastLogFormatDetector) DetectFormat(logLine []byte) string {
	// Quick heuristic checks first (much faster than regex)
	quoteCount := 0
	bracketCount := 0

	for _, b := range logLine {
		switch b {
		case '"':
			quoteCount++
		case '[', ']':
			bracketCount++
		}

		// Early termination - if we have enough quotes, likely combined format
		if quoteCount >= 4 {
			return "combined"
		}
	}

	// If we have brackets but few quotes, likely main format
	if bracketCount >= 2 && quoteCount < 4 {
		return "main"
	}

	// Fallback to regex matching for edge cases
	logLineStr := string(logLine)
	if flfd.combinedRegex != nil && flfd.combinedRegex.MatchString(logLineStr) {
		return "combined"
	}

	if flfd.mainRegex != nil && flfd.mainRegex.MatchString(logLineStr) {
		return "main"
	}

	return "unknown"
}

// PatternPool manages a pool of compiled patterns for high-concurrency usage
type PatternPool struct {
	patterns map[string]*sync.Pool
	mutex    sync.RWMutex
}

// NewPatternPool creates a new pattern pool
func NewPatternPool() *PatternPool {
	return &PatternPool{
		patterns: make(map[string]*sync.Pool),
	}
}

// GetPattern gets a regex from the pool (creates if not exists)
func (pp *PatternPool) GetPattern(pattern string) (*regexp.Regexp, error) {
	pp.mutex.RLock()
	pool, exists := pp.patterns[pattern]
	pp.mutex.RUnlock()

	if !exists {
		// Create new pool for this pattern
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}

		newPool := &sync.Pool{
			New: func() interface{} {
				// Reuse compiled regex; in Go 1.12+ it's safe for concurrent use
				return regex
			},
		}

		pp.mutex.Lock()
		pp.patterns[pattern] = newPool
		pool = newPool
		pp.mutex.Unlock()
	}

	return pool.Get().(*regexp.Regexp), nil
}

// PutPattern returns a regex to the pool
func (pp *PatternPool) PutPattern(pattern string, regex *regexp.Regexp) {
	pp.mutex.RLock()
	pool, exists := pp.patterns[pattern]
	pp.mutex.RUnlock()

	if exists {
		pool.Put(regex)
	}
}
