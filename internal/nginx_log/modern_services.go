package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
	IndexStatusIndexed    = indexer.IndexStatusIndexed
	IndexStatusIndexing   = indexer.IndexStatusIndexing
	IndexStatusNotIndexed = indexer.IndexStatusNotIndexed
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

// UpdateSearcherShards fetches all shards from the indexer and re-creates the searcher.
// This function is safe for concurrent use.
func UpdateSearcherShards() {
	servicesMutex.Lock() // Use a write lock as we are modifying a global variable
	defer servicesMutex.Unlock()
	updateSearcherShardsLocked()
}

// updateSearcherShardsLocked performs the actual update logic assumes the caller holds the lock.
func updateSearcherShardsLocked() {
	if !servicesInitialized || globalIndexer == nil {
		logger.Warn("Cannot update searcher shards, services not fully initialized.")
		return
	}

	allShards := globalIndexer.GetAllShards()

	// Re-create the searcher instance with the latest shards.
	// This ensures it reads the most up-to-date index state from disk.
	if globalSearcher != nil {
		// Stop the old searcher to release any resources
		if err := globalSearcher.Stop(); err != nil {
			logger.Warnf("Error stopping old searcher: %v", err)
		}
	}

	searcherConfig := searcher.DefaultSearcherConfig() // Or get from existing if config can change
	globalSearcher = searcher.NewDistributedSearcher(searcherConfig, allShards)

	// Also update the analytics service to use the new searcher instance
	globalAnalytics = analytics.NewService(globalSearcher)

	if len(allShards) > 0 {
		logger.Infof("Searcher re-created with %d shards.", len(allShards))
	} else {
		logger.Info("Searcher re-created with no shards.")
	}
}

// DestroyAllIndexes completely removes all indexed data from disk.
func DestroyAllIndexes() error {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized || globalIndexer == nil {
		logger.Warn("Cannot destroy indexes, services not initialized.")
		return fmt.Errorf("services not initialized")
	}

	return globalIndexer.DestroyAllIndexes()
}
