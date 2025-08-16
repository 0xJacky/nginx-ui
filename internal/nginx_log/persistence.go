package nginx_log

import (
	"fmt"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/uozi-tech/cosy/logger"
)

// PersistenceManager handles database operations for log index positions
type PersistenceManager struct{}

// NewPersistenceManager creates a new persistence manager
func NewPersistenceManager() *PersistenceManager {
	return &PersistenceManager{}
}

// GetLogIndex retrieves the index record for a log file path
func (pm *PersistenceManager) GetLogIndex(path string) (*model.NginxLogIndex, error) {
	db := model.UseDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var logIndex model.NginxLogIndex
	result := db.Where("path = ?", path).First(&logIndex)
	if result.Error != nil {
		if result.Error.Error() == "record not found" {
			// Return a new record for first-time indexing
			return &model.NginxLogIndex{
				Path:    path,
				Enabled: true,
			}, nil
		}
		return nil, fmt.Errorf("failed to get log index: %w", result.Error)
	}

	return &logIndex, nil
}

// SaveLogIndex saves or updates the index record
func (pm *PersistenceManager) SaveLogIndex(logIndex *model.NginxLogIndex) error {
	db := model.UseDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	var existing model.NginxLogIndex
	result := db.Where("path = ?", logIndex.Path).First(&existing)
	
	if result.Error != nil && result.Error.Error() != "record not found" {
		return fmt.Errorf("failed to check existing log index: %w", result.Error)
	}

	if result.Error == nil {
		// Update existing record
		existing.LastModified = logIndex.LastModified
		existing.LastSize = logIndex.LastSize
		existing.LastPosition = logIndex.LastPosition
		existing.LastIndexed = logIndex.LastIndexed
		existing.TimeRangeStart = logIndex.TimeRangeStart
		existing.TimeRangeEnd = logIndex.TimeRangeEnd
		existing.DocumentCount = logIndex.DocumentCount
		existing.Enabled = logIndex.Enabled

		if err := db.Save(&existing).Error; err != nil {
			return fmt.Errorf("failed to update log index: %w", err)
		}
		
		// Update the ID for the passed object
		logIndex.ID = existing.ID
		logIndex.CreatedAt = existing.CreatedAt
		logIndex.UpdatedAt = existing.UpdatedAt
	} else {
		// Create new record
		if err := db.Create(logIndex).Error; err != nil {
			return fmt.Errorf("failed to create log index: %w", err)
		}
	}

	return nil
}

// GetAllLogIndexes retrieves all log index records
func (pm *PersistenceManager) GetAllLogIndexes() ([]*model.NginxLogIndex, error) {
	db := model.UseDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var indexes []*model.NginxLogIndex
	if err := db.Where("enabled = ?", true).Find(&indexes).Error; err != nil {
		return nil, fmt.Errorf("failed to get log indexes: %w", err)
	}

	return indexes, nil
}

// DeleteLogIndex deletes a log index record (soft delete)
func (pm *PersistenceManager) DeleteLogIndex(path string) error {
	db := model.UseDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	result := db.Where("path = ?", path).Delete(&model.NginxLogIndex{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete log index: %w", result.Error)
	}

	logger.Infof("Deleted log index for path: %s", path)
	return nil
}

// DisableLogIndex disables indexing for a log file
func (pm *PersistenceManager) DisableLogIndex(path string) error {
	db := model.UseDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	result := db.Model(&model.NginxLogIndex{}).Where("path = ?", path).Update("enabled", false)
	if result.Error != nil {
		return fmt.Errorf("failed to disable log index: %w", result.Error)
	}

	logger.Infof("Disabled log index for path: %s", path)
	return nil
}

// EnableLogIndex enables indexing for a log file
func (pm *PersistenceManager) EnableLogIndex(path string) error {
	db := model.UseDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	result := db.Model(&model.NginxLogIndex{}).Where("path = ?", path).Update("enabled", true)
	if result.Error != nil {
		return fmt.Errorf("failed to enable log index: %w", result.Error)
	}

	logger.Infof("Enabled log index for path: %s", path)
	return nil
}

// CleanupOldIndexes removes index records for files that haven't been indexed in a long time
func (pm *PersistenceManager) CleanupOldIndexes(maxAge time.Duration) error {
	db := model.UseDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	cutoff := time.Now().Add(-maxAge)
	result := db.Where("last_indexed < ?", cutoff).Delete(&model.NginxLogIndex{})
	if result.Error != nil {
		return fmt.Errorf("failed to cleanup old indexes: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		logger.Infof("Cleaned up %d old log index records", result.RowsAffected)
	}

	return nil
}

// GetIndexStats returns statistics about stored index records
func (pm *PersistenceManager) GetIndexStats() (map[string]interface{}, error) {
	db := model.UseDB()
	if db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var totalCount int64
	var enabledCount int64
	var totalDocs uint64

	// Count total records
	if err := db.Model(&model.NginxLogIndex{}).Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count total indexes: %w", err)
	}

	// Count enabled records
	if err := db.Model(&model.NginxLogIndex{}).Where("enabled = ?", true).Count(&enabledCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count enabled indexes: %w", err)
	}

	// Sum document counts
	var result struct {
		Total uint64
	}
	if err := db.Model(&model.NginxLogIndex{}).Select("COALESCE(SUM(document_count), 0) as total").Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to sum document counts: %w", err)
	}
	totalDocs = result.Total

	return map[string]interface{}{
		"total_files":    totalCount,
		"enabled_files":  enabledCount,
		"total_documents": totalDocs,
	}, nil
}