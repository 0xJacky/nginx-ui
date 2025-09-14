package nginx_log

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/blevesearch/bleve/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Test configuration
	TestRecordsPerFile = 400000 // 40万条记录每个文件
	TestFileCount      = 3      // 3个测试文件
	TestBaseDir        = "./test_integration_logs"
	TestIndexDir       = "./test_integration_index"
)

// IntegrationTestSuite contains all integration test data and services
type IntegrationTestSuite struct {
	ctx             context.Context
	cancel          context.CancelFunc
	tempDir         string
	indexDir        string
	logFiles        []string
	logFilePaths    []string
	indexer         *indexer.ParallelIndexer
	searcher        searcher.Searcher
	analytics       analytics.Service
	logFileManager  *TestLogFileManager
	expectedMetrics map[string]*ExpectedFileMetrics
	cleanup         func()
}

// TestLogFileManager is a simplified log file manager for testing that doesn't require database
type TestLogFileManager struct {
	logCache       map[string]*indexer.NginxLogCache
	cacheMutex     sync.RWMutex
	indexingStatus map[string]bool
	indexMetadata  map[string]*TestIndexMetadata
	metadataMutex  sync.RWMutex
}

// TestIndexMetadata holds index metadata for testing
type TestIndexMetadata struct {
	Path          string
	DocumentCount uint64
	LastIndexed   time.Time
	Duration      time.Duration
	MinTime       *time.Time
	MaxTime       *time.Time
}

// ExpectedFileMetrics stores expected statistics for each log file
type ExpectedFileMetrics struct {
	TotalRecords uint64
	UniqueIPs    uint64
	UniquePaths  uint64
	UniqueAgents uint64
	StatusCodes  map[int]uint64
	Methods      map[string]uint64
	TimeRange    TestTimeRange
}

// TestTimeRange represents the time range of log entries for testing
type TestTimeRange struct {
	StartTime time.Time
	EndTime   time.Time
}

// NewIntegrationTestSuite creates a new integration test suite
func NewIntegrationTestSuite(t *testing.T) *IntegrationTestSuite {
	ctx, cancel := context.WithCancel(context.Background())

	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "nginx_ui_integration_test_*")
	require.NoError(t, err)

	indexDir := filepath.Join(tempDir, "index")
	logsDir := filepath.Join(tempDir, "logs")

	err = os.MkdirAll(indexDir, 0755)
	require.NoError(t, err)

	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	suite := &IntegrationTestSuite{
		ctx:             ctx,
		cancel:          cancel,
		tempDir:         tempDir,
		indexDir:        indexDir,
		expectedMetrics: make(map[string]*ExpectedFileMetrics),
	}

	// Set cleanup function
	suite.cleanup = func() {
		// Stop services
		if suite.indexer != nil {
			suite.indexer.Stop()
		}
		if suite.searcher != nil {
			suite.searcher.Stop()
		}

		// Cancel context
		cancel()

		// Remove temporary directories
		os.RemoveAll(tempDir)
	}

	return suite
}

// GenerateTestData generates the test log files with expected statistics
func (suite *IntegrationTestSuite) GenerateTestData(t *testing.T) {
	t.Logf("Generating %d test files with %d records each", TestFileCount, TestRecordsPerFile)

	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < TestFileCount; i++ {
		filename := fmt.Sprintf("access_%d.log", i+1)
		filepath := filepath.Join(suite.tempDir, "logs", filename)

		metrics := suite.generateSingleLogFile(t, filepath, baseTime.Add(time.Duration(i)*time.Hour))

		suite.logFiles = append(suite.logFiles, filename)
		suite.logFilePaths = append(suite.logFilePaths, filepath)
		suite.expectedMetrics[filepath] = metrics

		t.Logf("Generated %s with %d records", filename, metrics.TotalRecords)
	}

	t.Logf("Test data generation completed. Total files: %d", len(suite.logFiles))
}

