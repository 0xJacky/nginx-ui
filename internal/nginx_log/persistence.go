package nginx_log

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/uozi-tech/cosy/logger"
	"gorm.io/gen/field"
	"gorm.io/gorm"
)

// PersistenceManager handles database operations for log index positions
type PersistenceManager struct{}

// NewPersistenceManager creates a new persistence manager
func NewPersistenceManager() *PersistenceManager {
	return &PersistenceManager{}
}

// GetLogIndex retrieves the index record for a log file path
func (pm *PersistenceManager) GetLogIndex(path string) (*model.NginxLogIndex, error) {
	q := query.NginxLogIndex
	logIndex, err := q.Where(q.Path.Eq(path)).First()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return a new record for first-time indexing
			// Determine main log path for grouping
			mainLogPath := getMainLogPathFromFile(path)
			return &model.NginxLogIndex{
				Path:        path,
				MainLogPath: mainLogPath,
				Enabled:     true,
			}, nil
		}
		return nil, fmt.Errorf("failed to get log index: %w", err)
	}

	return logIndex, nil
}

// SaveLogIndex saves or updates the index record using gen Assign.FirstOrCreate
func (pm *PersistenceManager) SaveLogIndex(logIndex *model.NginxLogIndex) error {
	logIndex.Enabled = true

	// Ensure MainLogPath is set
	if logIndex.MainLogPath == "" {
		logIndex.MainLogPath = getMainLogPathFromFile(logIndex.Path)
	}

	q := query.NginxLogIndex
	savedRecord, err := q.Where(q.Path.Eq(logIndex.Path)).
		Assign(field.Attrs(logIndex)).
		FirstOrCreate()

	if err != nil {
		return fmt.Errorf("failed to save log index: %w", err)
	}

	// Update the passed object with the saved record data
	*logIndex = *savedRecord
	return nil
}

// GetAllLogIndexes retrieves all log index records
func (pm *PersistenceManager) GetAllLogIndexes() ([]*model.NginxLogIndex, error) {
	q := query.NginxLogIndex
	indexes, err := q.Where(q.Enabled.Is(true)).Find()
	if err != nil {
		return nil, fmt.Errorf("failed to get log indexes: %w", err)
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
	}

	return nil
}

// GetIndexStats returns statistics about stored index records
func (pm *PersistenceManager) GetIndexStats() (map[string]interface{}, error) {
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

	return map[string]interface{}{
		"total_files":     totalCount,
		"enabled_files":   enabledCount,
		"total_documents": result.Total,
	}, nil
}

// GetLogFileInfo retrieves the log file info for a given path.
func (pm *PersistenceManager) GetLogFileInfo(path string) (*LogFileInfo, error) {
	logIndex, err := pm.GetLogIndex(path)
	if err != nil {
		return nil, err
	}
	return &LogFileInfo{
		Path:         logIndex.Path,
		LastModified: logIndex.LastModified.Unix(),
		LastSize:     logIndex.LastSize,
		LastIndexed:  logIndex.LastIndexed.Unix(),
	}, nil
}

// SaveLogFileInfo saves the log file info for a given path.
func (pm *PersistenceManager) SaveLogFileInfo(path string, info *LogFileInfo) error {
	logIndex, err := pm.GetLogIndex(path)
	if err != nil {
		return err
	}
	logIndex.LastModified = time.Unix(info.LastModified, 0)
	logIndex.LastSize = info.LastSize
	logIndex.LastIndexed = time.Unix(info.LastIndexed, 0)
	return pm.SaveLogIndex(logIndex)
}

// Close is a no-op for PersistenceManager (database connections are managed globally)
func (pm *PersistenceManager) Close() error {
	return nil
}

// DeleteAllLogIndexes deletes all log index records
func (pm *PersistenceManager) DeleteAllLogIndexes() error {
	q := query.NginxLogIndex
	result, err := q.Unscoped().Delete()
	if err != nil {
		return fmt.Errorf("failed to delete all log indexes: %w", err)
	}

	logger.Infof("Deleted all %d log index records", result.RowsAffected)
	return nil
}

// getMainLogPathFromFile extracts the main log path from a file (including rotated files)
// This is a standalone version of getMainLogPath for use in persistence layer
func getMainLogPathFromFile(filePath string) string {
	dir := filepath.Dir(filePath)
	filename := filepath.Base(filePath)

	// Remove .gz compression suffix if present
	filename = strings.TrimSuffix(filename, ".gz")

	// Handle numbered rotation (access.log.1, access.log.2, etc.)
	// Use a more specific pattern to avoid matching date patterns like "20231201"
	if match := regexp.MustCompile(`^(.+)\.(\d{1,3})$`).FindStringSubmatch(filename); len(match) > 1 {
		// Only match if the number is reasonable for rotation (1-999)
		baseFilename := match[1]
		return filepath.Join(dir, baseFilename)
	}

	// Handle date-based rotation (access.20231201, access.2023-12-01, etc.)
	if isDatePattern(filename) {
		// This is a date-based rotation, return the parent directory
		// as we can't determine the exact base name
		return filepath.Join(dir, "access.log") // Default assumption
	}

	// Check if it's a dot-separated rotation (access.log.YYYYMMDD)
	parts := strings.Split(filename, ".")
	if len(parts) >= 3 {
		lastPart := parts[len(parts)-1]
		if isDatePattern(lastPart) {
			// Remove the date part
			basenameParts := parts[:len(parts)-1]
			baseFilename := strings.Join(basenameParts, ".")
			return filepath.Join(dir, baseFilename)
		}
	}

	// If no rotation pattern is found, return the original path
	return filePath
}
