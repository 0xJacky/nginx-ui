package indexer

import (
	"fmt"
	"testing"
	"time"
)

func TestPersistenceManager_Creation(t *testing.T) {
	// Test default config
	pm := NewPersistenceManager(nil)
	if pm == nil {
		t.Fatal("Expected non-nil persistence manager")
	}
	
	if pm.maxBatchSize != 1000 {
		t.Errorf("Expected default batch size 1000, got %d", pm.maxBatchSize)
	}
	
	if pm.flushInterval != 30*time.Second {
		t.Errorf("Expected default flush interval 30s, got %v", pm.flushInterval)
	}
	
	// Test custom config
	config := &IncrementalIndexConfig{
		MaxBatchSize:  500,
		FlushInterval: 15 * time.Second,
		CheckInterval: 2 * time.Minute,
		MaxAge:        7 * 24 * time.Hour,
	}
	
	pm2 := NewPersistenceManager(config)
	if pm2.maxBatchSize != 500 {
		t.Errorf("Expected custom batch size 500, got %d", pm2.maxBatchSize)
	}
	
	if pm2.flushInterval != 15*time.Second {
		t.Errorf("Expected custom flush interval 15s, got %v", pm2.flushInterval)
	}
}

func TestIncrementalIndexConfig_Default(t *testing.T) {
	config := DefaultIncrementalConfig()
	
	if config.MaxBatchSize != 1000 {
		t.Errorf("Expected default MaxBatchSize 1000, got %d", config.MaxBatchSize)
	}
	
	if config.FlushInterval != 30*time.Second {
		t.Errorf("Expected default FlushInterval 30s, got %v", config.FlushInterval)
	}
	
	if config.CheckInterval != 5*time.Minute {
		t.Errorf("Expected default CheckInterval 5m, got %v", config.CheckInterval)
	}
	
	if config.MaxAge != 30*24*time.Hour {
		t.Errorf("Expected default MaxAge 30 days, got %v", config.MaxAge)
	}
}

