package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// Global instances for new services
var (
	globalSearcher       searcher.Searcher
	globalAnalytics      analytics.Service
	globalIndexer        *indexer.ParallelIndexer
	globalLogFileManager *indexer.LogFileManager
	servicesInitialized  bool
	servicesMutex        sync.RWMutex
)

// InitializeModernServices initializes the new modular services
func InitializeModernServices(ctx context.Context) {
	servicesMutex.Lock()
	defer servicesMutex.Unlock()

	if servicesInitialized {
		logger.Info("Modern nginx log services already initialized, skipping")
		return
	}

	logger.Info("Initializing modern nginx log services...")

	// Initialize with default configuration directly
	if err := initializeWithDefaults(ctx); err != nil {
		logger.Errorf("Failed to initialize modern services: %v", err)
		return
	}

	logger.Info("Modern nginx log services initialization completed")

	// Monitor context for shutdown
	go func() {
		logger.Info("Started nginx_log shutdown monitor goroutine")
		<-ctx.Done()
		logger.Info("Shutting down modern nginx log services...")

		servicesMutex.Lock()
		defer servicesMutex.Unlock()

		// Stop services
		if globalIndexer != nil {
			if err := globalIndexer.Stop(); err != nil {
				logger.Errorf("Failed to stop indexer: %v", err)
			}
		}

		if globalAnalytics != nil {
			if err := globalAnalytics.Stop(); err != nil {
				logger.Errorf("Failed to stop analytics service: %v", err)
			}
		}

		// Stop searcher if it exists
		if globalSearcher != nil {
			if err := globalSearcher.Stop(); err != nil {
				logger.Errorf("Failed to stop searcher: %v", err)
			}
		}

		servicesInitialized = false
		logger.Info("Modern nginx log services shut down")
		logger.Info("Nginx_log shutdown monitor goroutine completed")
	}()
}

// initializeWithDefaults creates services with default configuration
func initializeWithDefaults(ctx context.Context) error {
	logger.Info("Initializing services with default configuration")

	// Create empty searcher (will be populated when indexes are available)
	searcherConfig := searcher.DefaultSearcherConfig()
	globalSearcher = searcher.NewDistributedSearcher(searcherConfig, []bleve.Index{})

	// Initialize analytics with empty searcher
	globalAnalytics = analytics.NewService(globalSearcher)

	// Initialize parallel indexer with shard manager
	indexerConfig := indexer.DefaultIndexerConfig()
	// Use config directory for index path
	indexerConfig.IndexPath = getConfigDirIndexPath()
	shardManager := indexer.NewDefaultShardManager(indexerConfig)
	globalIndexer = indexer.NewParallelIndexer(indexerConfig, shardManager)

	// Start the indexer
	if err := globalIndexer.Start(ctx); err != nil {
		logger.Errorf("Failed to start parallel indexer: %v", err)
		return fmt.Errorf("failed to start parallel indexer: %w", err)
	}

	// Initialize log file manager
	globalLogFileManager = indexer.NewLogFileManager()

	servicesInitialized = true

	// After all services are initialized, update the searcher with any existing shards.
	// This is crucial for loading the index state on application startup.
	// We call the 'locked' version because we already hold the mutex here.
	updateSearcherShardsLocked()

	return nil
}

// getConfigDirIndexPath returns the index path relative to the config file directory
func getConfigDirIndexPath() string {
	// Get the config file path from cosy settings
	if cSettings.ConfPath != "" {
		configDir := filepath.Dir(cSettings.ConfPath)
		indexPath := filepath.Join(configDir, "log-index")

		// Ensure the directory exists
		if err := os.MkdirAll(indexPath, 0755); err != nil {
			logger.Warnf("Failed to create index directory at %s: %v, using default", indexPath, err)
			return "./log-index"
		}

		return indexPath
	}

	// Fallback to default relative path
	logger.Warn("Config file path not available, using default index path")
	return "./log-index"
}

// GetModernSearcher returns the global searcher instance
func GetModernSearcher() searcher.Searcher {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
		logger.Warn("Modern services not initialized, returning nil")
		return nil
	}

	if globalSearcher == nil {
		logger.Warn("GetModernSearcher: globalSearcher is nil even though services are initialized")
		return nil
	}

	// Check searcher health status
	isHealthy := globalSearcher.IsHealthy()
	isRunning := globalSearcher.IsRunning()
	logger.Debugf("GetModernSearcher: returning searcher, isHealthy: %v, isRunning: %v", isHealthy, isRunning)

	return globalSearcher
}

// GetModernAnalytics returns the global analytics service instance
func GetModernAnalytics() analytics.Service {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
		logger.Warn("Modern services not initialized, returning nil")
		return nil
	}

	return globalAnalytics
}

// GetModernIndexer returns the global indexer instance
func GetModernIndexer() *indexer.ParallelIndexer {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
		logger.Warn("Modern services not initialized, returning nil")
		return nil
	}

	return globalIndexer
}

// GetLogFileManager returns the global log file manager instance
func GetLogFileManager() *indexer.LogFileManager {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
		// Only warn during actual operations, not during initialization
		return nil
	}

	if globalLogFileManager == nil {
		logger.Warnf("[nginx_log] GetLogFileManager: globalLogFileManager is nil even though servicesInitialized=true")
		return nil
	}

	return globalLogFileManager
}

// NginxLogCache Type aliases for backward compatibility
type NginxLogCache = indexer.NginxLogCache
type NginxLogWithIndex = indexer.NginxLogWithIndex

// Constants for backward compatibility
const (
	IndexStatusIndexed    = string(indexer.IndexStatusIndexed)
	IndexStatusIndexing   = string(indexer.IndexStatusIndexing)
	IndexStatusNotIndexed = string(indexer.IndexStatusNotIndexed)
)

