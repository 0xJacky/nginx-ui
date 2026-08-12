package nginx_log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
	cSettings "github.com/uozi-tech/cosy/settings"
)

// Global instances for new services
var (
	globalSearcher         *searcher.Searcher
	globalAnalytics        analytics.Service
	globalIndexer          *indexer.ParallelIndexer
	globalLogFileManager   *indexer.LogFileManager
	servicesInitialized    bool
	servicesInitializing   bool
	servicesMutex          sync.RWMutex
	shutdownCancel         context.CancelFunc
	isShuttingDown         bool
	lastShardUpdateAttempt int64
)

// configLogRegistry holds every log path discovered by scanning the nginx
// configuration. It deliberately lives in the package instead of inside
// LogFileManager: the manager is created by InitializeServices and destroyed by
// StopServices, so a registry owned by the manager would lose every discovered
// path whenever advanced indexing is switched off and on again, and nothing
// would repopulate it until the next periodic config scan five minutes later.
// A rebuild started in that window finds no log groups at all.
var (
	configLogRegistry      = make(map[string]*NginxLogCache)
	configLogRegistryMutex sync.RWMutex
)

// InitializeServices initializes the new modular services
func InitializeServices(ctx context.Context) {
	servicesMutex.Lock()

	// Check if indexing is enabled
	if !settings.NginxLogSettings.IndexingEnabled {
		servicesMutex.Unlock()
		logger.Info("Indexing is disabled, skipping nginx_log services initialization")
		return
	}

	if servicesInitialized {
		servicesMutex.Unlock()
		logger.Info("Modern nginx log services already initialized, skipping")
		return
	}

	if servicesInitializing {
		servicesMutex.Unlock()
		logger.Info("Modern nginx log services are initializing, skipping duplicate request")
		return
	}

	servicesInitializing = true
	servicesMutex.Unlock()

	logger.Info("Initializing modern nginx log services...")

	// Create a cancellable context for services
	serviceCtx, cancel := context.WithCancel(ctx)

	searcherInstance, analyticsInstance, indexerInstance, logFileManagerInstance, err := initializeWithDefaults(serviceCtx)
	if err != nil {
		cancel()
		servicesMutex.Lock()
		servicesInitializing = false
		servicesMutex.Unlock()
		logger.Errorf("Failed to initialize modern services: %v", err)
		return
	}

	servicesMutex.Lock()
	globalSearcher = searcherInstance
	globalAnalytics = analyticsInstance
	globalIndexer = indexerInstance
	globalLogFileManager = logFileManagerInstance
	shutdownCancel = cancel
	servicesInitialized = true
	servicesInitializing = false
	servicesMutex.Unlock()

	// Seed the brand-new manager with the log paths already discovered from the
	// nginx configuration. The manager is created empty on every start, and the
	// config scanner only refills it on a config change or on its five-minute
	// periodic sweep, so without this a rebuild triggered right after advanced
	// indexing is (re-)enabled would see zero log groups.
	if seeded := seedLogFileManager(logFileManagerInstance); seeded > 0 {
		logger.Infof("Seeded %d nginx log path(s) from the configuration scan into the log file manager", seeded)
	} else {
		logger.Info("No nginx log path discovered from the configuration yet; " +
			"the config scan registers them as it completes")
	}

	logger.Info("Modern nginx log services initialization completed")

	// Load existing shards after services are visible without blocking readers during initialization.
	UpdateSearcherShards()

	// Initialize task scheduler after services are ready
	go InitTaskScheduler(serviceCtx)

	// Monitor context for shutdown
	go func() {
		logger.Info("Started nginx_log shutdown monitor goroutine")
		<-serviceCtx.Done()
		logger.Info("Context cancelled, initiating shutdown...")

		// Use the same shutdown logic as manual stop
		StopServices()

		logger.Info("Nginx_log shutdown monitor goroutine completed")
	}()
}

