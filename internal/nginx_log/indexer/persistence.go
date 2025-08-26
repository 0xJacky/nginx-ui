package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gen/field"
)

// PersistenceManager handles database operations for log index positions
// Enhanced for incremental indexing with position tracking
type PersistenceManager struct {
	// Configuration for incremental indexing
	maxBatchSize  int
	flushInterval time.Duration
	enabledPaths  map[string]bool // Cache for enabled paths
	lastFlushTime time.Time
}

// LogFileInfo represents information about a log file for incremental indexing
type LogFileInfo struct {
	Path         string
	LastModified int64 // Unix timestamp
	LastSize     int64 // File size at last index
	LastIndexed  int64 // Unix timestamp of last indexing
	LastPosition int64 // Byte position where indexing left off
}

// IncrementalIndexConfig configuration for incremental indexing
type IncrementalIndexConfig struct {
	MaxBatchSize  int           `yaml:"max_batch_size" json:"max_batch_size"`
	FlushInterval time.Duration `yaml:"flush_interval" json:"flush_interval"`
	CheckInterval time.Duration `yaml:"check_interval" json:"check_interval"`
	MaxAge        time.Duration `yaml:"max_age" json:"max_age"`
}

// DefaultIncrementalConfig returns the default configuration for incremental indexing
func DefaultIncrementalConfig() *IncrementalIndexConfig {
	return &IncrementalIndexConfig{
		MaxBatchSize:  1000,
		FlushInterval: 30 * time.Second,
		CheckInterval: 5 * time.Minute,
		MaxAge:        30 * 24 * time.Hour, // 30 days
	}
}

// NewPersistenceManager creates a new persistence manager with incremental indexing support
func NewPersistenceManager(config *IncrementalIndexConfig) *PersistenceManager {
	if config == nil {
		config = DefaultIncrementalConfig()
	}

	return &PersistenceManager{
		maxBatchSize:  config.MaxBatchSize,
		flushInterval: config.FlushInterval,
		enabledPaths:  make(map[string]bool),
		lastFlushTime: time.Now(),
	}
}

// GetLogIndex retrieves the index record for a log file path
func (pm *PersistenceManager) GetLogIndex(path string) (*model.NginxLogIndex, error) {
	q := query.NginxLogIndex

	// Determine main log path for grouping
	mainLogPath := getMainLogPathFromFile(path)

	// Use FirstOrCreate to get existing record or create a new one
	logIndex, err := q.Where(q.Path.Eq(path)).
		Assign(field.Attrs(&model.NginxLogIndex{
			Path:        path,
			MainLogPath: mainLogPath,
			Enabled:     true,
		})).
		FirstOrCreate()

	if err != nil {
		return nil, fmt.Errorf("failed to get or create log index: %w", err)
	}

	return logIndex, nil
}

// SaveLogIndex saves or updates the index record with incremental indexing support
func (pm *PersistenceManager) SaveLogIndex(logIndex *model.NginxLogIndex) error {
	logIndex.Enabled = true

	// Ensure MainLogPath is set
	if logIndex.MainLogPath == "" {
		logIndex.MainLogPath = getMainLogPathFromFile(logIndex.Path)
	}

	// Update last indexed time
	logIndex.LastIndexed = time.Now()

	q := query.NginxLogIndex
	savedRecord, err := q.Where(q.Path.Eq(logIndex.Path)).
		Assign(field.Attrs(logIndex)).
		FirstOrCreate()

	if err != nil {
		return fmt.Errorf("failed to save log index: %w", err)
	}

	// Update the passed object with the saved record data
	*logIndex = *savedRecord

	// Update cache
	pm.enabledPaths[logIndex.Path] = logIndex.Enabled

	return nil
}

// GetIncrementalInfo retrieves incremental indexing information for a log file
func (pm *PersistenceManager) GetIncrementalInfo(path string) (*LogFileInfo, error) {
	logIndex, err := pm.GetLogIndex(path)
	if err != nil {
		return nil, err
	}

	return &LogFileInfo{
		Path:         logIndex.Path,
		LastModified: logIndex.LastModified.Unix(),
		LastSize:     logIndex.LastSize,
		LastIndexed:  logIndex.LastIndexed.Unix(),
		LastPosition: logIndex.LastPosition,
	}, nil
}

// UpdateIncrementalInfo updates incremental indexing information
func (pm *PersistenceManager) UpdateIncrementalInfo(path string, info *LogFileInfo) error {
	logIndex, err := pm.GetLogIndex(path)
	if err != nil {
		return err
	}

	logIndex.LastModified = time.Unix(info.LastModified, 0)
	logIndex.LastSize = info.LastSize
	logIndex.LastIndexed = time.Unix(info.LastIndexed, 0)
	logIndex.LastPosition = info.LastPosition

	return pm.SaveLogIndex(logIndex)
}