// Legacy compatibility functions for log cache system

// AddLogPath adds a log path to the log cache with the source config file
func AddLogPath(path, logType, name, configFile string) {
	manager := GetLogFileManager()
	if manager != nil {
		manager.AddLogPath(path, logType, name, configFile)
	} else {
		// Only warn if during initialization (when it might be expected)
		// Skip warning during shutdown or restart phases
	}
}

// RemoveLogPathsFromConfig removes all log paths associated with a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	manager := GetLogFileManager()
	if manager != nil {
		manager.RemoveLogPathsFromConfig(configFile)
	} else {
		// Silently skip if manager not available - this is normal during shutdown/restart
	}
}

// GetAllLogPaths returns all cached log paths, optionally filtered
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogPaths(filters...)
	}
	return []*NginxLogCache{}
}

// GetAllLogsWithIndex returns all cached log paths with their index status
func GetAllLogsWithIndex(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogsWithIndex(filters...)
	}
	return []*NginxLogWithIndex{}
}

// GetAllLogsWithIndexGrouped returns logs grouped by their base name
func GetAllLogsWithIndexGrouped(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogsWithIndexGrouped(filters...)
	}
	return []*NginxLogWithIndex{}
}

// SetIndexingStatus sets the indexing status for a specific file path
func SetIndexingStatus(path string, isIndexing bool) {
	if manager := GetLogFileManager(); manager != nil {
		manager.SetIndexingStatus(path, isIndexing)
	}
}

// GetIndexingFiles returns a list of files currently being indexed
func GetIndexingFiles() []string {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetIndexingFiles()
	}
	return []string{}
}

// UpdateSearcherShards fetches all shards from the indexer and performs zero-downtime shard updates.
// Uses Bleve IndexAlias.Swap() for atomic shard replacement without recreating the searcher.
// This function is safe for concurrent use and maintains service availability during index rebuilds.
func UpdateSearcherShards() {
	// Schedule async update to avoid blocking indexing operations
	logger.Debugf("UpdateSearcherShards: Scheduling async shard update")
	go updateSearcherShardsAsync()
}

// updateSearcherShardsAsync performs the actual shard update asynchronously
func updateSearcherShardsAsync() {
	// Small delay to let indexing operations complete
	time.Sleep(500 * time.Millisecond)

	logger.Debugf("updateSearcherShardsAsync: Attempting to acquire write lock...")
	servicesMutex.Lock()
	logger.Debugf("updateSearcherShardsAsync: Write lock acquired")
	defer func() {
		logger.Debugf("updateSearcherShardsAsync: Releasing write lock...")
		servicesMutex.Unlock()
	}()
	updateSearcherShardsLocked()
}

// updateSearcherShardsLocked performs the actual update logic assumes the caller holds the lock.
// Uses Bleve IndexAlias.Swap() for zero-downtime shard updates following official best practices.
func updateSearcherShardsLocked() {
	if !servicesInitialized || globalIndexer == nil {
		logger.Warn("Cannot update searcher shards, services not fully initialized.")
		return
	}

	// Check if indexer is healthy before getting shards
	if !globalIndexer.IsHealthy() {
		logger.Warn("Cannot update searcher shards, indexer is not healthy")
		return
	}

	newShards := globalIndexer.GetAllShards()
	logger.Infof("Retrieved %d new shards from indexer for hot-swap update", len(newShards))

	// If no searcher exists yet, create the initial one (first time setup)
	if globalSearcher == nil {
		logger.Info("Creating initial searcher with IndexAlias")
		searcherConfig := searcher.DefaultSearcherConfig()
		globalSearcher = searcher.NewDistributedSearcher(searcherConfig, newShards)

		if globalSearcher == nil {
			logger.Error("Failed to create initial searcher instance")
			return
		}

		// Create analytics service with the initial searcher
		globalAnalytics = analytics.NewService(globalSearcher)

		isHealthy := globalSearcher.IsHealthy()
		isRunning := globalSearcher.IsRunning()
		logger.Infof("Initial searcher created successfully, isHealthy: %v, isRunning: %v", isHealthy, isRunning)
		return
	}

	// For subsequent updates, use hot-swap through IndexAlias
	// This follows Bleve best practices for zero-downtime index updates
	if ds, ok := globalSearcher.(*searcher.DistributedSearcher); ok {
		oldShards := ds.GetShards()
		logger.Debugf("updateSearcherShardsLocked: About to call SwapShards...")

		// Perform atomic shard swap using IndexAlias
		if err := ds.SwapShards(newShards); err != nil {
			logger.Errorf("Failed to swap shards atomically: %v", err)
			return
		}
		logger.Debugf("updateSearcherShardsLocked: SwapShards completed successfully")

		logger.Infof("Successfully swapped %d old shards with %d new shards using IndexAlias",
			len(oldShards), len(newShards))

		// Verify searcher health after swap
		isHealthy := globalSearcher.IsHealthy()
		isRunning := globalSearcher.IsRunning()
		logger.Infof("Post-swap searcher status: isHealthy: %v, isRunning: %v", isHealthy, isRunning)

		// Note: We do NOT recreate the analytics service here since the searcher interface remains the same
		// The CardinalityCounter will automatically use the new shards through the same IndexAlias

	} else {
		logger.Warn("globalSearcher is not a DistributedSearcher, cannot perform hot-swap")
	}

}

// DestroyAllIndexes completely removes all indexed data from disk.
func DestroyAllIndexes(ctx context.Context) error {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized || globalIndexer == nil {
		logger.Warn("Cannot destroy indexes, services not initialized.")
		return fmt.Errorf("services not initialized")
	}

	return globalIndexer.DestroyAllIndexes(ctx)
}