// generateSingleLogFile generates a single log file with known statistics
func (suite *IntegrationTestSuite) generateSingleLogFile(t *testing.T, filepath string, baseTime time.Time) *ExpectedFileMetrics {
	file, err := os.Create(filepath)
	require.NoError(t, err)
	defer file.Close()

	metrics := &ExpectedFileMetrics{
		StatusCodes: make(map[int]uint64),
		Methods:     make(map[string]uint64),
		TimeRange: TestTimeRange{
			StartTime: baseTime,
			EndTime:   baseTime.Add(time.Duration(TestRecordsPerFile) * time.Second),
		},
	}

	// Predefined test data for consistent testing
	ips := []string{
		"192.168.1.1", "192.168.1.2", "192.168.1.3", "10.0.0.1", "10.0.0.2",
		"172.16.0.1", "172.16.0.2", "203.0.113.1", "203.0.113.2", "198.51.100.1",
	}

	paths := []string{
		"/", "/api/v1/status", "/api/v1/logs", "/admin", "/login",
		"/dashboard", "/api/v1/config", "/static/css/main.css", "/static/js/app.js", "/favicon.ico",
	}

	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		"PostmanRuntime/7.28.4",
		"Go-http-client/1.1",
	}

	statusCodes := []int{200, 301, 404, 500, 502}
	methods := []string{"GET", "POST", "PUT", "DELETE"}

	// Track unique values
	uniqueIPs := make(map[string]bool)
	uniquePaths := make(map[string]bool)
	uniqueAgents := make(map[string]bool)

	// use global rng defaults; no explicit rand.Seed needed

	for i := 0; i < TestRecordsPerFile; i++ {
		// Generate log entry timestamp
		timestamp := baseTime.Add(time.Duration(i) * time.Second)

		// Select random values
		ip := ips[rand.Intn(len(ips))]
		path := paths[rand.Intn(len(paths))]
		agent := userAgents[rand.Intn(len(userAgents))]
		status := statusCodes[rand.Intn(len(statusCodes))]
		method := methods[rand.Intn(len(methods))]
		size := rand.Intn(10000) + 100 // 100-10100 bytes

		// Track unique values
		uniqueIPs[ip] = true
		uniquePaths[path] = true
		uniqueAgents[agent] = true

		// Update metrics
		metrics.StatusCodes[status]++
		metrics.Methods[method]++

		// Generate nginx log line (Common Log Format)
		logLine := fmt.Sprintf(`%s - - [%s] "%s %s HTTP/1.1" %d %d "-" "%s"`+"\n",
			ip,
			timestamp.Format("02/Jan/2006:15:04:05 -0700"),
			method,
			path,
			status,
			size,
			agent,
		)

		_, err := file.WriteString(logLine)
		require.NoError(t, err)
	}

	// Finalize metrics
	metrics.TotalRecords = TestRecordsPerFile
	metrics.UniqueIPs = uint64(len(uniqueIPs))
	metrics.UniquePaths = uint64(len(uniquePaths))
	metrics.UniqueAgents = uint64(len(uniqueAgents))

	return metrics
}

// InitializeServices initializes all nginx_log services for testing
func (suite *IntegrationTestSuite) InitializeServices(t *testing.T) {
	t.Log("Initializing test services...")

	// Initialize indexer
	indexerConfig := indexer.DefaultIndexerConfig()
	indexerConfig.IndexPath = suite.indexDir
	shardManager := indexer.NewGroupedShardManager(indexerConfig)
	suite.indexer = indexer.NewParallelIndexer(indexerConfig, shardManager)

	err := suite.indexer.Start(suite.ctx)
	require.NoError(t, err)

	// Initialize searcher (empty initially)
	searcherConfig := searcher.DefaultSearcherConfig()
	suite.searcher = searcher.NewDistributedSearcher(searcherConfig, []bleve.Index{})

	// Initialize analytics
	suite.analytics = analytics.NewService(suite.searcher)

	// Initialize log file manager with test-specific behavior
	suite.logFileManager = suite.createTestLogFileManager(t)

	// Register test log files
	for _, logPath := range suite.logFilePaths {
		suite.logFileManager.AddLogPath(logPath, "access", filepath.Base(logPath), "test_config")
	}

	t.Log("Services initialized successfully")
}

// createTestLogFileManager creates a log file manager suitable for testing
func (suite *IntegrationTestSuite) createTestLogFileManager(t *testing.T) *TestLogFileManager {
	return &TestLogFileManager{
		logCache:       make(map[string]*indexer.NginxLogCache),
		indexingStatus: make(map[string]bool),
		indexMetadata:  make(map[string]*TestIndexMetadata),
	}
}