// IsPathEnabled checks if indexing is enabled for a path (with caching)
func (pm *PersistenceManager) IsPathEnabled(path string) (bool, error) {
	// Check cache first
	if enabled, exists := pm.enabledPaths[path]; exists {
		return enabled, nil
	}

	// Query database
	logIndex, err := pm.GetLogIndex(path)
	if err != nil {
		return false, err
	}

	// Update cache
	pm.enabledPaths[path] = logIndex.Enabled
	return logIndex.Enabled, nil
}

// GetChangedFiles returns files that have been modified since last indexing
func (pm *PersistenceManager) GetChangedFiles(mainLogPath string) ([]*model.NginxLogIndex, error) {
	q := query.NginxLogIndex
	indexes, err := q.Where(
		q.MainLogPath.Eq(mainLogPath),
		q.Enabled.Is(true),
	).Find()

	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	return indexes, nil
}

// GetFilesForFullReindex returns files that need full reindexing
func (pm *PersistenceManager) GetFilesForFullReindex(mainLogPath string, maxAge time.Duration) ([]*model.NginxLogIndex, error) {
	cutoff := time.Now().Add(-maxAge)
	q := query.NginxLogIndex

	indexes, err := q.Where(
		q.MainLogPath.Eq(mainLogPath),
		q.Enabled.Is(true),
		q.LastIndexed.Lt(cutoff),
	).Find()

	if err != nil {
		return nil, fmt.Errorf("failed to get files for full reindex: %w", err)
	}

	return indexes, nil
}

// MarkFileAsIndexed marks a file as successfully indexed with current timestamp and position
func (pm *PersistenceManager) MarkFileAsIndexed(path string, documentCount uint64, lastPosition int64) error {
	logIndex, err := pm.GetLogIndex(path)
	if err != nil {
		return err
	}

	now := time.Now()
	logIndex.LastIndexed = now
	logIndex.LastPosition = lastPosition
	logIndex.DocumentCount = documentCount

	return pm.SaveLogIndex(logIndex)
}

// GetAllLogIndexes retrieves all log index records
func (pm *PersistenceManager) GetAllLogIndexes() ([]*model.NginxLogIndex, error) {
	q := query.NginxLogIndex
	indexes, err := q.Where(q.Enabled.Is(true)).Order(q.Path).Find()
	if err != nil {
		return nil, fmt.Errorf("failed to get log indexes: %w", err)
	}

	return indexes, nil
}

// GetLogGroupIndexes retrieves all log index records for a specific log group
func (pm *PersistenceManager) GetLogGroupIndexes(mainLogPath string) ([]*model.NginxLogIndex, error) {
	q := query.NginxLogIndex
	indexes, err := q.Where(
		q.MainLogPath.Eq(mainLogPath),
		q.Enabled.Is(true),
	).Order(q.Path).Find()

	if err != nil {
		return nil, fmt.Errorf("failed to get log group indexes: %w", err)
	}

	return indexes, nil
}

// DeleteLogIndex deletes a log index record (hard delete)
func (pm *PersistenceManager) DeleteLogIndex(path string) error {
	q := query.NginxLogIndex
	_, err := q.Unscoped().Where(q.Path.Eq(path)).Delete()
	if err != nil {
		return fmt.Errorf("failed to delete log index: %w", err)
	}

	// Remove from cache
	delete(pm.enabledPaths, path)

	logger.Infof("Hard deleted log index for path: %s", path)
	return nil
}

// DisableLogIndex disables indexing for a log file
func (pm *PersistenceManager) DisableLogIndex(path string) error {
	q := query.NginxLogIndex
	_, err := q.Where(q.Path.Eq(path)).Update(q.Enabled, false)
	if err != nil {
		return fmt.Errorf("failed to disable log index: %w", err)
	}

	// Update cache
	pm.enabledPaths[path] = false

	logger.Infof("Disabled log index for path: %s", path)
	return nil
}

// EnableLogIndex enables indexing for a log file
func (pm *PersistenceManager) EnableLogIndex(path string) error {
	q := query.NginxLogIndex
	_, err := q.Where(q.Path.Eq(path)).Update(q.Enabled, true)
	if err != nil {
		return fmt.Errorf("failed to enable log index: %w", err)
	}

	// Update cache
	pm.enabledPaths[path] = true

	logger.Infof("Enabled log index for path: %s", path)
	return nil
}

