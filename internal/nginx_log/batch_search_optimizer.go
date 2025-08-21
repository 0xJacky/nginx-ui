package nginx_log

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// BatchSearchOptimizer handles multiple search requests efficiently
type BatchSearchOptimizer struct {
	searchQuery     *OptimizedSearchQuery
	index           bleve.Index
	batchSize       int
	workerCount     int
	requestTimeout  time.Duration
	
	// Batch processing channels
	requestChannel  chan *BatchSearchRequest
	responseChannel chan *BatchSearchResponse
	errorChannel    chan error
	
	// Control channels
	stopChannel     chan struct{}
	wg              sync.WaitGroup
	
	// Request batching
	requestBatch    []*BatchSearchRequest
	batchMutex      sync.Mutex
	batchTimer      *time.Timer
	batchInterval   time.Duration
	
	// Statistics
	totalBatches    int64
	totalRequests   int64
	avgBatchSize    float64
	mu              sync.RWMutex
}

// BatchSearchRequest represents a single search request in a batch
type BatchSearchRequest struct {
	ID              string
	Request         *QueryRequest
	Context         context.Context
	ResponseChannel chan *BatchSearchResponse
	StartTime       time.Time
}

// BatchSearchResponse represents the response for a batch search request
type BatchSearchResponse struct {
	ID       string
	Result   *QueryResult
	Error    error
	Duration time.Duration
}

// BatchOptimizationResult contains results from batch optimization
type BatchOptimizationResult struct {
	Requests        []*BatchSearchRequest
	OptimizedQuery  query.Query
	SharedFilters   map[string]interface{}
	CanBatch        bool
}

// BatchSearchConfig holds configuration for batch search optimization
type BatchSearchConfig struct {
	SearchQuery     *OptimizedSearchQuery
	Index           bleve.Index
	BatchSize       int
	WorkerCount     int
	BatchInterval   time.Duration
	RequestTimeout  time.Duration
}

// NewBatchSearchOptimizer creates a new batch search optimizer
func NewBatchSearchOptimizer(config *BatchSearchConfig) *BatchSearchOptimizer {
	// Set defaults
	if config.BatchSize == 0 {
		config.BatchSize = 10
	}
	if config.WorkerCount == 0 {
		config.WorkerCount = runtime.NumCPU()
	}
	if config.BatchInterval == 0 {
		config.BatchInterval = 50 * time.Millisecond
	}
	if config.RequestTimeout == 0 {
		config.RequestTimeout = 30 * time.Second
	}
	
	bso := &BatchSearchOptimizer{
		searchQuery:     config.SearchQuery,
		index:           config.Index,
		batchSize:       config.BatchSize,
		workerCount:     config.WorkerCount,
		requestTimeout:  config.RequestTimeout,
		batchInterval:   config.BatchInterval,
		
		requestChannel:  make(chan *BatchSearchRequest, config.BatchSize*2),
		responseChannel: make(chan *BatchSearchResponse, config.BatchSize*2),
		errorChannel:    make(chan error, config.WorkerCount),
		stopChannel:     make(chan struct{}),
		
		requestBatch:    make([]*BatchSearchRequest, 0, config.BatchSize),
	}
	
	// Start batch processing workers
	bso.startWorkers()
	
	return bso
}

// startWorkers starts the batch processing workers
func (bso *BatchSearchOptimizer) startWorkers() {
	// Start batch collector
	bso.wg.Add(1)
	go bso.batchCollector()
	
	// Start batch processors
	for i := 0; i < bso.workerCount; i++ {
		bso.wg.Add(1)
		go bso.batchProcessor(i)
	}
	
	logger.Infof("Started batch search optimizer with %d workers, batch size %d", 
		bso.workerCount, bso.batchSize)
}