// AddLogPath adds a log path to the test log cache
func (tlm *TestLogFileManager) AddLogPath(path, logType, name, configFile string) {
	tlm.cacheMutex.Lock()
	defer tlm.cacheMutex.Unlock()

	tlm.logCache[path] = &indexer.NginxLogCache{
		Path:       path,
		Type:       logType,
		Name:       name,
		ConfigFile: configFile,
	}
}

// GetAllLogsWithIndexGrouped returns all cached log paths with their index status for testing
func (tlm *TestLogFileManager) GetAllLogsWithIndexGrouped(filters ...func(*indexer.NginxLogWithIndex) bool) []*indexer.NginxLogWithIndex {
	tlm.cacheMutex.RLock()
	defer tlm.cacheMutex.RUnlock()

	tlm.metadataMutex.RLock()
	defer tlm.metadataMutex.RUnlock()

	var logs []*indexer.NginxLogWithIndex

	for _, logEntry := range tlm.logCache {
		logWithIndex := &indexer.NginxLogWithIndex{
			Path:        logEntry.Path,
			Type:        logEntry.Type,
			Name:        logEntry.Name,
			ConfigFile:  logEntry.ConfigFile,
			IndexStatus: "not_indexed",
		}

		// Check if we have index metadata for this path
		if metadata, exists := tlm.indexMetadata[logEntry.Path]; exists {
			logWithIndex.IndexStatus = "indexed"
			logWithIndex.DocumentCount = metadata.DocumentCount
			logWithIndex.LastIndexed = metadata.LastIndexed.Unix()
			logWithIndex.IndexDuration = int64(metadata.Duration.Milliseconds())

			if metadata.MinTime != nil {
				logWithIndex.HasTimeRange = true
				logWithIndex.TimeRangeStart = metadata.MinTime.Unix()
			}

			if metadata.MaxTime != nil {
				logWithIndex.HasTimeRange = true
				logWithIndex.TimeRangeEnd = metadata.MaxTime.Unix()
			}
		}

		// Apply filters
		include := true
		for _, filter := range filters {
			if !filter(logWithIndex) {
				include = false
				break
			}
		}

		if include {
			logs = append(logs, logWithIndex)
		}
	}

	return logs
}

// SaveIndexMetadata saves index metadata for testing
func (tlm *TestLogFileManager) SaveIndexMetadata(path string, docCount uint64, indexTime time.Time, duration time.Duration, minTime, maxTime *time.Time) error {
	tlm.metadataMutex.Lock()
	defer tlm.metadataMutex.Unlock()

	tlm.indexMetadata[path] = &TestIndexMetadata{
		Path:          path,
		DocumentCount: docCount,
		LastIndexed:   indexTime,
		Duration:      duration,
		MinTime:       minTime,
		MaxTime:       maxTime,
	}

	return nil
}

// DeleteIndexMetadataByGroup deletes index metadata for a log group (for testing)
func (tlm *TestLogFileManager) DeleteIndexMetadataByGroup(logGroup string) error {
	tlm.metadataMutex.Lock()
	defer tlm.metadataMutex.Unlock()

	delete(tlm.indexMetadata, logGroup)
	return nil
}

// DeleteAllIndexMetadata deletes all index metadata (for testing)
func (tlm *TestLogFileManager) DeleteAllIndexMetadata() error {
	tlm.metadataMutex.Lock()
	defer tlm.metadataMutex.Unlock()

	tlm.indexMetadata = make(map[string]*TestIndexMetadata)
	return nil
}

