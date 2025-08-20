package nginx_log

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/dgraph-io/ristretto/v2"
	"github.com/uozi-tech/cosy/logger"
)

// ConcurrentSearchProcessor provides high-performance concurrent search processing
type ConcurrentSearchProcessor struct {
	// Core components
	index           bleve.Index
	optimizedQuery  *OptimizedSearchQuery
	batchOptimizer  *BatchSearchOptimizer
	cache           *ristretto.Cache[string, *CachedSearchResult]
	
	// Concurrency configuration
	maxConcurrency  int
	semaphore       chan struct{}
	workerPool      *sync.Pool
	
	// Request queuing and load balancing
	requestQueue    chan *ConcurrentSearchRequest
	priorityQueue   chan *ConcurrentSearchRequest
	responseMap     *sync.Map
	
	// Circuit breaker and rate limiting
	circuitBreaker  *CircuitBreaker
	rateLimiter     *RateLimiter
	
	// Performance monitoring
	activeRequests  int64
	totalRequests   int64
	totalDuration   int64
	errorCount      int64
	timeoutCount    int64
	
	// Control channels
	stopChannel     chan struct{}
	wg              sync.WaitGroup
	
	// Configuration
	config          *ConcurrentSearchConfig
}

// ConcurrentSearchRequest represents a concurrent search request
type ConcurrentSearchRequest struct {
	ID          string
	Request     *QueryRequest
	Context     context.Context
	Priority    RequestPriority
	StartTime   time.Time
	Callback    func(*QueryResult, error)
	Response    chan *ConcurrentSearchResponse
}

// ConcurrentSearchResponse represents the response from concurrent search
type ConcurrentSearchResponse struct {
	ID       string
	Result   *QueryResult
	Error    error
	Duration time.Duration
	FromCache bool
	WorkerID int
}

// RequestPriority defines the priority of search requests
type RequestPriority int

const (
	PriorityLow RequestPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// ConcurrentSearchConfig holds configuration for concurrent search processing
type ConcurrentSearchConfig struct {
	Index              bleve.Index
	Cache              *ristretto.Cache[string, *CachedSearchResult]
	MaxConcurrency     int
	QueueSize          int
	RequestTimeout     time.Duration
	WorkerTimeout      time.Duration
	EnableCircuitBreaker bool
	EnableRateLimit    bool
	RateLimit          int // requests per second
	CircuitBreakerConfig *CircuitBreakerConfig
}

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	FailureThreshold   int
	SuccessThreshold   int
	Timeout           time.Duration
	MonitoringPeriod  time.Duration
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config          *CircuitBreakerConfig
	state           CircuitBreakerState
	failures        int64
	successes       int64
	lastFailureTime time.Time
	lastStateChange time.Time
	mu              sync.RWMutex
}

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	rate        int
	capacity    int
	tokens      int
	lastRefill  time.Time
	mu          sync.Mutex
}