// SearchAsync submits a search request for batch processing
func (bso *BatchSearchOptimizer) SearchAsync(ctx context.Context, req *QueryRequest) (*QueryResult, error) {
	// Create batch request
	batchReq := &BatchSearchRequest{
		ID:              fmt.Sprintf("req_%d_%d", time.Now().UnixNano(), len(req.Query)),
		Request:         req,
		Context:         ctx,
		ResponseChannel: make(chan *BatchSearchResponse, 1),
		StartTime:       time.Now(),
	}
	
	// Submit request
	select {
	case bso.requestChannel <- batchReq:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(bso.requestTimeout):
		return nil, fmt.Errorf("request submission timeout")
	}
	
	// Wait for response
	select {
	case response := <-batchReq.ResponseChannel:
		if response.Error != nil {
			return nil, response.Error
		}
		return response.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(bso.requestTimeout):
		return nil, fmt.Errorf("request processing timeout")
	}
}

// batchCollector collects individual requests into batches
func (bso *BatchSearchOptimizer) batchCollector() {
	defer bso.wg.Done()
	
	bso.batchTimer = time.NewTimer(bso.batchInterval)
	defer bso.batchTimer.Stop()
	
	for {
		select {
		case req := <-bso.requestChannel:
			bso.batchMutex.Lock()
			bso.requestBatch = append(bso.requestBatch, req)
			shouldProcess := len(bso.requestBatch) >= bso.batchSize
			bso.batchMutex.Unlock()
			
			if shouldProcess {
				bso.processBatch()
			} else if len(bso.requestBatch) == 1 {
				// First request in batch, reset timer
				bso.batchTimer.Reset(bso.batchInterval)
			}
			
		case <-bso.batchTimer.C:
			bso.processBatch()
			
		case <-bso.stopChannel:
			// Process final batch
			bso.processBatch()
			return
		}
	}
}

// processBatch processes the current batch of requests
func (bso *BatchSearchOptimizer) processBatch() {
	bso.batchMutex.Lock()
	if len(bso.requestBatch) == 0 {
		bso.batchMutex.Unlock()
		return
	}
	
	// Copy batch and reset
	batch := make([]*BatchSearchRequest, len(bso.requestBatch))
	copy(batch, bso.requestBatch)
	bso.requestBatch = bso.requestBatch[:0]
	bso.batchMutex.Unlock()
	
	// Send batch for processing
	select {
	case bso.responseChannel <- &BatchSearchResponse{ID: "batch", Error: fmt.Errorf("batch_marker")}:
		// Send individual requests
		for _, req := range batch {
			select {
			case bso.responseChannel <- &BatchSearchResponse{ID: req.ID, Error: fmt.Errorf("process_individual")}:
			case <-bso.stopChannel:
				return
			}
		}
	case <-bso.stopChannel:
		return
	}
	
	// Update statistics
	bso.mu.Lock()
	bso.totalBatches++
	bso.totalRequests += int64(len(batch))
	bso.avgBatchSize = float64(bso.totalRequests) / float64(bso.totalBatches)
	bso.mu.Unlock()
}

// batchProcessor processes batches of search requests
func (bso *BatchSearchOptimizer) batchProcessor(workerID int) {
	defer bso.wg.Done()
	
	for {
		select {
		case response := <-bso.responseChannel:
			if response.Error != nil && response.Error.Error() == "batch_marker" {
				// Process individual requests in this batch
				bso.processIndividualRequests(workerID)
			}
		case <-bso.stopChannel:
			return
		}
	}
}

// processIndividualRequests processes individual requests (fallback when batching not beneficial)
func (bso *BatchSearchOptimizer) processIndividualRequests(workerID int) {
	for {
		select {
		case response := <-bso.responseChannel:
			if response.Error != nil && response.Error.Error() == "process_individual" {
				// This would process individual requests
				// For now, we'll just acknowledge
				continue
			}
		case <-time.After(10 * time.Millisecond):
			// No more individual requests in this batch
			return
		case <-bso.stopChannel:
			return
		}
	}
}

// optimizeBatch analyzes a batch of requests and determines optimization strategies
func (bso *BatchSearchOptimizer) optimizeBatch(requests []*BatchSearchRequest) *BatchOptimizationResult {
	result := &BatchOptimizationResult{
		Requests:      requests,
		SharedFilters: make(map[string]interface{}),
		CanBatch:      false,
	}
	
	if len(requests) <= 1 {
		return result
	}
	
	// Analyze requests for common patterns
	commonTimeRange := bso.findCommonTimeRange(requests)
	commonFilters := bso.findCommonFilters(requests)
	
	// Determine if batching is beneficial
	if len(commonFilters) > 0 || commonTimeRange != nil {
		result.CanBatch = true
		result.SharedFilters = commonFilters
		
		if commonTimeRange != nil {
			result.SharedFilters["time_range"] = commonTimeRange
		}
		
		// Build optimized batch query
		result.OptimizedQuery = bso.buildBatchQuery(requests, commonFilters, commonTimeRange)
	}
	
	return result
}