// PerformGlobalIndexRebuild performs a complete index rebuild of all files
func (suite *IntegrationTestSuite) PerformGlobalIndexRebuild(t *testing.T) {
	t.Log("Starting global index rebuild...")

	startTime := time.Now()

	// Create progress tracking
	var completedFiles []string
	var mu sync.Mutex

	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 1 * time.Second,
		OnProgress: func(progress indexer.ProgressNotification) {
			t.Logf("Index progress: %s - %.1f%% (Files: %d/%d, Lines: %d/%d)",
				progress.LogGroupPath, progress.Percentage, progress.CompletedFiles,
				progress.TotalFiles, progress.ProcessedLines, progress.EstimatedLines)
		},
		OnCompletion: func(completion indexer.CompletionNotification) {
			mu.Lock()
			completedFiles = append(completedFiles, completion.LogGroupPath)
			mu.Unlock()

			t.Logf("Index completion: %s - Success: %t, Duration: %s, Lines: %d",
				completion.LogGroupPath, completion.Success, completion.Duration, completion.TotalLines)
		},
	}

	// Destroy existing indexes
	err := suite.indexer.DestroyAllIndexes(suite.ctx)
	require.NoError(t, err)

	// Re-initialize indexer
	err = suite.indexer.Start(suite.ctx)
	require.NoError(t, err)

	// Index all log files
	allLogs := suite.logFileManager.GetAllLogsWithIndexGrouped()
	for _, log := range allLogs {
		docsCountMap, minTime, maxTime, err := suite.indexer.IndexLogGroupWithProgress(log.Path, progressConfig)
		require.NoError(t, err, "Failed to index log group: %s", log.Path)

		// Save metadata
		duration := time.Since(startTime)
		var totalDocs uint64
		for _, docCount := range docsCountMap {
			totalDocs += docCount
		}

		err = suite.logFileManager.SaveIndexMetadata(log.Path, totalDocs, startTime, duration, minTime, maxTime)
		require.NoError(t, err)
	}

	// Flush and update searcher
	err = suite.indexer.FlushAll()
	require.NoError(t, err)

	suite.updateSearcher(t)

	totalDuration := time.Since(startTime)
	t.Logf("Global index rebuild completed in %s. Completed files: %v", totalDuration, completedFiles)
}

// PerformSingleFileIndexRebuild rebuilds index for a single file
func (suite *IntegrationTestSuite) PerformSingleFileIndexRebuild(t *testing.T, targetFile string) {
	t.Logf("Starting single file index rebuild for: %s", targetFile)

	startTime := time.Now()

	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 1 * time.Second,
		OnProgress: func(progress indexer.ProgressNotification) {
			t.Logf("Single file index progress: %s - %.1f%%", progress.LogGroupPath, progress.Percentage)
		},
		OnCompletion: func(completion indexer.CompletionNotification) {
			t.Logf("Single file index completion: %s - Success: %t, Lines: %d",
				completion.LogGroupPath, completion.Success, completion.TotalLines)
		},
	}

	// Delete existing index for this log group
	err := suite.indexer.DeleteIndexByLogGroup(targetFile, suite.logFileManager)
	require.NoError(t, err)

	// Clean up database records for this log group
	err = suite.logFileManager.DeleteIndexMetadataByGroup(targetFile)
	require.NoError(t, err)

	// Index the specific file
	docsCountMap, minTime, maxTime, err := suite.indexer.IndexLogGroupWithProgress(targetFile, progressConfig)
	require.NoError(t, err, "Failed to index single file: %s", targetFile)

	// Save metadata
	duration := time.Since(startTime)
	var totalDocs uint64
	for _, docCount := range docsCountMap {
		totalDocs += docCount
	}

	err = suite.logFileManager.SaveIndexMetadata(targetFile, totalDocs, startTime, duration, minTime, maxTime)
	require.NoError(t, err)

	// Flush and update searcher
	err = suite.indexer.FlushAll()
	require.NoError(t, err)

	suite.updateSearcher(t)

	totalDuration := time.Since(startTime)
	t.Logf("Single file index rebuild completed in %s for: %s", totalDuration, targetFile)
}

// updateSearcher updates the searcher with current shards
func (suite *IntegrationTestSuite) updateSearcher(t *testing.T) {
	if !suite.indexer.IsHealthy() {
		t.Fatal("Indexer is not healthy, cannot update searcher")
	}

	newShards := suite.indexer.GetAllShards()
	t.Logf("Updating searcher with %d shards", len(newShards))

	if ds, ok := suite.searcher.(*searcher.DistributedSearcher); ok {
		err := ds.SwapShards(newShards)
		require.NoError(t, err)
		t.Log("Searcher shards updated successfully")
	} else {
		t.Fatal("Searcher is not a DistributedSearcher")
	}
}

// ValidateCardinalityCounter validates the accuracy of cardinality counting
func (suite *IntegrationTestSuite) ValidateCardinalityCounter(t *testing.T, filePath string) {
	t.Logf("Validating CardinalityCounter accuracy for: %s", filePath)

	expected := suite.expectedMetrics[filePath]
	require.NotNil(t, expected, "Expected metrics not found for file: %s", filePath)

	// Test IP cardinality
	suite.testFieldCardinality(t, filePath, "remote_addr", expected.UniqueIPs, "IP addresses")

	// Test path cardinality
	suite.testFieldCardinality(t, filePath, "uri_path", expected.UniquePaths, "URI paths")

	// Test user agent cardinality
	suite.testFieldCardinality(t, filePath, "http_user_agent", expected.UniqueAgents, "User agents")

	t.Logf("CardinalityCounter validation completed for: %s", filePath)
}