// CleanupOldIndexes removes index records for files that haven't been indexed in a long time
func (pm *PersistenceManager) CleanupOldIndexes(maxAge time.Duration) error {
	cutoff := time.Now().Add(-maxAge)
	q := query.NginxLogIndex
	result, err := q.Unscoped().Where(q.LastIndexed.Lt(cutoff)).Delete()
	if err != nil {
		return fmt.Errorf("failed to cleanup old indexes: %w", err)
	}

	if result.RowsAffected > 0 {
		logger.Infof("Cleaned up %d old log index records", result.RowsAffected)
		// Clear cache for cleaned up entries
		pm.enabledPaths = make(map[string]bool)
	}

	return nil
}

// PersistenceStats represents statistics about stored index records
type PersistenceStats struct {
	TotalFiles     int64  `json:"total_files"`
	EnabledFiles   int64  `json:"enabled_files"`
	TotalDocuments uint64 `json:"total_documents"`
	ChangedFiles   int64  `json:"changed_files"`
}

// GetPersistenceStats returns statistics about stored index records
func (pm *PersistenceManager) GetPersistenceStats() (*PersistenceStats, error) {
	q := query.NginxLogIndex

	// Count total records
	totalCount, err := q.Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count total indexes: %w", err)
	}

	// Count enabled records
	enabledCount, err := q.Where(q.Enabled.Is(true)).Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count enabled indexes: %w", err)
	}

	// Sum document counts
	var result struct {
		Total uint64
	}
	if err := q.Select(q.DocumentCount.Sum().As("total")).Scan(&result); err != nil {
		return nil, fmt.Errorf("failed to sum document counts: %w", err)
	}

	// Count files needing incremental update
	cutoff := time.Now().Add(-time.Hour) // Files modified in last hour
	changedCount, err := q.Where(
		q.Enabled.Is(true),
		q.LastModified.Gt(cutoff),
	).Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count changed files: %w", err)
	}

	return &PersistenceStats{
		TotalFiles:     totalCount,
		EnabledFiles:   enabledCount,
		TotalDocuments: result.Total,
		ChangedFiles:   changedCount,
	}, nil
}

// GetLogFileInfo retrieves the log file info for a given path.
func (pm *PersistenceManager) GetLogFileInfo(path string) (*LogFileInfo, error) {
	return pm.GetIncrementalInfo(path)
}

// SaveLogFileInfo saves the log file info for a given path.
func (pm *PersistenceManager) SaveLogFileInfo(path string, info *LogFileInfo) error {
	return pm.UpdateIncrementalInfo(path, info)
}

// Close flushes any pending operations and cleans up resources
func (pm *PersistenceManager) Close() error {
	// Flush any pending operations
	pm.enabledPaths = nil
	return nil
}

// DeleteAllLogIndexes deletes all log index records
func (pm *PersistenceManager) DeleteAllLogIndexes() error {
	// GORM's `Delete` requires a WHERE clause for safety. To delete all records,
	// we use a raw Exec call, which is the standard way to perform bulk operations.
	db := cosy.UseDB(context.Background())
	if err := db.Exec("DELETE FROM nginx_log_indices").Error; err != nil {
		return fmt.Errorf("failed to delete all log indexes: %w", err)
	}

	// Clear cache
	pm.enabledPaths = make(map[string]bool)

	logger.Infof("Hard deleted all log index records")
	return nil
}

// DeleteLogIndexesByGroup deletes all log index records for a specific log group.
func (pm *PersistenceManager) DeleteLogIndexesByGroup(mainLogPath string) error {
	q := query.NginxLogIndex
	result, err := q.Unscoped().Where(q.MainLogPath.Eq(mainLogPath)).Delete()
	if err != nil {
		return fmt.Errorf("failed to delete log indexes for group %s: %w", mainLogPath, err)
	}

	logger.Infof("Deleted %d log index records for group: %s", result.RowsAffected, mainLogPath)
	return nil
}

// RefreshCache refreshes the enabled paths cache
func (pm *PersistenceManager) RefreshCache() error {
	q := query.NginxLogIndex
	indexes, err := q.Select(q.Path, q.Enabled).Find()
	if err != nil {
		return fmt.Errorf("failed to refresh cache: %w", err)
	}

	// Rebuild cache
	pm.enabledPaths = make(map[string]bool)
	for _, index := range indexes {
		pm.enabledPaths[index.Path] = index.Enabled
	}

	return nil
}

// IncrementalIndexStats represents statistics specific to incremental indexing
type IncrementalIndexStats struct {
	GroupFiles   int64 `json:"group_files"`
	ChangedFiles int   `json:"changed_files"`
	OldFiles     int   `json:"old_files"`
	NeedsReindex int   `json:"needs_reindex"`
}