func TestGetMainLogPathFromFile(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"/var/log/nginx/access.log", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.1", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.2.gz", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.1.log", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.2.log.gz", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.3.log.bz2", "/var/log/nginx/access.log"},
		{"/var/log/nginx/error.log.10", "/var/log/nginx/error.log"},
		{"/var/log/nginx/error.1.log.xz", "/var/log/nginx/error.log"},
		{"/var/log/nginx/access.log.20231201", "/var/log/nginx/access.log"},
		{"/var/log/nginx/access.log.2023-12-01", "/var/log/nginx/access.log"},
		{"/var/log/nginx/custom.log.99", "/var/log/nginx/custom.log"},
		{"/logs/app.5.log.lz4", "/logs/app.log"},
	}
	
	for _, tc := range testCases {
		result := getMainLogPathFromFile(tc.input)
		if result != tc.expected {
			t.Errorf("getMainLogPathFromFile(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestIsDatePattern(t *testing.T) {
	testCases := []struct {
		input    string
		expected bool
	}{
		{"20231201", true},     // YYYYMMDD
		{"2023-12-01", true},   // YYYY-MM-DD
		{"2023.12.01", true},   // YYYY.MM.DD
		{"231201", true},       // YYMMDD
		{"access", false},      // Not a date
		{"123", false},         // Too short
		{"12345678901", false}, // Too long
		{"2023-13-01", true},   // Would match pattern (validation not checked)
		{"log", false},         // Text
		{"1", false},           // Single digit
	}
	
	for _, tc := range testCases {
		result := isDatePattern(tc.input)
		if result != tc.expected {
			t.Errorf("isDatePattern(%s) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestLogFileInfo_Structure(t *testing.T) {
	// Test LogFileInfo struct initialization
	info := &LogFileInfo{
		Path:         "/var/log/nginx/access.log",
		LastModified: time.Now().Unix(),
		LastSize:     1024,
		LastIndexed:  time.Now().Unix(),
		LastPosition: 512,
	}
	
	if info.Path != "/var/log/nginx/access.log" {
		t.Errorf("Expected path to be set correctly")
	}
	
	if info.LastSize != 1024 {
		t.Errorf("Expected LastSize 1024, got %d", info.LastSize)
	}
	
	if info.LastPosition != 512 {
		t.Errorf("Expected LastPosition 512, got %d", info.LastPosition)
	}
	
	// Checksum field removed from LogFileInfo
}

// Mock tests (without database dependency)
func TestPersistenceManager_CacheOperations(t *testing.T) {
	pm := NewPersistenceManager(nil)
	
	// Test initial cache state
	if len(pm.enabledPaths) != 0 {
		t.Errorf("Expected empty cache initially, got %d entries", len(pm.enabledPaths))
	}
	
	// Simulate cache operations
	pm.enabledPaths["/test/path1"] = true
	pm.enabledPaths["/test/path2"] = false
	
	if len(pm.enabledPaths) != 2 {
		t.Errorf("Expected 2 cache entries, got %d", len(pm.enabledPaths))
	}
	
	// Test RefreshCache method preparation (would need database in real scenario)
	pm.enabledPaths = make(map[string]bool)
	if len(pm.enabledPaths) != 0 {
		t.Errorf("Expected cache to be cleared")
	}
}

func TestPersistenceManager_ConfigValidation(t *testing.T) {
	// Test with various configurations
	configs := []*IncrementalIndexConfig{
		{
			MaxBatchSize:  100,
			FlushInterval: 5 * time.Second,
			CheckInterval: 1 * time.Minute,
			MaxAge:        1 * time.Hour,
		},
		{
			MaxBatchSize:  10000,
			FlushInterval: 5 * time.Minute,
			CheckInterval: 30 * time.Minute,
			MaxAge:        90 * 24 * time.Hour,
		},
	}
	
	for i, config := range configs {
		pm := NewPersistenceManager(config)
		if pm.maxBatchSize != config.MaxBatchSize {
			t.Errorf("Config %d: Expected MaxBatchSize %d, got %d", i, config.MaxBatchSize, pm.maxBatchSize)
		}
		
		if pm.flushInterval != config.FlushInterval {
			t.Errorf("Config %d: Expected FlushInterval %v, got %v", i, config.FlushInterval, pm.flushInterval)
		}
	}
}

func TestGetMainLogPathFromFile_EdgeCases(t *testing.T) {
	edgeCases := []struct {
		input       string
		expected    string
		description string
	}{
		{
			"/var/log/nginx/access.999.log.gz",
			"/var/log/nginx/access.log",
			"High rotation number with compression",
		},
		{
			"/var/log/nginx/access.log.2023.12.01.gz",
			"/var/log/nginx/access.log",
			"Date-based rotation with compression",
		},
		{
			"/single/file.log",
			"/single/file.log",
			"No rotation pattern",
		},
		{
			"/path/with.dots.in.name.log.1",
			"/path/with.dots.in.name.log",
			"Multiple dots in filename",
		},
		{
			"/var/log/nginx/access.log.1000",
			"/var/log/nginx/access.log.1000",
			"Number too high for rotation (should not match)",
		},
	}
	
	for _, tc := range edgeCases {
		result := getMainLogPathFromFile(tc.input)
		if result != tc.expected {
			t.Errorf("%s: getMainLogPathFromFile(%s) = %s, expected %s",
				tc.description, tc.input, result, tc.expected)
		}
	}
}

func TestPersistenceManager_Close(t *testing.T) {
	pm := NewPersistenceManager(nil)
	
	// Add some cache entries
	pm.enabledPaths["/test/path1"] = true
	pm.enabledPaths["/test/path2"] = false
	
	// Close should clean up
	err := pm.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}
	
	// Cache should be cleared
	if pm.enabledPaths != nil {
		t.Errorf("Expected cache to be nil after close")
	}
}

// Benchmark tests for performance validation
func BenchmarkGetMainLogPathFromFile(b *testing.B) {
	testPaths := []string{
		"/var/log/nginx/access.log",
		"/var/log/nginx/access.log.1",
		"/var/log/nginx/access.log.2.gz",
		"/var/log/nginx/access.1.log",
		"/var/log/nginx/access.2.log.gz",
		"/var/log/nginx/error.log.20231201",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := testPaths[i%len(testPaths)]
		_ = getMainLogPathFromFile(path)
	}
}

func BenchmarkIsDatePattern(b *testing.B) {
	testStrings := []string{
		"20231201",
		"2023-12-01", 
		"access",
		"log",
		"231201",
		"notadate",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := testStrings[i%len(testStrings)]
		_ = isDatePattern(s)
	}
}

func BenchmarkPersistenceManager_CacheAccess(b *testing.B) {
	pm := NewPersistenceManager(nil)
	
	// Populate cache
	for i := 0; i < 1000; i++ {
		pm.enabledPaths[fmt.Sprintf("/path/file%d.log", i)] = i%2 == 0
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := fmt.Sprintf("/path/file%d.log", i%1000)
		_ = pm.enabledPaths[path]
	}
}