// initializeWithDefaults creates services with default configuration.
func initializeWithDefaults(ctx context.Context) (*searcher.Searcher, analytics.Service, *indexer.ParallelIndexer, *indexer.LogFileManager, error) {
	logger.Info("Initializing services with default configuration")

	// Initialize global log parser singleton before starting indexer/searcher
	indexer.InitLogParser()

	// Create empty searcher (will be populated when indexes are available)
	searcherConfig := searcher.DefaultSearcherConfig()
	searcherInstance := searcher.NewSearcher(searcherConfig, []bleve.Index{})

	// Initialize analytics with empty searcher
	analyticsInstance := analytics.NewService(searcherInstance)

	// Initialize parallel indexer with shard manager
	indexerConfig := indexer.DefaultIndexerConfig()
	// Use config directory for index path
	indexerConfig.IndexPath = getConfigDirIndexPath()
	indexStorageNeedsReset, err := indexer.PrepareIndexStorage(indexerConfig.IndexPath)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to prepare nginx log index storage: %w", err)
	}
	shardManager := indexer.NewGroupedShardManager(indexerConfig)
	indexerInstance := indexer.NewParallelIndexer(indexerConfig, shardManager)

	// Start the indexer
	if err := indexerInstance.Start(ctx); err != nil {
		logger.Errorf("Failed to start parallel indexer: %v", err)
		return nil, nil, nil, nil, fmt.Errorf("failed to start parallel indexer: %w", err)
	}

	// Initialize log file manager
	logFileManagerInstance := indexer.NewLogFileManager()
	// Inject indexer for precise doc counting before persisting
	logFileManagerInstance.SetIndexer(indexerInstance)
	if indexStorageNeedsReset {
		if err := logFileManagerInstance.DeleteAllIndexMetadata(); err != nil {
			_ = indexerInstance.Stop()
			return nil, nil, nil, nil, fmt.Errorf("failed to reset incompatible nginx log index metadata: %w", err)
		}
		if err := indexer.CommitIndexStorageVersion(indexerConfig.IndexPath); err != nil {
			_ = indexerInstance.Stop()
			return nil, nil, nil, nil, fmt.Errorf("failed to commit nginx log index storage migration: %w", err)
		}
		logger.Warn("Reset incompatible nginx log indexes; a clean rebuild will be scheduled")
	}

	return searcherInstance, analyticsInstance, indexerInstance, logFileManagerInstance, nil
}

// getConfigDirIndexPath returns the index path relative to the config file directory
func getConfigDirIndexPath() string {
	// Use custom path if configured
	if settings.NginxLogSettings.IndexPath != "" {
		indexPath := settings.NginxLogSettings.IndexPath
		// Ensure the directory exists
		if err := os.MkdirAll(indexPath, 0755); err != nil {
			logger.Warnf("Failed to create custom index directory at %s: %v, using default", indexPath, err)
		} else {
			logger.Infof("Using custom index path: %s", indexPath)
			return indexPath
		}
	}

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

// GetSearcher returns the global searcher instance
func GetSearcher() *searcher.Searcher {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
		logger.Warn("Modern services not initialized, returning nil")
		return nil
	}

	if globalSearcher == nil {
		logger.Warn("GetSearcher: globalSearcher is nil even though services are initialized")
		return nil
	}

	// Check searcher health status
	isHealthy := globalSearcher.IsHealthy()
	isRunning := globalSearcher.IsRunning()
	logger.Debugf("GetSearcher: returning searcher, isHealthy: %v, isRunning: %v", isHealthy, isRunning)

	// Auto-heal: if the searcher is running but unhealthy (likely zero shards),
	// and the indexer is initialized, trigger an async shard swap (throttled).
	if !isHealthy && isRunning && globalIndexer != nil {
		now := time.Now().UnixNano()
		prev := atomic.LoadInt64(&lastShardUpdateAttempt)
		if now-prev > int64(5*time.Second) {
			if atomic.CompareAndSwapInt64(&lastShardUpdateAttempt, prev, now) {
				logger.Debugf("GetSearcher: unhealthy detected, scheduling UpdateSearcherShards()")
				go UpdateSearcherShards()
			}
		}
	}

	return globalSearcher
}

// GetAnalytics returns the global analytics service instance
func GetAnalytics() analytics.Service {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
		logger.Warn("Modern services not initialized, returning nil")
		return nil
	}

	return globalAnalytics
}