// findCommonTimeRange finds a common time range across requests
func (bso *BatchSearchOptimizer) findCommonTimeRange(requests []*BatchSearchRequest) *BatchTimeRange {
	if len(requests) == 0 {
		return nil
	}
	
	var minStart, maxEnd time.Time
	hasTimeRange := false
	
	for _, req := range requests {
		if req.Request.StartTime != 0 && req.Request.EndTime != 0 {
			if !hasTimeRange {
				minStart = time.Unix(req.Request.StartTime, 0)
				maxEnd = time.Unix(req.Request.EndTime, 0)
				hasTimeRange = true
			} else {
				reqStartTime := time.Unix(req.Request.StartTime, 0)
				if reqStartTime.Before(minStart) {
					minStart = reqStartTime
				}
				reqEndTime := time.Unix(req.Request.EndTime, 0)
				if reqEndTime.After(maxEnd) {
					maxEnd = reqEndTime
				}
			}
		}
	}
	
	if !hasTimeRange {
		return nil
	}
	
	// Check if the combined time range is reasonable
	if maxEnd.Sub(minStart) > 24*time.Hour {
		return nil // Too wide to be beneficial
	}
	
	return &BatchTimeRange{
		Start: minStart,
		End:   maxEnd,
	}
}

// findCommonFilters finds filters that appear in multiple requests
func (bso *BatchSearchOptimizer) findCommonFilters(requests []*BatchSearchRequest) map[string]interface{} {
	commonFilters := make(map[string]interface{})
	filterCounts := make(map[string]int)
	
	// Count filter occurrences
	for _, req := range requests {
		if req.Request.Method != "" {
			filterCounts["method"]++
		}
		if req.Request.IP != "" {
			filterCounts["ip"]++
		}
		if len(req.Request.Status) > 0 {
			filterCounts["status"]++
		}
		if req.Request.Browser != "" {
			filterCounts["browser"]++
		}
		if req.Request.OS != "" {
			filterCounts["os"]++
		}
	}
	
	// Identify common filters (appear in > 50% of requests)
	threshold := len(requests) / 2
	for filter, count := range filterCounts {
		if count > threshold {
			// Find the most common value for this filter
			commonValue := bso.findMostCommonValue(requests, filter)
			if commonValue != nil {
				commonFilters[filter] = commonValue
			}
		}
	}
	
	return commonFilters
}

// findMostCommonValue finds the most common value for a given filter
func (bso *BatchSearchOptimizer) findMostCommonValue(requests []*BatchSearchRequest, filter string) interface{} {
	valueCounts := make(map[string]int)
	
	for _, req := range requests {
		var value string
		switch filter {
		case "method":
			value = req.Request.Method
		case "ip":
			value = req.Request.IP
		case "browser":
			value = req.Request.Browser
		case "os":
			value = req.Request.OS
		case "status":
			if len(req.Request.Status) > 0 {
				value = fmt.Sprintf("%d", req.Request.Status[0])
			}
		}
		
		if value != "" {
			valueCounts[value]++
		}
	}
	
	// Find most common value
	maxCount := 0
	var mostCommon string
	for value, count := range valueCounts {
		if count > maxCount {
			maxCount = count
			mostCommon = value
		}
	}
	
	if mostCommon != "" {
		return mostCommon
	}
	
	return nil
}