// NewConcurrentSearchProcessor creates a new concurrent search processor
func NewConcurrentSearchProcessor(config *ConcurrentSearchConfig) (*ConcurrentSearchProcessor, error) {
	// Set defaults
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = runtime.NumCPU() * 4
	}
	if config.QueueSize == 0 {
		config.QueueSize = config.MaxConcurrency * 10
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	if config.WorkerTimeout == 0 {
		config.WorkerTimeout = 10 * time.Second
	}
	if config.RateLimit == 0 {
		config.RateLimit = 1000 // 1000 requests per second default
	}
	
	// Create optimized query processor
	optimizedQuery := NewOptimizedSearchQuery(&OptimizedQueryConfig{
		Index:         config.Index,
		Cache:         config.Cache,
		MaxCacheSize:  256 * 1024 * 1024, // 256MB
		CacheTTL:      15 * time.Minute,
		MaxResultSize: 50000,
	})
	
	// Create batch optimizer
	batchOptimizer := NewBatchSearchOptimizer(&BatchSearchConfig{
		SearchQuery:   optimizedQuery,
		Index:         config.Index,
		BatchSize:     10,
		WorkerCount:   config.MaxConcurrency / 2,
		BatchInterval: 50 * time.Millisecond,
		RequestTimeout: config.RequestTimeout,
	})
	
	csp := &ConcurrentSearchProcessor{
		index:           config.Index,
		optimizedQuery:  optimizedQuery,
		batchOptimizer:  batchOptimizer,
		cache:           config.Cache,
		maxConcurrency:  config.MaxConcurrency,
		semaphore:       make(chan struct{}, config.MaxConcurrency),
		requestQueue:    make(chan *ConcurrentSearchRequest, config.QueueSize),
		priorityQueue:   make(chan *ConcurrentSearchRequest, config.QueueSize/4),
		responseMap:     &sync.Map{},
		stopChannel:     make(chan struct{}),
		config:          config,
		
		workerPool: &sync.Pool{
			New: func() interface{} {
				return &SearchWorker{
					ID: fmt.Sprintf("worker_%d", time.Now().UnixNano()),
				}
			},
		},
	}
	
	// Initialize circuit breaker if enabled
	if config.EnableCircuitBreaker {
		cbConfig := config.CircuitBreakerConfig
		if cbConfig == nil {
			cbConfig = &CircuitBreakerConfig{
				FailureThreshold:  10,
				SuccessThreshold:  5,
				Timeout:          30 * time.Second,
				MonitoringPeriod: 60 * time.Second,
			}
		}
		csp.circuitBreaker = NewCircuitBreaker(cbConfig)
	}
	
	// Initialize rate limiter if enabled
	if config.EnableRateLimit {
		csp.rateLimiter = NewRateLimiter(config.RateLimit, config.RateLimit*2)
	}
	
	// Start workers
	csp.startWorkers()
	
	return csp, nil
}

// SearchWorker represents a search worker
type SearchWorker struct {
	ID           string
	RequestCount int64
	TotalTime    time.Duration
}

// startWorkers starts the concurrent search workers
func (csp *ConcurrentSearchProcessor) startWorkers() {
	// Start request dispatcher
	csp.wg.Add(1)
	go csp.requestDispatcher()
	
	// Start worker pool
	for i := 0; i < csp.maxConcurrency; i++ {
		csp.wg.Add(1)
		go csp.searchWorker(i)
	}
	
	// Start monitoring goroutine
	csp.wg.Add(1)
	go csp.performanceMonitor()
	
	logger.Infof("Started concurrent search processor with %d workers", csp.maxConcurrency)
}

// SearchConcurrent performs a concurrent search
func (csp *ConcurrentSearchProcessor) SearchConcurrent(ctx context.Context, req *QueryRequest, priority RequestPriority) (*QueryResult, error) {
	// Check rate limiter
	if csp.rateLimiter != nil && !csp.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded")
	}
	
	// Check circuit breaker
	if csp.circuitBreaker != nil && !csp.circuitBreaker.Allow() {
		return nil, fmt.Errorf("circuit breaker is open")
	}
	
	// Create search request
	searchReq := &ConcurrentSearchRequest{
		ID:        fmt.Sprintf("req_%d", time.Now().UnixNano()),
		Request:   req,
		Context:   ctx,
		Priority:  priority,
		StartTime: time.Now(),
		Response:  make(chan *ConcurrentSearchResponse, 1),
	}
	
	// Submit request
	select {
	case csp.priorityQueue <- searchReq:
		// High priority request submitted
	case csp.requestQueue <- searchReq:
		// Normal priority request submitted
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(csp.config.RequestTimeout):
		atomic.AddInt64(&csp.timeoutCount, 1)
		return nil, fmt.Errorf("request submission timeout")
	}
	
	// Wait for response
	select {
	case response := <-searchReq.Response:
		// Update circuit breaker
		if csp.circuitBreaker != nil {
			if response.Error != nil {
				csp.circuitBreaker.RecordFailure()
			} else {
				csp.circuitBreaker.RecordSuccess()
			}
		}
		
		if response.Error != nil {
			atomic.AddInt64(&csp.errorCount, 1)
			return nil, response.Error
		}
		
		return response.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(csp.config.RequestTimeout):
		atomic.AddInt64(&csp.timeoutCount, 1)
		return nil, fmt.Errorf("request processing timeout")
	}
}