// GetIndexer returns the global indexer instance
func GetIndexer() *indexer.ParallelIndexer {
	servicesMutex.RLock()
	defer servicesMutex.RUnlock()

	if !servicesInitialized {
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
	// Record the path in the durable registry *before* looking the manager up.
	// InitializeServices publishes the manager and only then seeds it from the
	// registry, so this ordering guarantees a path discovered concurrently with
	// a service start is picked up by one of the two writes and can never be
	// dropped between them.
	configLogRegistryMutex.Lock()
	configLogRegistry[path] = &NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
	configLogRegistryMutex.Unlock()

	if manager := GetLogFileManager(); manager != nil {
		manager.AddLogPath(path, logType, name, configFile)
	}
}

// RemoveLogPathsFromConfig removes all log paths associated with a specific config file
func RemoveLogPathsFromConfig(configFile string) {
	configLogRegistryMutex.Lock()
	for p, entry := range configLogRegistry {
		if entry.ConfigFile == configFile {
			delete(configLogRegistry, p)
		}
	}
	configLogRegistryMutex.Unlock()

	if manager := GetLogFileManager(); manager != nil {
		manager.RemoveLogPathsFromConfig(configFile)
	}
}

// GetAllLogPaths returns all cached log paths, optionally filtered
func GetAllLogPaths(filters ...func(*NginxLogCache) bool) []*NginxLogCache {
	if manager := GetLogFileManager(); manager != nil {
		return manager.GetAllLogPaths(filters...)
	}

	// Fallback list
	configLogRegistryMutex.RLock()
	defer configLogRegistryMutex.RUnlock()

	var logs []*NginxLogCache
	for _, entry := range configLogRegistry {
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
	configLogRegistryMutex.RLock()
	defer configLogRegistryMutex.RUnlock()

	result := make([]*NginxLogWithIndex, 0, len(configLogRegistry))
	for _, c := range configLogRegistry {
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
	configLogRegistryMutex.RLock()
	defer configLogRegistryMutex.RUnlock()

	grouped := make(map[string]*NginxLogWithIndex)
	for _, c := range configLogRegistry {
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

// getBaseLogNameBasic derives the base log file for a rotated file name.
// It delegates to the canonical implementation so fallback-mode grouping
// matches the MainLogPath persisted by the indexer.
func getBaseLogNameBasic(filePath string) string {
	return utils.MainLogPathFromFile(filePath)
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

	// An empty shard set is expected before the first indexing task creates a
	// group. Keep the current searcher state until a usable replacement exists.
	if len(newShards) == 0 {
		currentShardCount := 0
		if globalSearcher != nil {
			currentShardCount = len(globalSearcher.GetShards())
		}
		logger.Debugf("No index shards available yet; keeping %d current searcher shards", currentShardCount)
		return
	}

	// If no searcher exists yet, create the initial one (first time setup)
	if globalSearcher == nil {
		logger.Info("Creating initial searcher with IndexAlias")
		searcherConfig := searcher.DefaultSearcherConfig()
		globalSearcher = searcher.NewSearcher(searcherConfig, newShards)

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
	if globalSearcher != nil {
		ds := globalSearcher
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

		// Note: We do NOT recreate the analytics service here since the searcher interface
		// remains the same. The analytics service rebuilds its cardinality counter lazily
		// when it detects the searcher's shard set has changed.

	} else {
		logger.Warn("globalSearcher is not a Searcher, cannot perform hot-swap")
	}
}

// StopServices stops all running modern services
func StopServices() {
	// The scheduler registration is cleared outside the services lock: task
	// recovery running inside the scheduler lock takes the services lock, so
	// grabbing them in the opposite order here would deadlock.
	defer resetTaskSchedulerState()

	stopServicesLocked()
}

// stopServicesLocked tears down the services while holding the services lock.
func stopServicesLocked() {
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

	// Release the parser singleton along with the GeoIP handle and the two
	// 10,000-entry caches it owns. It lives in a package global, so without this
	// it stays reachable - and resident - for the whole life of a process that
	// has already handed its listeners over to a new binary.
	indexer.ReleaseLogParser()

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

// SyncDiscoveredLogPaths copies every log path already discovered from the
// nginx configuration into the running LogFileManager. It is idempotent and
// never clears the registry, so it can be called after every service start.
// Returns the number of paths handed to the manager.
func SyncDiscoveredLogPaths() int {
	manager := GetLogFileManager()
	if manager == nil {
		logger.Warn("Cannot sync discovered nginx log paths: LogFileManager not initialized")
		return 0
	}

	return seedLogFileManager(manager)
}

// seedLogFileManager copies the registry into the given manager. It takes the
// manager as an argument so InitializeServices can seed the instance it just
// created without going through the global getter.
func seedLogFileManager(manager *indexer.LogFileManager) int {
	configLogRegistryMutex.RLock()
	entries := make([]NginxLogCache, 0, len(configLogRegistry))
	for _, entry := range configLogRegistry {
		entries = append(entries, *entry)
	}
	configLogRegistryMutex.RUnlock()

	for _, entry := range entries {
		manager.AddLogPath(entry.Path, entry.Type, entry.Name, entry.ConfigFile)
	}

	return len(entries)
}
