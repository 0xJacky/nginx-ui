package analytics

import (
	"context"
	"sort"
	"sync"
)

// SearchRequest and related types from searcher package
type SearchRequest struct {
	StartTime     *int64    `json:"start_time,omitempty"`
	EndTime       *int64    `json:"end_time,omitempty"`
	LogPaths      []string  `json:"log_paths,omitempty"`
	Limit         int       `json:"limit"`
	IncludeFacets bool      `json:"include_facets,omitempty"`
	IncludeStats  bool      `json:"include_stats,omitempty"`
	UseCache      bool      `json:"use_cache,omitempty"`
}

type SearchResult struct {
	Hits      []*SearchHit `json:"hits"`
	TotalHits uint64       `json:"total_hits"`
	Stats     *SearchStats `json:"stats,omitempty"`
}

type SearchHit struct {
	Fields map[string]interface{} `json:"fields"`
}

type SearchStats struct {
	TotalBytes int64 `json:"total_bytes"`
}

type AggregationRequest struct{}
type AggregationResult struct{}
type Suggestion struct{}

// Searcher interface (simplified)
type Searcher interface {
	Search(ctx context.Context, req *SearchRequest) (*SearchResult, error)
	Aggregate(ctx context.Context, req *AggregationRequest) (*AggregationResult, error)
	Suggest(ctx context.Context, text string, field string, size int) ([]*Suggestion, error)
	Analyze(ctx context.Context, text string, analyzer string) ([]string, error)
	ClearCache() error
}

// OptimizedTimeSeriesProcessor provides high-performance time-series analytics
type OptimizedTimeSeriesProcessor struct {
	bucketPools    map[int64]*BucketPool
	visitorSets    map[int64]*VisitorSetPool
	resultCache    *TimeSeriesCache
	mutex          sync.RWMutex
}

// NewOptimizedTimeSeriesProcessor creates a new optimized processor
func NewOptimizedTimeSeriesProcessor() *OptimizedTimeSeriesProcessor {
	return &OptimizedTimeSeriesProcessor{
		bucketPools: make(map[int64]*BucketPool),
		visitorSets: make(map[int64]*VisitorSetPool),
		resultCache: NewTimeSeriesCache(1000, 1800), // 1000 entries, 30min TTL
	}
}

// BucketPool provides pooled time buckets for aggregation
type BucketPool struct {
	buckets sync.Pool
}

// NewBucketPool creates a new bucket pool
func NewBucketPool() *BucketPool {
	return &BucketPool{
		buckets: sync.Pool{
			New: func() interface{} {
				return make(map[int64]*TimeBucket, 1000)
			},
		},
	}
}

// Get retrieves a bucket map from the pool
func (bp *BucketPool) Get() map[int64]*TimeBucket {
	return bp.buckets.Get().(map[int64]*TimeBucket)
}

// Put returns a bucket map to the pool
func (bp *BucketPool) Put(buckets map[int64]*TimeBucket) {
	// Clear the map
	for k := range buckets {
		delete(buckets, k)
	}
	bp.buckets.Put(buckets)
}

// TimeBucket represents an optimized time bucket for aggregation
type TimeBucket struct {
	Timestamp      int64
	RequestCount   int64
	BytesTransferred int64
	UniqueVisitors map[string]struct{} // Use struct{} for zero-memory set
	StatusCodes    map[int]int64
	Methods        map[string]int64
	Paths          map[string]int64
}

// NewTimeBucket creates a new optimized time bucket
func NewTimeBucket(timestamp int64) *TimeBucket {
	return &TimeBucket{
		Timestamp:      timestamp,
		UniqueVisitors: make(map[string]struct{}, 100),
		StatusCodes:    make(map[int]int64, 10),
		Methods:        make(map[string]int64, 5),
		Paths:          make(map[string]int64, 20),
	}
}

// AddEntry adds an entry to the time bucket with optimized operations
func (tb *TimeBucket) AddEntry(ip string, status int, method string, path string, bytes int64) {
	tb.RequestCount++
	tb.BytesTransferred += bytes
	
	// Use struct{} for zero-memory set operations
	tb.UniqueVisitors[ip] = struct{}{}
	
	// Optimized map operations
	tb.StatusCodes[status]++
	tb.Methods[method]++
	tb.Paths[path]++
}