// GetIncrementalIndexStats returns statistics specific to incremental indexing
func (pm *PersistenceManager) GetIncrementalIndexStats(mainLogPath string) (*IncrementalIndexStats, error) {
	q := query.NginxLogIndex

	// Files in this log group
	groupCount, err := q.Where(q.MainLogPath.Eq(mainLogPath), q.Enabled.Is(true)).Count()
	if err != nil {
		return nil, fmt.Errorf("failed to count group files: %w", err)
	}

	// Files needing incremental update
	changedFiles, err := pm.GetChangedFiles(mainLogPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	// Files needing full reindex (older than 7 days)
	oldFiles, err := pm.GetFilesForFullReindex(mainLogPath, 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to get old files: %w", err)
	}

	return &IncrementalIndexStats{
		GroupFiles:   groupCount,
		ChangedFiles: len(changedFiles),
		OldFiles:     len(oldFiles),
		NeedsReindex: len(changedFiles) + len(oldFiles),
	}, nil
}

// getMainLogPathFromFile extracts the main log path from a file (including rotated files)
// Enhanced for better rotation pattern detection
func getMainLogPathFromFile(filePath string) string {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)

	// Remove compression extensions (.gz, .bz2, .xz, .lz4)
	for _, ext := range []string{".gz", ".bz2", ".xz", ".lz4"} {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Check if it's a dot-separated date rotation FIRST (access.log.YYYYMMDD or access.log.YYYY.MM.DD)
	// This must come before numbered rotation check to avoid false positives
	parts := strings.Split(filename, ".")
	if len(parts) >= 3 {
		// First check for multi-part date patterns like YYYY.MM.DD (need at least 4 parts total)
		if len(parts) >= 4 {
			// Try to match the last 3 parts as a date
			lastThreeParts := strings.Join(parts[len(parts)-3:], ".")
			// Check if this looks like YYYY.MM.DD pattern
			if matched, _ := regexp.MatchString(`^\d{4}\.\d{2}\.\d{2}$`, lastThreeParts); matched {
				// Remove the date parts (last 3 parts)
				basenameParts := parts[:len(parts)-3]
				baseFilename := strings.Join(basenameParts, ".")
				return filepath.Join(dir, baseFilename)
			}
		}

		// Then check for single-part date patterns in the last part
		lastPart := parts[len(parts)-1]
		if isFullDatePattern(lastPart) { // Only match full date patterns, not partial ones
			// Remove the date part
			basenameParts := parts[:len(parts)-1]
			baseFilename := strings.Join(basenameParts, ".")
			return filepath.Join(dir, baseFilename)
		}
	}

	// Handle numbered rotation (access.log.1, access.log.2, etc.)
	// This comes AFTER date pattern checks to avoid matching date components as rotation numbers
	if match := regexp.MustCompile(`^(.+)\.(\d{1,3})$`).FindStringSubmatch(filename); len(match) > 1 {
		baseFilename := match[1]
		return filepath.Join(dir, baseFilename)
	}

	// Handle middle-numbered rotation (access.1.log, access.2.log)
	if match := regexp.MustCompile(`^(.+)\.(\d{1,3})\.log$`).FindStringSubmatch(filename); len(match) > 1 {
		baseName := match[1]
		return filepath.Join(dir, baseName+".log")
	}

	// Handle date-based rotation (access.20231201, access.2023-12-01, etc.)
	if isDatePattern(filename) {
		// This is a date-based rotation, return the parent directory
		// as we can't determine the exact base name
		return filepath.Join(dir, "access.log") // Default assumption
	}

	// If no rotation pattern is found, return the original path
	return filePath
}

// isDatePattern checks if a string looks like a date pattern (including multi-part)
func isDatePattern(s string) bool {
	// Check for full date patterns first
	if isFullDatePattern(s) {
		return true
	}

	// Check for multi-part date patterns like YYYY.MM.DD
	if matched, _ := regexp.MatchString(`^2\d{3}\.\d{2}\.\d{2}$`, s); matched {
		return true
	}

	return false
}

// isFullDatePattern checks if a string is a complete date pattern (not partial)
func isFullDatePattern(s string) bool {
	// Complete date patterns for log rotation
	patterns := []string{
		`^\d{8}$`,             // YYYYMMDD
		`^\d{4}-\d{2}-\d{2}$`, // YYYY-MM-DD
		`^\d{6}$`,             // YYMMDD
	}

	for _, pattern := range patterns {
		if matched, _ := regexp.MatchString(pattern, s); matched {
			return true
		}
	}
	return false
}