// testFieldCardinality tests cardinality counting for a specific field
func (suite *IntegrationTestSuite) testFieldCardinality(t *testing.T, filePath string, field string, expectedCount uint64, fieldName string) {
	if ds, ok := suite.searcher.(*searcher.DistributedSearcher); ok {
		cardinalityCounter := searcher.NewCardinalityCounter(ds.GetShards())

		req := &searcher.CardinalityRequest{
			Field:    field,
			LogPaths: []string{filePath},
		}

		result, err := cardinalityCounter.CountCardinality(suite.ctx, req)
		require.NoError(t, err, "Failed to count cardinality for field: %s", field)

		// Allow for small discrepancies due to indexing behavior
		tolerance := uint64(expectedCount) / 100 // 1% tolerance
		if tolerance < 1 {
			tolerance = 1
		}

		assert.InDelta(t, expectedCount, result.Cardinality, float64(tolerance),
			"Cardinality mismatch for %s in %s: expected %d, got %d",
			fieldName, filePath, expectedCount, result.Cardinality)

		t.Logf("✓ %s cardinality: expected=%d, actual=%d, total_docs=%d",
			fieldName, expectedCount, result.Cardinality, result.TotalDocs)
	} else {
		t.Fatal("Searcher is not a DistributedSearcher")
	}
}

// ValidateAnalyticsData validates the accuracy of analytics statistics
func (suite *IntegrationTestSuite) ValidateAnalyticsData(t *testing.T, filePath string) {
	t.Logf("Validating Analytics data accuracy for: %s", filePath)

	expected := suite.expectedMetrics[filePath]
	require.NotNil(t, expected, "Expected metrics not found for file: %s", filePath)

	// Test dashboard analytics
	dashboardReq := &analytics.DashboardQueryRequest{
		LogPaths:  []string{filePath},
		StartTime: expected.TimeRange.StartTime.Unix(),
		EndTime:   expected.TimeRange.EndTime.Unix(),
	}

	dashboard, err := suite.analytics.GetDashboardAnalytics(suite.ctx, dashboardReq)
	require.NoError(t, err, "Failed to get dashboard data for: %s", filePath)

	// Validate basic metrics
	tolerance := float64(expected.TotalRecords) * 0.01 // 1% tolerance
	assert.InDelta(t, expected.TotalRecords, dashboard.Summary.TotalPV, tolerance,
		"Total requests mismatch for %s", filePath)

	t.Logf("✓ Dashboard validation completed for: %s", filePath)
	t.Logf("  Total requests: expected=%d, actual=%d", expected.TotalRecords, dashboard.Summary.TotalPV)
	t.Logf("  Unique visitors: %d", dashboard.Summary.TotalUV)
	t.Logf("  Average daily PV: %f", dashboard.Summary.AvgDailyPV)
}

// ValidatePaginationFunctionality validates pagination works correctly using searcher
func (suite *IntegrationTestSuite) ValidatePaginationFunctionality(t *testing.T, filePath string) {
	t.Logf("Validating pagination functionality for: %s", filePath)

	expected := suite.expectedMetrics[filePath]
	require.NotNil(t, expected, "Expected metrics not found for file: %s", filePath)

	startTime := expected.TimeRange.StartTime.Unix()
	endTime := expected.TimeRange.EndTime.Unix()

	// Test first page
	searchReq1 := &searcher.SearchRequest{
		Query:     "*",
		LogPaths:  []string{filePath},
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100,
		Offset:    0,
		SortBy:    "timestamp",
		SortOrder: "desc",
	}

	result1, err := suite.searcher.Search(suite.ctx, searchReq1)
	require.NoError(t, err, "Failed to get page 1 for: %s", filePath)
	assert.Equal(t, 100, len(result1.Hits), "First page should have 100 entries")
	assert.Equal(t, expected.TotalRecords, result1.TotalHits, "Total count mismatch")

	// Test second page
	searchReq2 := &searcher.SearchRequest{
		Query:     "*",
		LogPaths:  []string{filePath},
		StartTime: &startTime,
		EndTime:   &endTime,
		Limit:     100,
		Offset:    100,
		SortBy:    "timestamp",
		SortOrder: "desc",
	}

	result2, err := suite.searcher.Search(suite.ctx, searchReq2)
	require.NoError(t, err, "Failed to get page 2 for: %s", filePath)
	assert.Equal(t, 100, len(result2.Hits), "Second page should have 100 entries")
	assert.Equal(t, expected.TotalRecords, result2.TotalHits, "Total count should be consistent")

	// Ensure different pages return different entries
	if len(result1.Hits) > 0 && len(result2.Hits) > 0 {
		firstPageFirstEntry := result1.Hits[0].ID
		secondPageFirstEntry := result2.Hits[0].ID
		assert.NotEqual(t, firstPageFirstEntry, secondPageFirstEntry,
			"Different pages should return different entries")
	}

	t.Logf("✓ Pagination validation completed for: %s", filePath)
	t.Logf("  Page 1 entries: %d", len(result1.Hits))
	t.Logf("  Page 2 entries: %d", len(result2.Hits))
	t.Logf("  Total entries: %d", result1.TotalHits)
}