// buildBatchQuery builds an optimized query for a batch of requests
func (bso *BatchSearchOptimizer) buildBatchQuery(requests []*BatchSearchRequest, commonFilters map[string]interface{}, timeRange *BatchTimeRange) query.Query {
	var queries []query.Query
	
	// Add common time range filter
	if timeRange != nil {
		timeQuery := bleve.NewDateRangeQuery(timeRange.Start, timeRange.End)
		timeQuery.SetField("timestamp")
		queries = append(queries, timeQuery)
	}
	
	// Add common filters
	for filter, value := range commonFilters {
		switch filter {
		case "method":
			methodQuery := bleve.NewTermQuery(value.(string))
			methodQuery.SetField("method")
			queries = append(queries, methodQuery)
		case "ip":
			ipQuery := bleve.NewTermQuery(value.(string))
			ipQuery.SetField("ip")
			queries = append(queries, ipQuery)
		case "browser":
			browserQuery := bleve.NewTermQuery(value.(string))
			browserQuery.SetField("browser")
			queries = append(queries, browserQuery)
		case "os":
			osQuery := bleve.NewTermQuery(value.(string))
			osQuery.SetField("os")
			queries = append(queries, osQuery)
		}
	}
	
	// Create individual request queries and combine with OR
	individualQueries := make([]query.Query, 0, len(requests))
	for _, req := range requests {
		// Build query for individual request with remaining filters
		reqQuery := bso.buildIndividualRequestQuery(req.Request, commonFilters)
		if reqQuery != nil {
			individualQueries = append(individualQueries, reqQuery)
		}
	}
	
	// Combine all queries
	if len(queries) == 0 && len(individualQueries) == 0 {
		return bleve.NewMatchAllQuery()
	}
	
	if len(individualQueries) > 0 {
		orQuery := bleve.NewDisjunctionQuery(individualQueries...)
		queries = append(queries, orQuery)
	}
	
	if len(queries) == 1 {
		return queries[0]
	}
	
	return bleve.NewConjunctionQuery(queries...)
}

// buildIndividualRequestQuery builds a query for an individual request excluding common filters
func (bso *BatchSearchOptimizer) buildIndividualRequestQuery(req *QueryRequest, commonFilters map[string]interface{}) query.Query {
	var queries []query.Query
	
	// Add filters that are not common
	if req.Query != "" {
		textQuery := bleve.NewMatchQuery(req.Query)
		textQuery.SetField("raw")
		queries = append(queries, textQuery)
	}
	
	if req.Path != "" {
		pathQuery := bleve.NewTermQuery(req.Path)
		pathQuery.SetField("path")
		queries = append(queries, pathQuery)
	}
	
	// Add non-common filters
	if req.Method != "" && commonFilters["method"] == nil {
		methodQuery := bleve.NewTermQuery(req.Method)
		methodQuery.SetField("method")
		queries = append(queries, methodQuery)
	}
	
	if req.IP != "" && commonFilters["ip"] == nil {
		ipQuery := bleve.NewTermQuery(req.IP)
		ipQuery.SetField("ip")
		queries = append(queries, ipQuery)
	}
	
	if len(queries) == 0 {
		return bleve.NewMatchAllQuery()
	}
	
	if len(queries) == 1 {
		return queries[0]
	}
	
	return bleve.NewConjunctionQuery(queries...)
}

// BatchTimeRange represents a time range for batch optimization
type BatchTimeRange struct {
	Start time.Time
	End   time.Time
}

// GetStatistics returns batch processing statistics
func (bso *BatchSearchOptimizer) GetStatistics() map[string]interface{} {
	bso.mu.RLock()
	defer bso.mu.RUnlock()
	
	return map[string]interface{}{
		"total_batches":    bso.totalBatches,
		"total_requests":   bso.totalRequests,
		"avg_batch_size":   fmt.Sprintf("%.2f", bso.avgBatchSize),
		"batch_size":       bso.batchSize,
		"worker_count":     bso.workerCount,
		"batch_interval":   bso.batchInterval.String(),
		"request_timeout":  bso.requestTimeout.String(),
	}
}

// Close shuts down the batch search optimizer
func (bso *BatchSearchOptimizer) Close() error {
	// Signal all workers to stop
	close(bso.stopChannel)
	
	// Wait for all workers to finish
	bso.wg.Wait()
	
	// Close channels
	close(bso.requestChannel)
	close(bso.responseChannel)
	close(bso.errorChannel)
	
	logger.Infof("Batch search optimizer closed. Final stats: %+v", bso.GetStatistics())
	return nil
}