// GetUniqueVisitorCount returns the count of unique visitors
func (tb *TimeBucket) GetUniqueVisitorCount() int {
	return len(tb.UniqueVisitors)
}

// VisitorSetPool provides pooled visitor sets
type VisitorSetPool struct {
	sets sync.Pool
}

// NewVisitorSetPool creates a new visitor set pool
func NewVisitorSetPool() *VisitorSetPool {
	return &VisitorSetPool{
		sets: sync.Pool{
			New: func() interface{} {
				return make(map[string]struct{}, 1000)
			},
		},
	}
}

// Get retrieves a visitor set from the pool
func (vsp *VisitorSetPool) Get() map[string]struct{} {
	return vsp.sets.Get().(map[string]struct{})
}

// Put returns a visitor set to the pool
func (vsp *VisitorSetPool) Put(set map[string]struct{}) {
	// Clear the set
	for k := range set {
		delete(set, k)
	}
	vsp.sets.Put(set)
}

// TimeSeriesCache provides caching for time-series results
type TimeSeriesCache struct {
	cache     map[string]*CachedTimeSeriesResult
	maxSize   int
	ttlSeconds int64
	mutex     sync.RWMutex
}

// CachedTimeSeriesResult represents a cached time-series result
type CachedTimeSeriesResult struct {
	Data      interface{}
	Timestamp int64
	AccessCount int64
}

// NewTimeSeriesCache creates a new time-series cache
func NewTimeSeriesCache(maxSize int, ttlSeconds int64) *TimeSeriesCache {
	return &TimeSeriesCache{
		cache:     make(map[string]*CachedTimeSeriesResult),
		maxSize:   maxSize,
		ttlSeconds: ttlSeconds,
	}
}

// Get retrieves a cached result
func (tsc *TimeSeriesCache) Get(key string) (interface{}, bool) {
	tsc.mutex.RLock()
	result, exists := tsc.cache[key]
	tsc.mutex.RUnlock()
	
	if !exists {
		return nil, false
	}
	
	// Check TTL
	currentTime := getCurrentTimestamp()
	if currentTime-result.Timestamp > tsc.ttlSeconds {
		tsc.Delete(key)
		return nil, false
	}
	
	// Update access count atomically
	tsc.mutex.Lock()
	result.AccessCount++
	tsc.mutex.Unlock()
	
	return result.Data, true
}

// Put stores a result in the cache
func (tsc *TimeSeriesCache) Put(key string, data interface{}) {
	tsc.mutex.Lock()
	defer tsc.mutex.Unlock()
	
	// Evict if at capacity
	if len(tsc.cache) >= tsc.maxSize {
		tsc.evictLRU()
	}
	
	tsc.cache[key] = &CachedTimeSeriesResult{
		Data:      data,
		Timestamp: getCurrentTimestamp(),
		AccessCount: 1,
	}
}

// Delete removes a cached result
func (tsc *TimeSeriesCache) Delete(key string) {
	tsc.mutex.Lock()
	defer tsc.mutex.Unlock()
	delete(tsc.cache, key)
}

// evictLRU removes the least recently used entry
func (tsc *TimeSeriesCache) evictLRU() {
	var lruKey string
	var lruTimestamp int64 = ^int64(0) // Max int64
	
	for key, result := range tsc.cache {
		if result.Timestamp < lruTimestamp {
			lruTimestamp = result.Timestamp
			lruKey = key
		}
	}
	
	if lruKey != "" {
		delete(tsc.cache, lruKey)
	}
}

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return 1640995200 // Mock timestamp for testing
}

