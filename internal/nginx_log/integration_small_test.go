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
	// Small test configuration for faster execution
	SmallTestRecordsPerFile = 1000  // 1000条记录每个文件
	SmallTestFileCount      = 3     // 3个测试文件
)

// SmallIntegrationTestSuite contains all integration test data and services (small version)
type SmallIntegrationTestSuite struct {
	ctx              context.Context
	cancel           context.CancelFunc
	tempDir          string
	indexDir         string
	logFiles         []string
	logFilePaths     []string
	indexer          *indexer.ParallelIndexer
	searcher         searcher.Searcher
	analytics        analytics.Service
	logFileManager   *TestLogFileManager
	expectedMetrics  map[string]*SmallExpectedFileMetrics
	mu               sync.RWMutex
	cleanup          func()
}

// SmallExpectedFileMetrics stores expected statistics for each log file (small version)
type SmallExpectedFileMetrics struct {
	TotalRecords  uint64
	UniqueIPs     uint64
	UniquePaths   uint64
	UniqueAgents  uint64
	StatusCodes   map[int]uint64
	Methods       map[string]uint64
	TimeRange     SmallTestTimeRange
}

// SmallTestTimeRange represents the time range of log entries for small testing
type SmallTestTimeRange struct {
	StartTime time.Time
	EndTime   time.Time
}