// requestDispatcher dispatches requests to workers based on priority
func (csp *ConcurrentSearchProcessor) requestDispatcher() {
	defer csp.wg.Done()
	
	for {
		select {
		case req := <-csp.priorityQueue:
			// High priority request - process immediately
			csp.processRequest(req)
		case req := <-csp.requestQueue:
			// Normal priority request
			csp.processRequest(req)
		case <-csp.stopChannel:
			return
		}
	}
}

// processRequest processes a search request
func (csp *ConcurrentSearchProcessor) processRequest(req *ConcurrentSearchRequest) {
	// Acquire semaphore slot
	select {
	case csp.semaphore <- struct{}{}:
		// Slot acquired, process request
		go func() {
			defer func() { <-csp.semaphore }()
			csp.executeRequest(req)
		}()
	case <-time.After(csp.config.WorkerTimeout):
		// No workers available, return error
		req.Response <- &ConcurrentSearchResponse{
			ID:    req.ID,
			Error: fmt.Errorf("no workers available"),
		}
	case <-csp.stopChannel:
		// Shutting down
		return
	}
}

// executeRequest executes a search request
func (csp *ConcurrentSearchProcessor) executeRequest(req *ConcurrentSearchRequest) {
	start := time.Now()
	atomic.AddInt64(&csp.activeRequests, 1)
	atomic.AddInt64(&csp.totalRequests, 1)
	
	defer func() {
		atomic.AddInt64(&csp.activeRequests, -1)
		duration := time.Since(start)
		atomic.AddInt64(&csp.totalDuration, duration.Nanoseconds())
	}()
	
	// Get worker from pool
	worker := csp.workerPool.Get().(*SearchWorker)
	defer csp.workerPool.Put(worker)
	
	// Execute search using optimized query processor
	result, err := csp.optimizedQuery.SearchLogsOptimized(req.Context, req.Request)
	
	// Create response
	response := &ConcurrentSearchResponse{
		ID:       req.ID,
		Result:   result,
		Error:    err,
		Duration: time.Since(start),
		WorkerID: 0, // Use numeric worker ID
	}
	
	if result != nil {
		response.FromCache = result.FromCache
	}
	
	// Send response
	select {
	case req.Response <- response:
	case <-req.Context.Done():
		// Request context cancelled
	case <-time.After(1 * time.Second):
		// Response channel blocked, log warning
		logger.Warnf("Response channel blocked for request %s", req.ID)
	}
}

// searchWorker is a dedicated search worker (currently using request dispatcher)
func (csp *ConcurrentSearchProcessor) searchWorker(workerID int) {
	defer csp.wg.Done()
	
	// This worker is now handled by the request dispatcher
	// We keep this for future direct worker implementation if needed
	for {
		select {
		case <-csp.stopChannel:
			return
		case <-time.After(100 * time.Millisecond):
			// Worker heartbeat
			continue
		}
	}
}

// performanceMonitor monitors performance metrics
func (csp *ConcurrentSearchProcessor) performanceMonitor() {
	defer csp.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			stats := csp.GetStatistics()
			logger.Infof("Concurrent search stats: %+v", stats)
		case <-csp.stopChannel:
			return
		}
	}
}

// GetStatistics returns performance statistics
func (csp *ConcurrentSearchProcessor) GetStatistics() map[string]interface{} {
	active := atomic.LoadInt64(&csp.activeRequests)
	total := atomic.LoadInt64(&csp.totalRequests)
	totalDur := atomic.LoadInt64(&csp.totalDuration)
	errors := atomic.LoadInt64(&csp.errorCount)
	timeouts := atomic.LoadInt64(&csp.timeoutCount)
	
	avgDuration := float64(0)
	if total > 0 {
		avgDuration = float64(totalDur) / float64(total) / 1e6 // Convert to milliseconds
	}
	
	errorRate := float64(0)
	if total > 0 {
		errorRate = float64(errors) / float64(total) * 100
	}
	
	stats := map[string]interface{}{
		"active_requests":     active,
		"total_requests":      total,
		"error_count":         errors,
		"timeout_count":       timeouts,
		"error_rate_percent":  fmt.Sprintf("%.2f", errorRate),
		"avg_duration_ms":     fmt.Sprintf("%.2f", avgDuration),
		"max_concurrency":     csp.maxConcurrency,
		"queue_size":          len(csp.requestQueue),
		"priority_queue_size": len(csp.priorityQueue),
	}
	
	// Add circuit breaker stats
	if csp.circuitBreaker != nil {
		cbStats := csp.circuitBreaker.GetStatistics()
		stats["circuit_breaker"] = cbStats
	}
	
	// Add rate limiter stats
	if csp.rateLimiter != nil {
		rlStats := csp.rateLimiter.GetStatistics()
		stats["rate_limiter"] = rlStats
	}
	
	// Add optimized query stats
	if csp.optimizedQuery != nil {
		oqStats := csp.optimizedQuery.GetStatistics()
		stats["optimized_query"] = oqStats
	}
	
	return stats
}