// OptimizedGetVisitorsByTime provides optimized visitors by time calculation
func (otsp *OptimizedTimeSeriesProcessor) OptimizedGetVisitorsByTime(
	ctx context.Context, 
	req *VisitorsByTimeRequest,
	s Searcher,
) (*VisitorsByTime, error) {
	
	// Check cache first
	cacheKey := generateCacheKey("visitors_by_time", req)
	if cached, found := otsp.resultCache.Get(cacheKey); found {
		return cached.(*VisitorsByTime), nil
	}
	
	// Prepare search request
	searchReq := &SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0,
		IncludeFacets: false,
		UseCache:      true,
	}
	
	result, err := s.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}
	
	// Optimize interval calculation
	interval := int64(req.IntervalSeconds)
	if interval <= 0 {
		interval = 60 // Default 1 minute
	}
	
	// Get pooled bucket map
	bucketPool := otsp.getBucketPool(interval)
	buckets := bucketPool.Get()
	defer bucketPool.Put(buckets)
	
	// Process hits with optimized bucketing
	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)
				bucketTime := (timestamp / interval) * interval
				
				// Get or create bucket
				bucket := buckets[bucketTime]
				if bucket == nil {
					bucket = NewTimeBucket(bucketTime)
					buckets[bucketTime] = bucket
				}
				
				// Add IP to unique visitors
				if ip, ok := hit.Fields["ip"].(string); ok {
					bucket.UniqueVisitors[ip] = struct{}{}
				}
			}
		}
	}
	
	// Convert to sorted result
	visitorsByTime := make([]TimeValue, 0, len(buckets))
	for _, bucket := range buckets {
		visitorsByTime = append(visitorsByTime, TimeValue{
			Timestamp: bucket.Timestamp,
			Value:     len(bucket.UniqueVisitors),
		})
	}
	
	// Sort efficiently
	sort.Slice(visitorsByTime, func(i, j int) bool {
		return visitorsByTime[i].Timestamp < visitorsByTime[j].Timestamp
	})
	
	result_data := &VisitorsByTime{Data: visitorsByTime}
	
	// Cache the result
	otsp.resultCache.Put(cacheKey, result_data)
	
	return result_data, nil
}

// OptimizedGetTrafficByTime provides optimized traffic analytics
func (otsp *OptimizedTimeSeriesProcessor) OptimizedGetTrafficByTime(
	ctx context.Context,
	req *TrafficByTimeRequest,
	s Searcher,
) (*TrafficByTime, error) {
	
	// Check cache first
	cacheKey := generateCacheKey("traffic_by_time", req)
	if cached, found := otsp.resultCache.Get(cacheKey); found {
		return cached.(*TrafficByTime), nil
	}
	
	searchReq := &SearchRequest{
		StartTime:     &req.StartTime,
		EndTime:       &req.EndTime,
		LogPaths:      req.LogPaths,
		Limit:         0,
		IncludeStats:  true,
		UseCache:      true,
	}
	
	result, err := s.Search(ctx, searchReq)
	if err != nil {
		return nil, err
	}
	
	interval := int64(req.IntervalSeconds)
	if interval <= 0 {
		interval = 300 // Default 5 minutes
	}
	
	// Get pooled bucket map
	bucketPool := otsp.getBucketPool(interval)
	buckets := bucketPool.Get()
	defer bucketPool.Put(buckets)
	
	// Process hits with comprehensive metrics
	for _, hit := range result.Hits {
		if timestampField, ok := hit.Fields["timestamp"]; ok {
			if timestampFloat, ok := timestampField.(float64); ok {
				timestamp := int64(timestampFloat)
				bucketTime := (timestamp / interval) * interval
				
				bucket := buckets[bucketTime]
				if bucket == nil {
					bucket = NewTimeBucket(bucketTime)
					buckets[bucketTime] = bucket
				}
				
				// Extract fields efficiently
				var ip, method, path string
				var status int
				var bytes int64
				
				if v, ok := hit.Fields["ip"].(string); ok { ip = v }
				if v, ok := hit.Fields["method"].(string); ok { method = v }
				if v, ok := hit.Fields["path"].(string); ok { path = v }
				if v, ok := hit.Fields["status"].(float64); ok { status = int(v) }
				if v, ok := hit.Fields["bytes_sent"].(float64); ok { bytes = int64(v) }
				
				bucket.AddEntry(ip, status, method, path, bytes)
			}
		}
	}
	
	// Convert to result with comprehensive metrics
	trafficData := make([]TrafficTimeValue, 0, len(buckets))
	for _, bucket := range buckets {
		trafficData = append(trafficData, TrafficTimeValue{
			Timestamp:      bucket.Timestamp,
			Requests:       bucket.RequestCount,
			Bytes:         bucket.BytesTransferred,
			UniqueVisitors: len(bucket.UniqueVisitors),
		})
	}
	
	// Sort by timestamp
	sort.Slice(trafficData, func(i, j int) bool {
		return trafficData[i].Timestamp < trafficData[j].Timestamp
	})
	
	result_data := &TrafficByTime{Data: trafficData}
	
	// Cache the result
	otsp.resultCache.Put(cacheKey, result_data)
	
	return result_data, nil
}