// NewSmallIntegrationTestSuite creates a new small integration test suite
func NewSmallIntegrationTestSuite(t *testing.T) *SmallIntegrationTestSuite {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create temporary directories
	tempDir, err := os.MkdirTemp("", "nginx_ui_small_integration_test_*")
	require.NoError(t, err)
	
	indexDir := filepath.Join(tempDir, "index")
	logsDir := filepath.Join(tempDir, "logs")
	
	err = os.MkdirAll(indexDir, 0755)
	require.NoError(t, err)
	
	err = os.MkdirAll(logsDir, 0755)
	require.NoError(t, err)

	suite := &SmallIntegrationTestSuite{
		ctx:             ctx,
		cancel:          cancel,
		tempDir:         tempDir,
		indexDir:        indexDir,
		expectedMetrics: make(map[string]*SmallExpectedFileMetrics),
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

// GenerateSmallTestData generates the small test log files with expected statistics
func (suite *SmallIntegrationTestSuite) GenerateSmallTestData(t *testing.T) {
	t.Logf("Generating %d test files with %d records each", SmallTestFileCount, SmallTestRecordsPerFile)
	
	baseTime := time.Now().Add(-24 * time.Hour)
	
	for i := 0; i < SmallTestFileCount; i++ {
		filename := fmt.Sprintf("small_access_%d.log", i+1)
		filepath := filepath.Join(suite.tempDir, "logs", filename)
		
		metrics := suite.generateSmallSingleLogFile(t, filepath, baseTime.Add(time.Duration(i)*time.Hour))
		
		suite.logFiles = append(suite.logFiles, filename)
		suite.logFilePaths = append(suite.logFilePaths, filepath)
		suite.expectedMetrics[filepath] = metrics
		
		t.Logf("Generated %s with %d records", filename, metrics.TotalRecords)
	}
	
	t.Logf("Small test data generation completed. Total files: %d", len(suite.logFiles))
}

// generateSmallSingleLogFile generates a single small log file with known statistics
func (suite *SmallIntegrationTestSuite) generateSmallSingleLogFile(t *testing.T, filepath string, baseTime time.Time) *SmallExpectedFileMetrics {
	file, err := os.Create(filepath)
	require.NoError(t, err)
	defer file.Close()

	metrics := &SmallExpectedFileMetrics{
		StatusCodes: make(map[int]uint64),
		Methods:     make(map[string]uint64),
		TimeRange: SmallTestTimeRange{
			StartTime: baseTime,
			EndTime:   baseTime.Add(time.Duration(SmallTestRecordsPerFile) * time.Second),
		},
	}

	// Predefined test data for consistent testing
	ips := []string{
		"192.168.1.1", "192.168.1.2", "192.168.1.3", "10.0.0.1", "10.0.0.2",
	}
	
	paths := []string{
		"/", "/api/v1/status", "/api/v1/logs", "/admin", "/login",
	}
	
	userAgents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"PostmanRuntime/7.28.4",
	}
	
	statusCodes := []int{200, 301, 404, 500}
	methods := []string{"GET", "POST", "PUT"}

	// Track unique values
	uniqueIPs := make(map[string]bool)
	uniquePaths := make(map[string]bool)
	uniqueAgents := make(map[string]bool)

	rand.Seed(time.Now().UnixNano() + int64(len(filepath))) // Different seed per file

	for i := 0; i < SmallTestRecordsPerFile; i++ {
		// Generate log entry timestamp
		timestamp := baseTime.Add(time.Duration(i) * time.Second)
		
		// Select random values
		ip := ips[rand.Intn(len(ips))]
		path := paths[rand.Intn(len(paths))]
		agent := userAgents[rand.Intn(len(userAgents))]
		status := statusCodes[rand.Intn(len(statusCodes))]
		method := methods[rand.Intn(len(methods))]
		size := rand.Intn(1000) + 100 // 100-1100 bytes
		
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
	metrics.TotalRecords = SmallTestRecordsPerFile
	metrics.UniqueIPs = uint64(len(uniqueIPs))
	metrics.UniquePaths = uint64(len(uniquePaths))
	metrics.UniqueAgents = uint64(len(uniqueAgents))

	return metrics
}

// InitializeSmallServices initializes all nginx_log services for small testing
func (suite *SmallIntegrationTestSuite) InitializeSmallServices(t *testing.T) {
	t.Log("Initializing small test services...")
	
	// Initialize indexer
	indexerConfig := indexer.DefaultIndexerConfig()
	indexerConfig.IndexPath = suite.indexDir
	shardManager := indexer.NewDefaultShardManager(indexerConfig)
	suite.indexer = indexer.NewParallelIndexer(indexerConfig, shardManager)
	
	err := suite.indexer.Start(suite.ctx)
	require.NoError(t, err)
	
	// Initialize searcher (empty initially)
	searcherConfig := searcher.DefaultSearcherConfig()
	suite.searcher = searcher.NewDistributedSearcher(searcherConfig, []bleve.Index{})
	
	// Initialize analytics
	suite.analytics = analytics.NewService(suite.searcher)
	
	// Initialize log file manager with test-specific behavior
	suite.logFileManager = &TestLogFileManager{
		logCache:       make(map[string]*indexer.NginxLogCache),
		indexingStatus: make(map[string]bool),
		indexMetadata:  make(map[string]*TestIndexMetadata),
	}
	
	// Register test log files
	for _, logPath := range suite.logFilePaths {
		suite.logFileManager.AddLogPath(logPath, "access", filepath.Base(logPath), "test_config")
	}
	
	t.Log("Small services initialized successfully")
}

// PerformSmallGlobalIndexRebuild performs a complete index rebuild of all small files
func (suite *SmallIntegrationTestSuite) PerformSmallGlobalIndexRebuild(t *testing.T) {
	t.Log("Starting small global index rebuild...")
	
	startTime := time.Now()
	
	// Create progress tracking
	var completedFiles []string
	var mu sync.Mutex
	
	progressConfig := &indexer.ProgressConfig{
		NotifyInterval: 500 * time.Millisecond,
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
	
	suite.updateSmallSearcher(t)
	
	totalDuration := time.Since(startTime)
	t.Logf("Small global index rebuild completed in %s. Completed files: %v", totalDuration, completedFiles)
}

// updateSmallSearcher updates the searcher with current shards
func (suite *SmallIntegrationTestSuite) updateSmallSearcher(t *testing.T) {
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

// ValidateSmallCardinalityCounter validates the accuracy of cardinality counting
func (suite *SmallIntegrationTestSuite) ValidateSmallCardinalityCounter(t *testing.T, filePath string) {
	t.Logf("Validating CardinalityCounter accuracy for: %s", filePath)
	
	expected := suite.expectedMetrics[filePath]
	require.NotNil(t, expected, "Expected metrics not found for file: %s", filePath)
	
	if ds, ok := suite.searcher.(*searcher.DistributedSearcher); ok {
		cardinalityCounter := searcher.NewCardinalityCounter(ds.GetShards())
		
		// Test IP cardinality (for all files combined since we can't filter by file path yet)
		req := &searcher.CardinalityRequest{
			Field: "remote_addr",
		}
		
		result, err := cardinalityCounter.CountCardinality(suite.ctx, req)
		require.NoError(t, err, "Failed to count IP cardinality")
		
		// For combined files, we expect at least the unique IPs from this file
		// but possibly more since we're counting across all files
		assert.GreaterOrEqual(t, result.Cardinality, expected.UniqueIPs,
			"IP cardinality should be at least %d, got %d", expected.UniqueIPs, result.Cardinality)
		
		t.Logf("✓ IP cardinality (all files): actual=%d (expected at least %d), total_docs=%d",
			result.Cardinality, expected.UniqueIPs, result.TotalDocs)
	} else {
		t.Fatal("Searcher is not a DistributedSearcher")
	}
}

// ValidateSmallAnalyticsData validates the accuracy of analytics statistics
func (suite *SmallIntegrationTestSuite) ValidateSmallAnalyticsData(t *testing.T, filePath string) {
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
	tolerance := float64(10) // Small tolerance for small datasets
	assert.InDelta(t, expected.TotalRecords, dashboard.Summary.TotalPV, tolerance,
		"Total requests mismatch for %s", filePath)
	
	t.Logf("✓ Dashboard validation completed for: %s", filePath)
	t.Logf("  Total requests: expected=%d, actual=%d", expected.TotalRecords, dashboard.Summary.TotalPV)
	t.Logf("  Unique visitors: %d", dashboard.Summary.TotalUV)
	t.Logf("  Average daily PV: %f", dashboard.Summary.AvgDailyPV)
}

// ValidateSmallPaginationFunctionality validates pagination works correctly using searcher
func (suite *SmallIntegrationTestSuite) ValidateSmallPaginationFunctionality(t *testing.T, filePath string) {
	t.Logf("Validating pagination functionality for: %s", filePath)
	
	expected := suite.expectedMetrics[filePath]
	require.NotNil(t, expected, "Expected metrics not found for file: %s", filePath)
	
	// Test first page - search all records without any filters
	searchReq1 := &searcher.SearchRequest{
		Query:     "", // Empty query should use match_all
		Limit:     50,
		Offset:    0,
		SortBy:    "timestamp",
		SortOrder: "desc",
	}
	
	result1, err := suite.searcher.Search(suite.ctx, searchReq1)
	require.NoError(t, err, "Failed to get page 1 for: %s", filePath)
	
	// For small integration test, we expect at least some results from all files combined
	totalExpectedRecords := uint64(SmallTestFileCount * SmallTestRecordsPerFile)
	assert.Greater(t, len(result1.Hits), 0, "First page should have some entries")
	assert.Equal(t, totalExpectedRecords, result1.TotalHits, "Total count should match all files")
	
	// Test second page  
	searchReq2 := &searcher.SearchRequest{
		Query:     "", // Empty query should use match_all
		Limit:     50,
		Offset:    50,
		SortBy:    "timestamp",
		SortOrder: "desc",
	}
	
	result2, err := suite.searcher.Search(suite.ctx, searchReq2)
	require.NoError(t, err, "Failed to get page 2 for: %s", filePath)
	
	// Check that pagination works by ensuring we get different results
	assert.Greater(t, len(result2.Hits), 0, "Second page should have some entries")
	assert.Equal(t, totalExpectedRecords, result2.TotalHits, "Total count should be consistent")
	
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

// TestSmallNginxLogIntegration is the main small integration test function
func TestSmallNginxLogIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	suite := NewSmallIntegrationTestSuite(t)
	defer suite.cleanup()
	
	t.Log("=== Starting Small Nginx Log Integration Test ===")
	
	// Step 1: Generate test data
	suite.GenerateSmallTestData(t)
	
	// Step 2: Initialize services
	suite.InitializeSmallServices(t)
	
	// Step 3: Perform global index rebuild and validate
	t.Log("\n=== Testing Small Global Index Rebuild ===")
	suite.PerformSmallGlobalIndexRebuild(t)
	
	// Step 4: Validate all files after global rebuild
	for _, filePath := range suite.logFilePaths {
		t.Logf("\n--- Validating file after global rebuild: %s ---", filepath.Base(filePath))
		suite.ValidateSmallCardinalityCounter(t, filePath)
		suite.ValidateSmallAnalyticsData(t, filePath)
		suite.ValidateSmallPaginationFunctionality(t, filePath)
	}
	
	t.Log("\n=== Small Integration Test Completed Successfully ===")
}