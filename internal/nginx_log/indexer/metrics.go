package indexer

import (
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMetricsCollector implements basic metrics collection for indexing operations
type DefaultMetricsCollector struct {
	// Counters
	totalOperations   int64
	successOperations int64
	failedOperations  int64
	totalDocuments    int64
	totalBatches      int64

	// Timing
	totalDuration        int64 // nanoseconds
	batchDuration        int64 // nanoseconds
	optimizationCount    int64
	optimizationDuration int64 // nanoseconds

	// Rate calculations
	lastUpdateTime    int64 // unix timestamp
	lastDocumentCount int64
	currentRate       int64 // docs per second (atomic)

	// Detailed metrics
	operationHistory []OperationMetric
	historyMutex     sync.RWMutex
	maxHistorySize   int

	// Performance tracking
	minLatency int64 // nanoseconds
	maxLatency int64 // nanoseconds
	avgLatency int64 // nanoseconds
}

// OperationMetric represents a single operation's metrics
type OperationMetric struct {
	Timestamp time.Time     `json:"timestamp"`
	Documents int           `json:"documents"`
	Duration  time.Duration `json:"duration"`
	Success   bool          `json:"success"`
	Type      string        `json:"type"` // "index", "batch", "optimize"
}

// NewDefaultMetricsCollector creates a new metrics collector
func NewDefaultMetricsCollector() *DefaultMetricsCollector {
	now := time.Now().Unix()
	return &DefaultMetricsCollector{
		lastUpdateTime:   now,
		maxHistorySize:   1000,             // Keep last 1000 operations
		minLatency:       int64(time.Hour), // Start with high value
		operationHistory: make([]OperationMetric, 0, 1000),
	}
}

// RecordIndexOperation records metrics for an indexing operation
func (m *DefaultMetricsCollector) RecordIndexOperation(docs int, duration time.Duration, success bool) {
	atomic.AddInt64(&m.totalOperations, 1)
	atomic.AddInt64(&m.totalDocuments, int64(docs))
	atomic.AddInt64(&m.totalDuration, int64(duration))

	if success {
		atomic.AddInt64(&m.successOperations, 1)
	} else {
		atomic.AddInt64(&m.failedOperations, 1)
	}

	// Update latency tracking
	durationNs := int64(duration)

	// Update min latency
	for {
		current := atomic.LoadInt64(&m.minLatency)
		if durationNs >= current || atomic.CompareAndSwapInt64(&m.minLatency, current, durationNs) {
			break
		}
	}

	// Update max latency
	for {
		current := atomic.LoadInt64(&m.maxLatency)
		if durationNs <= current || atomic.CompareAndSwapInt64(&m.maxLatency, current, durationNs) {
			break
		}
	}

	// Update average latency (simple running average)
	totalOps := atomic.LoadInt64(&m.totalOperations)
	if totalOps > 0 {
		currentAvg := atomic.LoadInt64(&m.avgLatency)
		newAvg := (currentAvg*(totalOps-1) + durationNs) / totalOps
		atomic.StoreInt64(&m.avgLatency, newAvg)
	}

	// Update rate calculation
	m.updateRate(docs)

	// Record in history
	m.addToHistory(OperationMetric{
		Timestamp: time.Now(),
		Documents: docs,
		Duration:  duration,
		Success:   success,
		Type:      "index",
	})
}

// RecordBatchOperation records metrics for batch operations
func (m *DefaultMetricsCollector) RecordBatchOperation(batchSize int, duration time.Duration) {
	atomic.AddInt64(&m.totalBatches, 1)
	atomic.AddInt64(&m.batchDuration, int64(duration))

	m.addToHistory(OperationMetric{
		Timestamp: time.Now(),
		Documents: batchSize,
		Duration:  duration,
		Success:   true, // Batch operations are typically successful if they complete
		Type:      "batch",
	})
}

// RecordOptimization records metrics for optimization operations
func (m *DefaultMetricsCollector) RecordOptimization(duration time.Duration, success bool) {
	atomic.AddInt64(&m.optimizationCount, 1)
	atomic.AddInt64(&m.optimizationDuration, int64(duration))

	m.addToHistory(OperationMetric{
		Timestamp: time.Now(),
		Documents: 0, // Optimization doesn't process new documents
		Duration:  duration,
		Success:   success,
		Type:      "optimize",
	})
}

// GetMetrics returns current metrics as a structured type
func (m *DefaultMetricsCollector) GetMetrics() *Metrics {
	totalOps := atomic.LoadInt64(&m.totalOperations)
	successOps := atomic.LoadInt64(&m.successOperations)
	failedOps := atomic.LoadInt64(&m.failedOperations)
	totalDocs := atomic.LoadInt64(&m.totalDocuments)
	totalDuration := atomic.LoadInt64(&m.totalDuration)
	totalBatches := atomic.LoadInt64(&m.totalBatches)
	batchDuration := atomic.LoadInt64(&m.batchDuration)
	optimizationCount := atomic.LoadInt64(&m.optimizationCount)
	optimizationDuration := atomic.LoadInt64(&m.optimizationDuration)
	currentRate := atomic.LoadInt64(&m.currentRate)
	minLatency := atomic.LoadInt64(&m.minLatency)
	maxLatency := atomic.LoadInt64(&m.maxLatency)
	avgLatency := atomic.LoadInt64(&m.avgLatency)

	metrics := &Metrics{
		TotalOperations:   totalOps,
		SuccessOperations: successOps,
		FailedOperations:  failedOps,
		TotalDocuments:    totalDocs,
		TotalBatches:      totalBatches,
		OptimizationCount: optimizationCount,
		IndexingRate:      float64(currentRate), // docs per second
		AverageLatencyMS:  float64(avgLatency) / float64(time.Millisecond),
		MinLatencyMS:      float64(minLatency) / float64(time.Millisecond),
		MaxLatencyMS:      float64(maxLatency) / float64(time.Millisecond),
	}

	// Calculate derived metrics
	if totalOps > 0 {
		metrics.SuccessRate = float64(successOps) / float64(totalOps)

		if totalDuration > 0 {
			totalDurationS := float64(totalDuration) / float64(time.Second)
			metrics.AverageThroughput = float64(totalDocs) / totalDurationS
		}
	}

	if totalBatches > 0 && batchDuration > 0 {
		metrics.AverageBatchTimeMS = float64(batchDuration) / float64(totalBatches) / float64(time.Millisecond)
	}

	if optimizationCount > 0 && optimizationDuration > 0 {
		metrics.AverageOptTimeS = float64(optimizationDuration) / float64(optimizationCount) / float64(time.Second)
	}

	// Reset min latency if it's still at the initial high value
	if minLatency == int64(time.Hour) {
		metrics.MinLatencyMS = 0.0
	}

	return metrics
}

// Reset resets all metrics
func (m *DefaultMetricsCollector) Reset() {
	atomic.StoreInt64(&m.totalOperations, 0)
	atomic.StoreInt64(&m.successOperations, 0)
	atomic.StoreInt64(&m.failedOperations, 0)
	atomic.StoreInt64(&m.totalDocuments, 0)
	atomic.StoreInt64(&m.totalBatches, 0)
	atomic.StoreInt64(&m.totalDuration, 0)
	atomic.StoreInt64(&m.batchDuration, 0)
	atomic.StoreInt64(&m.optimizationCount, 0)
	atomic.StoreInt64(&m.optimizationDuration, 0)
	atomic.StoreInt64(&m.currentRate, 0)
	atomic.StoreInt64(&m.lastUpdateTime, time.Now().Unix())
	atomic.StoreInt64(&m.lastDocumentCount, 0)
	atomic.StoreInt64(&m.minLatency, int64(time.Hour))
	atomic.StoreInt64(&m.maxLatency, 0)
	atomic.StoreInt64(&m.avgLatency, 0)

	m.historyMutex.Lock()
	m.operationHistory = m.operationHistory[:0]
	m.historyMutex.Unlock()
}

// updateRate calculates the current indexing rate
func (m *DefaultMetricsCollector) updateRate(newDocs int) {
	now := time.Now().Unix()
	lastUpdate := atomic.LoadInt64(&m.lastUpdateTime)

	// Update rate every second
	if now > lastUpdate {
		currentDocs := atomic.LoadInt64(&m.totalDocuments)
		lastDocs := atomic.LoadInt64(&m.lastDocumentCount)

		if now > lastUpdate {
			timeDiff := now - lastUpdate
			docDiff := currentDocs - lastDocs

			if timeDiff > 0 {
				rate := docDiff / timeDiff
				atomic.StoreInt64(&m.currentRate, rate)
				atomic.StoreInt64(&m.lastUpdateTime, now)
				atomic.StoreInt64(&m.lastDocumentCount, currentDocs)
			}
		}
	}
}

// addToHistory adds an operation to the history buffer
func (m *DefaultMetricsCollector) addToHistory(metric OperationMetric) {
	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()

	// Add new metric
	m.operationHistory = append(m.operationHistory, metric)

	// Trim history if it exceeds max size
	if len(m.operationHistory) > m.maxHistorySize {
		// Keep the most recent metrics
		copy(m.operationHistory, m.operationHistory[len(m.operationHistory)-m.maxHistorySize:])
		m.operationHistory = m.operationHistory[:m.maxHistorySize]
	}
}

// GetOperationHistory returns the operation history
func (m *DefaultMetricsCollector) GetOperationHistory(limit int) []OperationMetric {
	m.historyMutex.RLock()
	defer m.historyMutex.RUnlock()

	if limit <= 0 || limit > len(m.operationHistory) {
		limit = len(m.operationHistory)
	}

	// Return the most recent operations
	start := len(m.operationHistory) - limit
	if start < 0 {
		start = 0
	}

	result := make([]OperationMetric, limit)
	copy(result, m.operationHistory[start:])

	return result
}

// GetRateHistory returns indexing rate over time
func (m *DefaultMetricsCollector) GetRateHistory(duration time.Duration) []RatePoint {
	m.historyMutex.RLock()
	defer m.historyMutex.RUnlock()

	cutoff := time.Now().Add(-duration)
	var points []RatePoint

	// Group operations by time windows (e.g., per minute)
	window := time.Minute
	var currentWindow time.Time
	var currentDocs int

	for _, op := range m.operationHistory {
		if op.Timestamp.Before(cutoff) {
			continue
		}

		windowStart := op.Timestamp.Truncate(window)

		if currentWindow.IsZero() || windowStart.After(currentWindow) {
			if !currentWindow.IsZero() {
				points = append(points, RatePoint{
					Timestamp: currentWindow,
					Rate:      float64(currentDocs) / window.Seconds(),
					Documents: currentDocs,
				})
			}
			currentWindow = windowStart
			currentDocs = 0
		}

		if op.Type == "index" {
			currentDocs += op.Documents
		}
	}

	// Add the last window
	if !currentWindow.IsZero() {
		points = append(points, RatePoint{
			Timestamp: currentWindow,
			Rate:      float64(currentDocs) / window.Seconds(),
			Documents: currentDocs,
		})
	}

	return points
}

// RatePoint represents a point in time for rate calculation
type RatePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Rate      float64   `json:"rate"`      // Documents per second
	Documents int       `json:"documents"` // Total documents in this time window
}

// GetCurrentRate returns the current indexing rate
func (m *DefaultMetricsCollector) GetCurrentRate() float64 {
	return float64(atomic.LoadInt64(&m.currentRate))
}

// SetMaxHistorySize sets the maximum number of operations to keep in history
func (m *DefaultMetricsCollector) SetMaxHistorySize(size int) {
	if size <= 0 {
		return
	}

	m.historyMutex.Lock()
	defer m.historyMutex.Unlock()

	m.maxHistorySize = size

	// Trim existing history if needed
	if len(m.operationHistory) > size {
		start := len(m.operationHistory) - size
		copy(m.operationHistory, m.operationHistory[start:])
		m.operationHistory = m.operationHistory[:size]
	}
}
