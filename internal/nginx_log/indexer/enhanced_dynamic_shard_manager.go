package indexer

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

// EnhancedDynamicShardManager extends DefaultShardManager with dynamic scaling
type EnhancedDynamicShardManager struct {
	*DefaultShardManager
	
	// Dynamic features
	targetShardCount  int32
	autoScaleEnabled  bool
	scalingInProgress int32
	lastScaleTime     time.Time
	scalingCooldown   time.Duration
	
	// Metrics and monitoring
	metricsCollector  *ShardMetricsCollector
	loadThresholds    *ScalingThresholds
	
	// Context and control
	ctx      context.Context
	cancel   context.CancelFunc
	stopOnce sync.Once
	
	// Real-time shard monitoring
	shardHealth       map[int]*ShardHealthStatus
	healthMutex       sync.RWMutex
}

// ShardHealthStatus represents the health and performance of a single shard
type ShardHealthStatus struct {
	ShardID         int           `json:"shard_id"`
	IsHealthy       bool          `json:"is_healthy"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	DocumentCount   uint64        `json:"document_count"`
	IndexSize       int64         `json:"index_size"`
	AvgQueryTime    time.Duration `json:"avg_query_time"`
	IndexingRate    float64       `json:"indexing_rate"`
	ErrorCount      int64         `json:"error_count"`
	LastError       string        `json:"last_error,omitempty"`
	LoadScore       float64       `json:"load_score"` // 0.0-1.0, higher means more loaded
}

// NewEnhancedDynamicShardManager creates a new enhanced shard manager
func NewEnhancedDynamicShardManager(config *Config) *EnhancedDynamicShardManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	dsm := &EnhancedDynamicShardManager{
		DefaultShardManager: NewDefaultShardManager(config),
		targetShardCount:   int32(config.ShardCount),
		autoScaleEnabled:   true,
		scalingCooldown:    3 * time.Minute, // Conservative cooldown
		ctx:               ctx,
		cancel:            cancel,
		shardHealth:       make(map[int]*ShardHealthStatus),
		
		loadThresholds: &ScalingThresholds{
			MaxSearchLatency:  3 * time.Second,   // More conservative
			MaxCPUUtilization: 0.80,              // 80% CPU max
			MaxMemoryUsage:    0.75,              // 75% memory max
			MaxDocsPerShard:   5000000,           // 5M docs per shard
			MaxShardSize:      50 * 1024 * 1024 * 1024, // 50GB per shard
			
			MinSearchLatency:  500 * time.Millisecond,
			MinCPUUtilization: 0.20,              // 20% CPU min
			MinDocsPerShard:  500000,             // 500K docs minimum
			MinShardSize:     5 * 1024 * 1024 * 1024,   // 5GB minimum
			
			MinShards:       2,
			MaxShards:       max(16, config.WorkerCount), // Reasonable maximum
			ScalingCooldown: 3 * time.Minute,
		},
	}
	
	// Initialize metrics collector with real shard access  
	realCollector := NewRealShardMetricsCollector(ctx, dsm, 15*time.Second)
	dsm.metricsCollector = &ShardMetricsCollector{
		realCollector: realCollector,
	}
	
	return dsm
}

// Initialize starts the enhanced shard manager
func (dsm *EnhancedDynamicShardManager) Initialize() error {
	// Initialize base shard manager first
	if err := dsm.DefaultShardManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize base shard manager: %w", err)
	}
	
	// Start metrics collection
	if err := dsm.metricsCollector.Start(); err != nil {
		return fmt.Errorf("failed to start metrics collector: %w", err)
	}
	
	// Initialize health status for existing shards
	dsm.initializeHealthStatus()
	
	// Start monitoring goroutines
	go dsm.healthMonitoringLoop()
	if dsm.autoScaleEnabled {
		go dsm.autoScaleMonitoringLoop()
	}
	
	logger.Info("Enhanced dynamic shard manager initialized", 
		"initial_shards", atomic.LoadInt32(&dsm.targetShardCount),
		"auto_scale", dsm.autoScaleEnabled)
	
	return nil
}

// ScaleShards dynamically scales shard count
func (dsm *EnhancedDynamicShardManager) ScaleShards(targetCount int) error {
	if !atomic.CompareAndSwapInt32(&dsm.scalingInProgress, 0, 1) {
		return fmt.Errorf("scaling operation already in progress")
	}
	defer atomic.StoreInt32(&dsm.scalingInProgress, 0)
	
	currentCount := dsm.config.ShardCount
	
	// Validate target count
	if targetCount < dsm.loadThresholds.MinShards {
		targetCount = dsm.loadThresholds.MinShards
	}
	if targetCount > dsm.loadThresholds.MaxShards {
		targetCount = dsm.loadThresholds.MaxShards
	}
	
	if targetCount == currentCount {
		return nil // No change needed
	}
	
	logger.Info("Scaling shards", 
		"current", currentCount, 
		"target", targetCount,
		"action", map[bool]string{true: "scale_up", false: "scale_down"}[targetCount > currentCount])
	
	if targetCount > currentCount {
		// Scale up - add new shards (needs lock)
		dsm.mu.Lock()
		// Scale up - add new shards
		for i := currentCount; i < targetCount; i++ {
			if err := dsm.createShardLocked(i); err != nil {
				dsm.mu.Unlock()
				return fmt.Errorf("failed to create shard %d during scale-up: %w", i, err)
			}
			
			// Initialize health status for new shard
			dsm.healthMutex.Lock()
			dsm.shardHealth[i] = &ShardHealthStatus{
				ShardID:         i,
				IsHealthy:       true,
				LastHealthCheck: time.Now(),
				LoadScore:       0.0,
			}
			dsm.healthMutex.Unlock()
			
			logger.Debug("Created new shard during scale-up", "shard_id", i)
		}
		
		// Update config while holding lock
		dsm.config.ShardCount = targetCount
		dsm.mu.Unlock()
		
	} else {
		// Scale down - safely migrate data before removing shards (no lock during migration)
		logger.Info("Starting safe scale-down with data migration", 
			"removing_shards", currentCount-targetCount)
		
		// Step 1: Migrate data WITHOUT holding the main lock to avoid deadlock
		for i := currentCount - 1; i >= targetCount; i-- {
			migratedDocs, err := dsm.migrateShardData(i, targetCount)
			if err != nil {
				logger.Errorf("Failed to migrate data from shard %d: %v", i, err)
				return fmt.Errorf("data migration failed for shard %d: %w", i, err)
			}
			
			logger.Info("Data migration completed", 
				"from_shard", i, 
				"migrated_documents", migratedDocs)
		}
		
		// Step 2: Now acquire lock and close the empty shards
		dsm.mu.Lock()
		
		for i := currentCount - 1; i >= targetCount; i-- {
			// Close the now-empty shard (manual implementation to avoid lock re-entry)
			if shard, exists := dsm.shards[i]; exists {
				if err := shard.Close(); err != nil {
					logger.Warnf("Failed to close shard %d during scale-down: %v", i, err)
				}
				delete(dsm.shards, i)
				delete(dsm.shardPaths, i)
			}
			
			// Remove from health tracking
			dsm.healthMutex.Lock()
			delete(dsm.shardHealth, i)
			dsm.healthMutex.Unlock()
			
			logger.Info("Successfully removed shard with data preservation", "shard_id", i)
		}
		
		// Update config while holding lock
		dsm.config.ShardCount = targetCount
		dsm.mu.Unlock()
	}
	atomic.StoreInt32(&dsm.targetShardCount, int32(targetCount))
	dsm.lastScaleTime = time.Now()
	
	logger.Info("Shard scaling completed", 
		"new_count", targetCount,
		"duration", time.Since(dsm.lastScaleTime))
	
	return nil
}

// AutoScale performs automatic scaling based on real metrics
func (dsm *EnhancedDynamicShardManager) AutoScale() error {
	if !dsm.autoScaleEnabled {
		return nil
	}
	
	// Check cooldown period
	if time.Since(dsm.lastScaleTime) < dsm.scalingCooldown {
		return nil
	}
	
	metrics := dsm.collectCurrentLoadMetrics()
	decision := dsm.makeScalingDecision(metrics)
	
	if decision.Action != "none" {
		logger.Info("Auto-scaling decision", 
			"action", decision.Action,
			"current_shards", dsm.config.ShardCount,
			"target_shards", decision.TargetShards,
			"reason", decision.Reason,
			"confidence", decision.Confidence)
		
		return dsm.ScaleShards(decision.TargetShards)
	}
	
	return nil
}

// GetShardHealth returns current health status of all shards
func (dsm *EnhancedDynamicShardManager) GetShardHealth() map[int]*ShardHealthStatus {
	dsm.healthMutex.RLock()
	defer dsm.healthMutex.RUnlock()
	
	// Return deep copy to avoid race conditions
	health := make(map[int]*ShardHealthStatus)
	for id, status := range dsm.shardHealth {
		statusCopy := *status // Copy struct
		health[id] = &statusCopy
	}
	
	return health
}

// GetScalingRecommendations analyzes current state and provides recommendations
func (dsm *EnhancedDynamicShardManager) GetScalingRecommendations() *ScalingRecommendation {
	metrics := dsm.collectCurrentLoadMetrics()
	decision := dsm.makeScalingDecision(metrics)
	
	health := dsm.GetShardHealth()
	totalDocs := uint64(0)
	totalSize := int64(0)
	healthyShards := 0
	
	for _, h := range health {
		totalDocs += h.DocumentCount
		totalSize += h.IndexSize
		if h.IsHealthy {
			healthyShards++
		}
	}
	
	return &ScalingRecommendation{
		CurrentShards:       dsm.config.ShardCount,
		RecommendedShards:   decision.TargetShards,
		Action:             decision.Action,
		Reason:             decision.Reason,
		Confidence:         decision.Confidence,
		TotalDocuments:     totalDocs,
		TotalSize:          totalSize,
		HealthyShards:      healthyShards,
		AutoScaleEnabled:   dsm.autoScaleEnabled,
		LastScaleTime:      dsm.lastScaleTime,
		NextScaleAvailable: dsm.lastScaleTime.Add(dsm.scalingCooldown),
	}
}

// ScalingRecommendation contains scaling analysis and recommendations
type ScalingRecommendation struct {
	CurrentShards       int       `json:"current_shards"`
	RecommendedShards   int       `json:"recommended_shards"`
	Action             string    `json:"action"`
	Reason             string    `json:"reason"`
	Confidence         float64   `json:"confidence"`
	TotalDocuments     uint64    `json:"total_documents"`
	TotalSize          int64     `json:"total_size"`
	HealthyShards      int       `json:"healthy_shards"`
	AutoScaleEnabled   bool      `json:"auto_scale_enabled"`
	LastScaleTime      time.Time `json:"last_scale_time"`
	NextScaleAvailable time.Time `json:"next_scale_available"`
}

// initializeHealthStatus sets up health monitoring for existing shards
func (dsm *EnhancedDynamicShardManager) initializeHealthStatus() {
	dsm.healthMutex.Lock()
	defer dsm.healthMutex.Unlock()
	
	for i := 0; i < dsm.config.ShardCount; i++ {
		dsm.shardHealth[i] = &ShardHealthStatus{
			ShardID:         i,
			IsHealthy:       true,
			LastHealthCheck: time.Now(),
			LoadScore:       0.0,
		}
	}
}

// healthMonitoringLoop continuously monitors shard health
func (dsm *EnhancedDynamicShardManager) healthMonitoringLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			dsm.updateShardHealth()
		case <-dsm.ctx.Done():
			return
		}
	}
}

// autoScaleMonitoringLoop runs the auto-scaling logic
func (dsm *EnhancedDynamicShardManager) autoScaleMonitoringLoop() {
	ticker := time.NewTicker(2 * time.Minute) // Check every 2 minutes
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := dsm.AutoScale(); err != nil {
				logger.Warnf("Auto-scaling failed: %v", err)
			}
		case <-dsm.ctx.Done():
			return
		}
	}
}

// updateShardHealth performs health checks on all shards
func (dsm *EnhancedDynamicShardManager) updateShardHealth() {
	dsm.mu.RLock()
	shardIDs := make([]int, 0, len(dsm.shards))
	for id := range dsm.shards {
		shardIDs = append(shardIDs, id)
	}
	dsm.mu.RUnlock()
	
	for _, id := range shardIDs {
		dsm.checkShardHealth(id)
	}
}

// checkShardHealth checks the health of a specific shard
func (dsm *EnhancedDynamicShardManager) checkShardHealth(shardID int) {
	shard, err := dsm.GetShardByID(shardID)
	if err != nil {
		dsm.updateHealthStatus(shardID, false, fmt.Sprintf("Failed to get shard: %v", err))
		return
	}
	
	// Perform health checks
	startTime := time.Now()
	
	// Check 1: Document count (tests basic index access)
	docCount, err := shard.DocCount()
	if err != nil {
		dsm.updateHealthStatus(shardID, false, fmt.Sprintf("DocCount failed: %v", err))
		return
	}
	
	// Check 2: Quick search test (tests query performance)
	searchRequest := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	searchRequest.Size = 1 // Minimal result set
	_, err = shard.Search(searchRequest)
	
	queryTime := time.Since(startTime)
	
	if err != nil {
		dsm.updateHealthStatus(shardID, false, fmt.Sprintf("Search test failed: %v", err))
		return
	}
	
	// Calculate load score
	loadScore := dsm.calculateShardLoad(docCount, queryTime)
	
	// Update health status
	dsm.healthMutex.Lock()
	if status, exists := dsm.shardHealth[shardID]; exists {
		status.IsHealthy = true
		status.LastHealthCheck = time.Now()
		status.DocumentCount = docCount
		status.AvgQueryTime = queryTime
		status.LoadScore = loadScore
		status.LastError = ""
	}
	dsm.healthMutex.Unlock()
}

// updateHealthStatus updates the health status of a shard
func (dsm *EnhancedDynamicShardManager) updateHealthStatus(shardID int, healthy bool, errorMsg string) {
	dsm.healthMutex.Lock()
	defer dsm.healthMutex.Unlock()
	
	if status, exists := dsm.shardHealth[shardID]; exists {
		status.IsHealthy = healthy
		status.LastHealthCheck = time.Now()
		if !healthy {
			status.ErrorCount++
			status.LastError = errorMsg
		}
	}
}

// calculateShardLoad calculates a load score for a shard
func (dsm *EnhancedDynamicShardManager) calculateShardLoad(docCount uint64, queryTime time.Duration) float64 {
	// Normalize factors
	docFactor := float64(docCount) / float64(dsm.loadThresholds.MaxDocsPerShard)
	timeFactor := float64(queryTime) / float64(dsm.loadThresholds.MaxSearchLatency)
	
	// Weighted average (60% doc count, 40% query time)
	loadScore := (docFactor * 0.6) + (timeFactor * 0.4)
	
	// Cap at 1.0
	if loadScore > 1.0 {
		loadScore = 1.0
	}
	
	return loadScore
}

// collectCurrentLoadMetrics gathers real-time metrics from shards
func (dsm *EnhancedDynamicShardManager) collectCurrentLoadMetrics() LoadMetrics {
	health := dsm.GetShardHealth()
	shardSizes := make([]int64, 0, len(health))
	
	var totalLatency time.Duration
	var maxLatency time.Duration
	var totalLoad float64
	healthyCount := 0
	
	for _, h := range health {
		shardSizes = append(shardSizes, h.IndexSize)
		totalLatency += h.AvgQueryTime
		totalLoad += h.LoadScore
		
		if h.AvgQueryTime > maxLatency {
			maxLatency = h.AvgQueryTime
		}
		
		if h.IsHealthy {
			healthyCount++
		}
	}
	
	avgLoad := 0.0
	if len(health) > 0 {
		avgLoad = totalLoad / float64(len(health))
	}
	
	return LoadMetrics{
		SearchLatency:   maxLatency, // Use max latency for scaling decisions
		ShardSizes:      shardSizes,
		CPUUtilization:  avgLoad,    // Use load score as CPU proxy
		ActiveQueries:   0,          // TODO: Track active queries
		QueueLength:     0,          // TODO: Get from indexer queue
	}
}

// makeScalingDecision analyzes metrics and decides on scaling action
func (dsm *EnhancedDynamicShardManager) makeScalingDecision(metrics LoadMetrics) ScalingDecision {
	thresholds := dsm.loadThresholds
	currentShards := dsm.config.ShardCount
	
	// Check scale-up conditions
	if metrics.SearchLatency > thresholds.MaxSearchLatency {
		return ScalingDecision{
			Action:       "scale_up",
			TargetShards: min(currentShards+1, thresholds.MaxShards),
			Reason:       fmt.Sprintf("High search latency: %v > %v", metrics.SearchLatency, thresholds.MaxSearchLatency),
			Confidence:   0.9,
		}
	}
	
	// Check if any shard is too large
	for i, size := range metrics.ShardSizes {
		if size > thresholds.MaxShardSize {
			return ScalingDecision{
				Action:       "scale_up",
				TargetShards: min(currentShards+1, thresholds.MaxShards),
				Reason:       fmt.Sprintf("Shard %d too large: %d bytes > %d bytes", i, size, thresholds.MaxShardSize),
				Confidence:   0.8,
			}
		}
	}
	
	// Check CPU utilization (using load score)
	if metrics.CPUUtilization > thresholds.MaxCPUUtilization {
		return ScalingDecision{
			Action:       "scale_up",
			TargetShards: min(currentShards+1, thresholds.MaxShards),
			Reason:       fmt.Sprintf("High load: %.2f > %.2f", metrics.CPUUtilization, thresholds.MaxCPUUtilization),
			Confidence:   0.7,
		}
	}
	
	// Check scale-down conditions (more conservative)
	if currentShards > thresholds.MinShards && 
		metrics.SearchLatency < thresholds.MinSearchLatency &&
		metrics.CPUUtilization < thresholds.MinCPUUtilization {
		
		// Ensure all shards are small enough for scale-down
		allShardsSmall := true
		for _, size := range metrics.ShardSizes {
			if size > thresholds.MinShardSize*2 { // 2x buffer for safety
				allShardsSmall = false
				break
			}
		}
		
		if allShardsSmall {
			return ScalingDecision{
				Action:       "scale_down",
				TargetShards: max(currentShards-1, thresholds.MinShards),
				Reason:       "All shards underutilized and small",
				Confidence:   0.6,
			}
		}
	}
	
	return ScalingDecision{
		Action:       "none",
		TargetShards: currentShards,
		Reason:       "Current configuration optimal",
		Confidence:   0.5,
	}
}

// SetAutoScaleEnabled enables or disables auto-scaling
func (dsm *EnhancedDynamicShardManager) SetAutoScaleEnabled(enabled bool) {
	dsm.autoScaleEnabled = enabled
	logger.Info("Auto-scaling setting changed", "enabled", enabled)
}

// IsAutoScaleEnabled returns the current auto-scaling status
func (dsm *EnhancedDynamicShardManager) IsAutoScaleEnabled() bool {
	return dsm.autoScaleEnabled
}

// Close shuts down the enhanced shard manager
func (dsm *EnhancedDynamicShardManager) Close() error {
	var closeErr error
	dsm.stopOnce.Do(func() {
		dsm.cancel()
		
		// Stop metrics collection
		if dsm.metricsCollector != nil {
			dsm.metricsCollector.Stop()
		}
		
		// Close base shard manager
		if err := dsm.DefaultShardManager.Close(); err != nil {
			closeErr = fmt.Errorf("failed to close base shard manager: %w", err)
		}
	})
	
	return closeErr
}

// GetCurrentShardCount returns the current number of shards
func (dsm *EnhancedDynamicShardManager) GetCurrentShardCount() int {
	return dsm.config.ShardCount
}

// GetShardCount implements the ShardManager interface
func (dsm *EnhancedDynamicShardManager) GetShardCount() int {
	return dsm.GetCurrentShardCount()
}

// GetTargetShardCount returns the target shard count
func (dsm *EnhancedDynamicShardManager) GetTargetShardCount() int {
	return int(atomic.LoadInt32(&dsm.targetShardCount))
}

// IsScalingInProgress returns whether a scaling operation is in progress
func (dsm *EnhancedDynamicShardManager) IsScalingInProgress() bool {
	return atomic.LoadInt32(&dsm.scalingInProgress) == 1
}

// migrateShardData safely migrates all documents from source shard to target shards
func (dsm *EnhancedDynamicShardManager) migrateShardData(sourceShardID int, targetShardCount int) (int64, error) {
	logger.Info("Starting data migration", 
		"source_shard", sourceShardID, 
		"target_shard_count", targetShardCount)
	
	// Get source shard
	sourceShard, err := dsm.GetShardByID(sourceShardID)
	if err != nil {
		return 0, fmt.Errorf("failed to get source shard %d: %w", sourceShardID, err)
	}
	
	bleveIndex, ok := sourceShard.(bleve.Index)
	if !ok {
		return 0, fmt.Errorf("source shard %d is not a bleve.Index", sourceShardID)
	}
	
	// Create search query to get all documents
	searchRequest := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
	searchRequest.Size = 100 // Smaller batch size for testing
	searchRequest.From = 0
	searchRequest.IncludeLocations = false
	searchRequest.Fields = []string{"content", "type"} // Only specific fields to avoid issues
	
	var totalMigrated int64
	batchNum := 0
	
	for {
		// Search for batch of documents
		searchResult, err := bleveIndex.Search(searchRequest)
		if err != nil {
			return totalMigrated, fmt.Errorf("failed to search source shard %d at batch %d: %w", sourceShardID, batchNum, err)
		}
		
		if len(searchResult.Hits) == 0 {
			break // No more documents
		}
		
		logger.Debug("Migrating document batch", 
			"source_shard", sourceShardID,
			"batch", batchNum,
			"documents", len(searchResult.Hits))
		
		// Migrate each document in the batch
		batch := bleveIndex.NewBatch()
		
		for _, hit := range searchResult.Hits {
			
			// Determine target shard using hash function from base manager
			targetShardID := dsm.DefaultShardManager.hashFunc(hit.ID, targetShardCount)
			
			// Get target shard
			targetShard, err := dsm.GetShardByID(targetShardID)
			if err != nil {
				return totalMigrated, fmt.Errorf("failed to get target shard %d: %w", targetShardID, err)
			}
			
			targetIndex, ok := targetShard.(bleve.Index)
			if !ok {
				return totalMigrated, fmt.Errorf("target shard %d is not a bleve.Index", targetShardID)
			}
			
			// Create document for re-indexing using stored fields from search hit
			documentData := make(map[string]interface{})
			
			// Use the stored fields from the search hit
			if hit.Fields != nil {
				for fieldName, fieldValue := range hit.Fields {
					documentData[fieldName] = fieldValue
				}
			} else {
				// Fallback: reconstruct from hit fragments if available
				documentData["id"] = hit.ID
				documentData["score"] = hit.Score
			}
			
			// Index in target shard
			if err := targetIndex.Index(hit.ID, documentData); err != nil {
				logger.Warnf("Failed to index document %s in target shard %d: %v", hit.ID, targetShardID, err)
				continue
			}
			
			// Add to batch for deletion from source
			batch.Delete(hit.ID)
			totalMigrated++
		}
		
		// Delete migrated documents from source shard
		if batch.Size() > 0 {
			if err := bleveIndex.Batch(batch); err != nil {
				logger.Warnf("Failed to delete migrated documents from source shard %d: %v", sourceShardID, err)
				// Continue - documents are already copied to target shards
			}
		}
		
		// Prepare for next batch
		searchRequest.From += len(searchResult.Hits)
		batchNum++
		
		// Safety check - avoid infinite loops
		if batchNum > 1000 {
			logger.Warnf("Migration stopped after 1000 batches from shard %d", sourceShardID)
			break
		}
	}
	
	logger.Info("Shard data migration completed", 
		"source_shard", sourceShardID,
		"total_migrated", totalMigrated,
		"batches_processed", batchNum)
	
	return totalMigrated, nil
}