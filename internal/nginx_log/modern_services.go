package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// Global instances for new services
var (
	globalSearcher         searcher.Searcher
	globalAnalytics        analytics.Service
	globalIndexer          *indexer.ParallelIndexer
	globalLogFileManager   *indexer.LogFileManager
	servicesInitialized    bool
	servicesMutex          sync.RWMutex
	shutdownCancel         context.CancelFunc
	isShuttingDown         bool
	lastShardUpdateAttempt int64
)

// Fallback storage when AdvancedIndexingEnabled is disabled
var (
	fallbackCache      = make(map[string]*NginxLogCache)
	fallbackCacheMutex sync.RWMutex
)

// InitializeModernServices initializes the new modular services
func InitializeModernServices(ctx context.Context) {
	servicesMutex.Lock()
	defer servicesMutex.Unlock()

	// Check if advanced indexing is enabled
	if !settings.NginxLogSettings.AdvancedIndexingEnabled {
		logger.Info("Advanced indexing is disabled, skipping nginx_log services initialization")
		return
	}

	if servicesInitialized {
		logger.Info("Modern nginx log services already initialized, skipping")
		return
	}

	logger.Info("Initializing modern nginx log services...")

	// Create a cancellable context for services
	serviceCtx, cancel := context.WithCancel(ctx)
	shutdownCancel = cancel

	// Initialize with default configuration directly
	if err := initializeWithDefaults(serviceCtx); err != nil {
		logger.Errorf("Failed to initialize modern services: %v", err)
		return
	}

	logger.Info("Modern nginx log services initialization completed")

	// Monitor context for shutdown
	go func() {
		logger.Info("Started nginx_log shutdown monitor goroutine")
		<-serviceCtx.Done()
		logger.Info("Context cancelled, initiating shutdown...")

		// Use the same shutdown logic as manual stop
		StopModernServices()

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
	shardManager := indexer.NewGroupedShardManager(indexerConfig)
	globalIndexer = indexer.NewParallelIndexer(indexerConfig, shardManager)

	// Start the indexer
	if err := globalIndexer.Start(ctx); err != nil {
		logger.Errorf("Failed to start parallel indexer: %v", err)
		return fmt.Errorf("failed to start parallel indexer: %w", err)
	}

	// Initialize log file manager
	globalLogFileManager = indexer.NewLogFileManager()
	// Inject indexer for precise doc counting before persisting
	globalLogFileManager.SetIndexer(globalIndexer)

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

	// Auto-heal: if the searcher is running but unhealthy (likely zero shards),
	// and the indexer is initialized, trigger an async shard swap (throttled).
	if !isHealthy && isRunning && globalIndexer != nil {
		now := time.Now().UnixNano()
		prev := atomic.LoadInt64(&lastShardUpdateAttempt)
		if now-prev > int64(5*time.Second) {
			if atomic.CompareAndSwapInt64(&lastShardUpdateAttempt, prev, now) {
				logger.Debugf("GetModernSearcher: unhealthy detected, scheduling UpdateSearcherShards()")
				go UpdateSearcherShards()
			}
		}
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
	IndexStatusIndexed    = string(indexer.IndexStatusIndexed)
	IndexStatusIndexing   = string(indexer.IndexStatusIndexing)
	IndexStatusNotIndexed = string(indexer.IndexStatusNotIndexed)
)

// Legacy compatibility functions for log cache system

// AddLogPath adds a log path to the log cache with the source config file
func AddLogPath(path, logType, name, configFile string) {
	if manager := GetLogFileManager(); manager != nil {
		manager.AddLogPath(path, logType, name, configFile)
		return
	}

	// Fallback storage
	fallbackCacheMutex.Lock()
	fallbackCache[path] = &NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
	fallbackCacheMutex.Unlock()
}

// RemoveLogPathsFromConfig removes all log paths associated with a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	if manager := GetLogFileManager(); manager != nil {
		manager.RemoveLogPathsFromConfig(configFile)
		return
	}

	// Fallback removal
	fallbackCacheMutex.Lock()
	for p, entry := range fallbackCache {
		if entry.ConfigFile == configFile {
			delete(fallbackCache, p)
		}
	}
	fallbackCacheMutex.Unlock()
}

// GetAllLogPaths returns all cached log paths, optionally filtered
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogPaths(filters...)
	}

	// Fallback list
	fallbackCacheMutex.RLock()
	defer fallbackCacheMutex.RUnlock()

	var logs []*NginxLogCache
	for _, entry := range fallbackCache {
		include := true
		for _, f := range filters {
			if !f(entry) {
				include = false
				break
			}
		}
		if include {
			// Create a copy to avoid external mutation
			e := *entry
			logs = append(logs, &e)
		}
	}
	return logs
}

// GetAllLogsWithIndex returns all cached log paths with their index status
func GetAllLogsWithIndex(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogsWithIndex(filters...)
	}

	// Fallback: produce basic entries without indexing metadata
	fallbackCacheMutex.RLock()
	defer fallbackCacheMutex.RUnlock()

	result := make([]*NginxLogWithIndex, 0, len(fallbackCache))
	for _, c := range fallbackCache {
		lw := &NginxLogWithIndex{
			Path:        c.Path,
			Type:        c.Type,
			Name:        c.Name,
			ConfigFile:  c.ConfigFile,
			IndexStatus: IndexStatusNotIndexed,
		}

		include := true
		for _, f := range filters {
			if !f(lw) {
				include = false
				break
			}
		}
		if include {
			result = append(result, lw)
		}
	}
	return result
}