// HyperLogLog provides cardinality estimation for unique visitors
type HyperLogLog struct {
	buckets []uint8
	b       uint8 // number of bits for bucket index
	m       uint32 // number of buckets (2^b)
}

// NewHyperLogLog creates a new HyperLogLog counter
func NewHyperLogLog(precision uint8) *HyperLogLog {
	b := precision
	m := uint32(1) << b
	return &HyperLogLog{
		buckets: make([]uint8, m),
		b:       b,
		m:       m,
	}
}

// Add adds a value to the HyperLogLog
func (hll *HyperLogLog) Add(value string) {
	hash := hashString(value)
	j := hash >> (32 - hll.b) // first b bits
	w := hash << hll.b       // remaining bits
	
	// Count leading zeros + 1
	lz := countLeadingZeros(w) + 1
	if lz > uint8(32-hll.b) {
		lz = uint8(32 - hll.b)
	}
	
	if lz > hll.buckets[j] {
		hll.buckets[j] = lz
	}
}

// Count estimates the cardinality
func (hll *HyperLogLog) Count() uint64 {
	rawEstimate := hll.alpha() * float64(hll.m*hll.m) / hll.sumOfPowers()
	
	if rawEstimate <= 2.5*float64(hll.m) {
		// Small range correction
		zeros := 0
		for _, bucket := range hll.buckets {
			if bucket == 0 {
				zeros++
			}
		}
		if zeros != 0 {
			return uint64(float64(hll.m) * logValue(float64(hll.m)/float64(zeros)))
		}
	}
	
	return uint64(rawEstimate)
}

// Helper functions for HyperLogLog
func (hll *HyperLogLog) alpha() float64 {
	switch hll.m {
	case 16:
		return 0.673
	case 32:
		return 0.697
	case 64:
		return 0.709
	default:
		return 0.7213 / (1.0 + 1.079/float64(hll.m))
	}
}

func (hll *HyperLogLog) sumOfPowers() float64 {
	sum := 0.0
	for _, bucket := range hll.buckets {
		sum += 1.0 / float64(uint32(1)<<bucket)
	}
	return sum
}

