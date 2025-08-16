package nginx_log

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/uozi-tech/cosy/logger"
)

// BackgroundLogService manages automatic log discovery and indexing
type BackgroundLogService struct {
	indexer *LogIndexer
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewBackgroundLogService creates a new background log service
func NewBackgroundLogService() (*BackgroundLogService, error) {
	indexer, err := NewLogIndexer()
	if err != nil {
		return nil, err
	}

	service := &BackgroundLogService{
		indexer: indexer,
	}

	return service, nil
}

// Start begins the background log discovery and indexing process
func (s *BackgroundLogService) Start() {
	logger.Info("Starting background log service")

	// Initialize analytics service and set indexer
	InitAnalyticsService()
	SetAnalyticsServiceIndexer(s.indexer)

	// Initialize Bleve stats service
	InitBleveStatsService()
	SetBleveStatsServiceIndexer(s.indexer)

	// Load existing log indexes from database
	go s.loadExistingIndexes()

	// Start periodic log discovery
	go s.periodicLogDiscovery()

	// Discover initial log files
	go s.discoverInitialLogs()
}

// Stop stops the background log service
func (s *BackgroundLogService) Stop() {
	logger.Info("Stopping background log service")

	if s.cancel != nil {
		s.cancel()
	}

	if s.indexer != nil {
		s.indexer.Close()
	}
}

// GetIndexer returns the log indexer instance
func (s *BackgroundLogService) GetIndexer() *LogIndexer {
	return s.indexer
}

// discoverInitialLogs discovers and indexes initial log files
func (s *BackgroundLogService) discoverInitialLogs() {
	logger.Info("Starting initial log discovery")

	// Get access log path
	accessLogPath := nginx.GetAccessLogPath()
	if accessLogPath != "" && IsLogPathUnderWhiteList(accessLogPath) {
		logger.Infof("Discovering access logs from: %s", accessLogPath)
		s.discoverLogFiles(accessLogPath)
	} else {
		logger.Warn("Access log path not available or not in whitelist")
	}

	// Skip error logs - they have different format and are not indexed for structured search

	logger.Info("Initial log discovery completed")
}

// periodicLogDiscovery runs periodic log discovery
func (s *BackgroundLogService) periodicLogDiscovery() {
	ticker := time.NewTicker(30 * time.Minute) // Check every 30 minutes
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			logger.Info("Periodic log discovery stopping")
			return
		case <-ticker.C:
			logger.Debug("Running periodic log discovery")
			s.discoverInitialLogs()
		}
	}
}

// discoverLogFiles discovers log files for a given log path
func (s *BackgroundLogService) discoverLogFiles(logPath string) {
	if s.indexer == nil {
		logger.Error("Log indexer not available")
		return
	}

	logDir := filepath.Dir(logPath)
	baseLogName := filepath.Base(logPath)

	logger.Debugf("Discovering log files in %s with base name %s", logDir, baseLogName)

	if err := s.indexer.DiscoverLogFiles(logDir, baseLogName); err != nil {
		logger.Errorf("Failed to discover log files for %s: %v", logPath, err)
	} else {
		logger.Debugf("Successfully discovered log files for %s (queued for indexing)", logPath)
		// Note: Index ready notification will be sent after actual indexing is complete
	}
}

// loadExistingIndexes loads log file paths from the database and sets up monitoring
func (s *BackgroundLogService) loadExistingIndexes() {
	logger.Info("Loading existing log indexes from database")

	if s.indexer == nil {
		logger.Error("Log indexer not available for loading existing indexes")
		return
	}

	persistence := NewPersistenceManager()
	indexes, err := persistence.GetAllLogIndexes()
	if err != nil {
		logger.Errorf("Failed to load existing log indexes: %v", err)
		return
	}

	logger.Infof("Found %d existing log indexes in database", len(indexes))

	// If no indexes found in database but we should have log files, discover them
	if len(indexes) == 0 {
		logger.Warnf("No existing log indexes found in database, running initial log discovery")
		s.discoverInitialLogs()
		return
	}

	for _, logIndex := range indexes {
		// Check if file still exists
		if _, err := os.Stat(logIndex.Path); os.IsNotExist(err) {
			logger.Warnf("Log file no longer exists, skipping: %s", logIndex.Path)
			continue
		}

		logger.Infof("Loading existing log index: %s", logIndex.Path)

		// Add to indexer (this will set up monitoring and check for updates)
		if err := s.indexer.AddLogPath(logIndex.Path); err != nil {
			logger.Errorf("Failed to add existing log path %s: %v", logIndex.Path, err)
			continue
		}

		// Check if file needs reindexing (only if file has changed since last index)
		if !logIndex.LastIndexed.IsZero() {
			fileInfo, err := os.Stat(logIndex.Path)
			if err == nil {
				// Check if file has been modified or if log group index size has changed since last index
				totalSize := s.indexer.calculateRelatedLogFilesSize(logIndex.Path)
				needsReindex := fileInfo.ModTime().After(logIndex.LastModified) ||
					totalSize != logIndex.LastSize

				if needsReindex {
					logger.Infof("File %s has changed since last index, queuing incremental update", logIndex.Path)
					// Queue for incremental indexing
					if err := s.indexer.IndexLogFileWithMode(logIndex.Path, false); err != nil {
						logger.Errorf("Failed to queue incremental index for %s: %v", logIndex.Path, err)
					}
				} else {
					logger.Infof("File %s unchanged since last index, skipping", logIndex.Path)
				}
			}
		}

		logger.Infof("Successfully loaded log index: %s", logIndex.Path)
	}

	logger.Infof("Finished loading existing log indexes")
}

// Global background service instance
var backgroundService *BackgroundLogService

// InitBackgroundLogService initializes the global background log service
func InitBackgroundLogService(ctx context.Context) error {
	var err error
	backgroundService, err = NewBackgroundLogService()
	if err != nil {
		return err
	}

	// Use the provided context instead of creating a new one
	backgroundService.ctx, backgroundService.cancel = context.WithCancel(ctx)
	backgroundService.Start()
	return nil
}

// GetBackgroundLogService returns the global background service instance
func GetBackgroundLogService() *BackgroundLogService {
	return backgroundService
}

// StopBackgroundLogService stops the global background log service
func StopBackgroundLogService() {
	if backgroundService != nil {
		backgroundService.Stop()
		backgroundService = nil
	}
}