// Close shuts down the concurrent search processor
func (csp *ConcurrentSearchProcessor) Close() error {
	// Signal all workers to stop
	close(csp.stopChannel)
	
	// Wait for all workers to finish
	csp.wg.Wait()
	
	// Close batch optimizer
	if csp.batchOptimizer != nil {
		csp.batchOptimizer.Close()
	}
	
	// Close channels
	close(csp.requestQueue)
	close(csp.priorityQueue)
	
	logger.Infof("Concurrent search processor closed. Final stats: %+v", csp.GetStatistics())
	return nil
}

// Circuit Breaker Implementation

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config *CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:          config,
		state:           CircuitClosed,
		lastStateChange: time.Now(),
	}
}

// Allow checks if a request should be allowed through the circuit breaker
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed
		if time.Since(cb.lastStateChange) > cb.config.Timeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			if cb.state == CircuitOpen && time.Since(cb.lastStateChange) > cb.config.Timeout {
				cb.state = CircuitHalfOpen
				cb.lastStateChange = time.Now()
			}
			cb.mu.Unlock()
			cb.mu.RLock()
			return cb.state == CircuitHalfOpen
		}
		return false
	case CircuitHalfOpen:
		return true
	default:
		return false
	}
}

// RecordSuccess records a successful operation
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.AddInt64(&cb.successes, 1)
	
	if cb.state == CircuitHalfOpen {
		if cb.successes >= int64(cb.config.SuccessThreshold) {
			cb.state = CircuitClosed
			cb.failures = 0
			cb.successes = 0
			cb.lastStateChange = time.Now()
		}
	}
}

// RecordFailure records a failed operation
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	atomic.AddInt64(&cb.failures, 1)
	cb.lastFailureTime = time.Now()
	
	if cb.state == CircuitClosed {
		if cb.failures >= int64(cb.config.FailureThreshold) {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
		}
	} else if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	}
}

// GetStatistics returns circuit breaker statistics
func (cb *CircuitBreaker) GetStatistics() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	stateStr := "closed"
	switch cb.state {
	case CircuitOpen:
		stateStr = "open"
	case CircuitHalfOpen:
		stateStr = "half-open"
	}
	
	return map[string]interface{}{
		"state":            stateStr,
		"failures":         cb.failures,
		"successes":        cb.successes,
		"last_state_change": cb.lastStateChange.Format(time.RFC3339),
		"failure_threshold": cb.config.FailureThreshold,
		"success_threshold": cb.config.SuccessThreshold,
	}
}

// Rate Limiter Implementation

// NewRateLimiter creates a new token bucket rate limiter
func NewRateLimiter(rate, capacity int) *RateLimiter {
	return &RateLimiter{
		rate:       rate,
		capacity:   capacity,
		tokens:     capacity,
		lastRefill: time.Now(),
	}
}

// Allow checks if a request should be allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	
	// Refill tokens based on time passed
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))
	
	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.capacity {
			rl.tokens = rl.capacity
		}
		rl.lastRefill = now
	}
	
	// Check if we have tokens available
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}
	
	return false
}

// GetStatistics returns rate limiter statistics
func (rl *RateLimiter) GetStatistics() map[string]interface{} {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	return map[string]interface{}{
		"rate":       rl.rate,
		"capacity":   rl.capacity,
		"tokens":     rl.tokens,
		"last_refill": rl.lastRefill.Format(time.RFC3339),
	}
}