// GetAllLogsWithIndexGrouped returns logs grouped by their base name
func GetAllLogsWithIndexGrouped(filters ...func(*NginxLogWithIndex) bool) []*NginxLogWithIndex {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogsWithIndexGrouped(filters...)
	}

	// Fallback grouping by base log name (handle simple rotation patterns)
	fallbackCacheMutex.RLock()
	defer fallbackCacheMutex.RUnlock()

	grouped := make(map[string]*NginxLogWithIndex)
	for _, c := range fallbackCache {
		base := getBaseLogNameBasic(c.Path)
		if existing, ok := grouped[base]; ok {
			// Preserve most recent non-indexed default; nothing to aggregate in basic mode
			_ = existing
			continue
		}
		grouped[base] = &NginxLogWithIndex{
			Path:        base,
			Type:        c.Type,
			Name:        filepath.Base(base),
			ConfigFile:  c.ConfigFile,
			IndexStatus: IndexStatusNotIndexed,
		}
	}

	// Build slice and apply filters
	keys := make([]string, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]*NginxLogWithIndex, 0, len(keys))
	for _, k := range keys {
		v := grouped[k]
		include := true
		for _, f := range filters {
			if !f(v) {
				include = false
				break
			}
		}
		if include {
			result = append(result, v)
		}
	}
	return result
}

// --- Fallback helpers ---

// getBaseLogNameBasic attempts to derive the base log file for a rotated file name.
// Mirrors the logic used by the indexer, simplified for basic mode.
func getBaseLogNameBasic(filePath string) string {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)

	// Remove compression extensions
	for _, ext := range []string{".gz", ".bz2", ".xz", ".lz4"} {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Check YYYY.MM.DD at end
	parts := strings.Split(filename, ".")
	if len(parts) >= 4 {
		lastThree := strings.Join(parts[len(parts)-3:], ".")
		if matched, _ := regexp.MatchString(`^\d{4}\.\d{2}\.\d{2}$`, lastThree); matched {
			base := strings.Join(parts[:len(parts)-3], ".")
			return filepath.Join(dir, base)
		}
	}

	// Single-part date suffix (YYYYMMDD / YYYY-MM-DD / YYMMDD)
	if len(parts) >= 2 {
		last := parts[len(parts)-1]
		if isFullDatePatternBasic(last) {
			base := strings.Join(parts[:len(parts)-1], ".")
			return filepath.Join(dir, base)
		}
	}

	// Numbered rotation: access.log.1
	if m := regexp.MustCompile(`^(.+)\.(\d{1,3})$`).FindStringSubmatch(filename); len(m) > 1 {
		base := m[1]
		return filepath.Join(dir, base)
	}

	// Middle-numbered rotation: access.1.log
	if m := regexp.MustCompile(`^(.+)\.(\d{1,3})\.log$`).FindStringSubmatch(filename); len(m) > 1 {
		base := m[1] + ".log"
		return filepath.Join(dir, base)
	}

	// Fallback: return original path
	return filePath
}

func isFullDatePatternBasic(s string) bool {
	patterns := []string{
		`^\d{8}$`,             // YYYYMMDD
		`^\d{4}-\d{2}-\d{2}$`, // YYYY-MM-DD
		`^\d{6}$`,             // YYMMDD
	}
	for _, p := range patterns {
		if matched, _ := regexp.MatchString(p, s); matched {
			return true
		}
	}
	return false
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

// StopModernServices stops all running modern services
func StopModernServices() {
	servicesMutex.Lock()
	defer servicesMutex.Unlock()

	if !servicesInitialized {
		logger.Debug("Modern nginx log services not initialized, nothing to stop")
		return
	}

	if isShuttingDown {
		logger.Debug("Modern nginx log services already shutting down")
		return
	}

	logger.Debug("Stopping modern nginx log services...")
	isShuttingDown = true

	// Cancel the service context to trigger graceful shutdown
	if shutdownCancel != nil {
		shutdownCancel()
		// Wait a bit for graceful shutdown
		time.Sleep(500 * time.Millisecond)
	}

	// Stop all services
	if globalIndexer != nil {
		if err := globalIndexer.Stop(); err != nil {
			logger.Errorf("Failed to stop indexer: %v", err)
		}
		globalIndexer = nil
	}

	if globalAnalytics != nil {
		if err := globalAnalytics.Stop(); err != nil {
			logger.Errorf("Failed to stop analytics service: %v", err)
		}
		globalAnalytics = nil
	}

	if globalSearcher != nil {
		if err := globalSearcher.Stop(); err != nil {
			logger.Errorf("Failed to stop searcher: %v", err)
		}
		globalSearcher = nil
	}

	// Reset state
	globalLogFileManager = nil
	servicesInitialized = false
	shutdownCancel = nil
	isShuttingDown = false

	logger.Debug("Modern nginx log services stopped")
}

// DestroyAllIndexes completely removes all indexed data from disk.
func DestroyAllIndexes(ctx context.Context) error {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized || globalIndexer == nil {
		logger.Debug("Cannot destroy indexes, services not initialized.")
		return fmt.Errorf("services not initialized")
	}

	return globalIndexer.DestroyAllIndexes(ctx)
}