// TestNginxLogIntegration is the main integration test function
func TestNginxLogIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	suite := NewIntegrationTestSuite(t)
	defer suite.cleanup()

	t.Log("=== Starting Nginx Log Integration Test ===")

	// Step 1: Generate test data
	suite.GenerateTestData(t)

	// Step 2: Initialize services
	suite.InitializeServices(t)

	// Step 3: Perform global index rebuild and validate during indexing
	t.Log("\n=== Testing Global Index Rebuild ===")
	suite.PerformGlobalIndexRebuild(t)

	// Step 4: Validate all files after global rebuild
	for _, filePath := range suite.logFilePaths {
		t.Logf("\n--- Validating file after global rebuild: %s ---", filepath.Base(filePath))
		suite.ValidateCardinalityCounter(t, filePath)
		suite.ValidateAnalyticsData(t, filePath)
		suite.ValidatePaginationFunctionality(t, filePath)
	}

	// Step 5: Test single file rebuild
	t.Log("\n=== Testing Single File Index Rebuild ===")
	targetFile := suite.logFilePaths[1] // Rebuild second file
	suite.PerformSingleFileIndexRebuild(t, targetFile)

	// Step 6: Validate all files after single file rebuild
	for _, filePath := range suite.logFilePaths {
		t.Logf("\n--- Validating file after single file rebuild: %s ---", filepath.Base(filePath))
		suite.ValidateCardinalityCounter(t, filePath)
		suite.ValidateAnalyticsData(t, filePath)
		suite.ValidatePaginationFunctionality(t, filePath)
	}

	t.Log("\n=== Integration Test Completed Successfully ===")
}

// TestConcurrentIndexingAndQuerying tests querying while indexing is in progress
func TestConcurrentIndexingAndQuerying(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent integration test in short mode")
	}
	suite := NewIntegrationTestSuite(t)
	defer suite.cleanup()

	t.Log("=== Starting Concurrent Indexing and Querying Test ===")

	// Generate test data and initialize services
	suite.GenerateTestData(t)
	suite.InitializeServices(t)

	var wg sync.WaitGroup

	// Start indexing in background
	wg.Add(1)
	go func() {
		defer wg.Done()
		suite.PerformGlobalIndexRebuild(t)
	}()

	// Wait a bit for indexing to start
	time.Sleep(2 * time.Second)

	// Query while indexing is in progress
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 10; i++ {
			time.Sleep(1 * time.Second)

			// Test search functionality
			if suite.searcher.IsHealthy() {
				searchReq := &searcher.SearchRequest{
					Query:    "GET",
					LogPaths: []string{suite.logFilePaths[0]},
					Limit:    10,
				}

				result, err := suite.searcher.Search(suite.ctx, searchReq)
				if err == nil {
					t.Logf("Concurrent query %d: found %d results", i+1, result.TotalHits)
				}
			}
		}
	}()

	wg.Wait()

	// Final validation
	for _, filePath := range suite.logFilePaths {
		suite.ValidateCardinalityCounter(t, filePath)
		suite.ValidateAnalyticsData(t, filePath)
	}

	t.Log("=== Concurrent Test Completed Successfully ===")
}