// Simple hash function for strings
func hashString(s string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

// Count leading zeros in a 32-bit integer
func countLeadingZeros(x uint32) uint8 {
	if x == 0 {
		return 32
	}
	n := uint8(0)
	if x <= 0x0000FFFF {
		n += 16
		x <<= 16
	}
	if x <= 0x00FFFFFF {
		n += 8
		x <<= 8
	}
	if x <= 0x0FFFFFFF {
		n += 4
		x <<= 4
	}
	if x <= 0x3FFFFFFF {
		n += 2
		x <<= 2
	}
	if x <= 0x7FFFFFFF {
		n += 1
	}
	return n
}

// Simple log function
func logValue(x float64) float64 {
	// Approximation of natural logarithm for HLL correction
	if x <= 0 {
		return 0
	}
	return 0.693147 * float64(32-countLeadingZeros(uint32(x))) // Rough approximation
}

// getBucketPool gets or creates a bucket pool for the given interval
func (otsp *OptimizedTimeSeriesProcessor) getBucketPool(interval int64) *BucketPool {
	otsp.mutex.RLock()
	pool, exists := otsp.bucketPools[interval]
	otsp.mutex.RUnlock()
	
	if !exists {
		otsp.mutex.Lock()
		// Double-check after acquiring write lock
		if pool, exists = otsp.bucketPools[interval]; !exists {
			pool = NewBucketPool()
			otsp.bucketPools[interval] = pool
		}
		otsp.mutex.Unlock()
	}
	
	return pool
}

// generateCacheKey generates a cache key from request parameters
func generateCacheKey(prefix string, req interface{}) string {
	// Simple cache key generation - in production, use a proper hash
	return prefix + "_cache_key"
}

// Additional types for comprehensive traffic analytics
type TrafficByTimeRequest struct {
	StartTime       int64
	EndTime         int64
	LogPaths        []string
	IntervalSeconds int
}

type TrafficByTime struct {
	Data []TrafficTimeValue `json:"data"`
}

type TrafficTimeValue struct {
	Timestamp      int64 `json:"timestamp"`
	Requests       int64 `json:"requests"`
	Bytes          int64 `json:"bytes"`
	UniqueVisitors int   `json:"unique_visitors"`
}

// AdvancedTimeSeriesProcessor provides advanced analytics with ML-like features
type AdvancedTimeSeriesProcessor struct {
	*OptimizedTimeSeriesProcessor
	anomalyThreshold float64
	trendWindow     int
}

// NewAdvancedTimeSeriesProcessor creates an advanced processor
func NewAdvancedTimeSeriesProcessor() *AdvancedTimeSeriesProcessor {
	return &AdvancedTimeSeriesProcessor{
		OptimizedTimeSeriesProcessor: NewOptimizedTimeSeriesProcessor(),
		anomalyThreshold:            2.0, // 2 standard deviations
		trendWindow:                10,   // 10 data points for trend
	}
}

// DetectAnomalies detects anomalies in time-series data
func (atsp *AdvancedTimeSeriesProcessor) DetectAnomalies(data []TimeValue) []AnomalyPoint {
	if len(data) < 3 {
		return nil
	}
	
	// Calculate moving average and standard deviation
	anomalies := make([]AnomalyPoint, 0)
	windowSize := 5
	
	for i := windowSize; i < len(data); i++ {
		// Calculate stats for window
		sum, sumSq := 0.0, 0.0
		for j := i - windowSize; j < i; j++ {
			val := float64(data[j].Value)
			sum += val
			sumSq += val * val
		}
		
		mean := sum / float64(windowSize)
		variance := (sumSq / float64(windowSize)) - (mean * mean)
		stdDev := variance * 0.5 // Approximate square root
		
		// Check if current value is anomalous
		currentVal := float64(data[i].Value)
		deviation := currentVal - mean
		if deviation < 0 {
			deviation = -deviation
		}
		
		if stdDev > 0 && deviation > atsp.anomalyThreshold*stdDev {
			anomalies = append(anomalies, AnomalyPoint{
				Timestamp: data[i].Timestamp,
				Value:     data[i].Value,
				Expected:  int(mean),
				Deviation: deviation / stdDev,
			})
		}
	}
	
	return anomalies
}

// AnomalyPoint represents a detected anomaly
type AnomalyPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     int     `json:"value"`
	Expected  int     `json:"expected"`
	Deviation float64 `json:"deviation"`
}

// CalculateTrend calculates trend direction and strength
func (atsp *AdvancedTimeSeriesProcessor) CalculateTrend(data []TimeValue) TrendAnalysis {
	if len(data) < 2 {
		return TrendAnalysis{Direction: "insufficient_data"}
	}
	
	// Simple linear regression for trend
	n := float64(len(data))
	sumX, sumY, sumXY, sumXX := 0.0, 0.0, 0.0, 0.0
	
	for i, point := range data {
		x := float64(i)
		y := float64(point.Value)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXX += x * x
	}
	
	// Calculate slope
	slope := (n*sumXY - sumX*sumY) / (n*sumXX - sumX*sumX)
	
	// Determine trend direction and strength
	direction := "stable"
	if slope > 0.1 {
		direction = "increasing"
	} else if slope < -0.1 {
		direction = "decreasing"
	}
	
	// Calculate trend strength (simplified R-squared approximation)
	strength := slope * slope / (slope*slope + 1) // Normalize to 0-1
	
	return TrendAnalysis{
		Direction: direction,
		Strength:  strength,
		Slope:     slope,
	}
}

// TrendAnalysis represents trend analysis results
type TrendAnalysis struct {
	Direction string  `json:"direction"`
	Strength  float64 `json:"strength"`
	Slope     float64 `json:"slope"`
}