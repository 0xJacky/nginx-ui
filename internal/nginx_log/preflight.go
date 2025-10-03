package nginx_log

import (
	"fmt"
	"os"

	"github.com/0xJacky/Nginx-UI/internal/event"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// FileInfo represents basic file information for internal use
type FileInfo struct {
	Exists       bool  `json:"exists"`
	Readable     bool  `json:"readable"`
	Size         int64 `json:"size,omitempty"`
	LastModified int64 `json:"last_modified,omitempty"`
}

// TimeRange represents a time range for log data for internal use
type TimeRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// PreflightResponse represents the response from preflight checks for internal use
type PreflightResponse struct {
	Available   bool       `json:"available"`
	IndexStatus string     `json:"index_status"`
	Message     string     `json:"message,omitempty"`
	TimeRange   *TimeRange `json:"time_range,omitempty"`
	FileInfo    *FileInfo  `json:"file_info,omitempty"`
}

// Preflight handles preflight checks for log files
type Preflight struct{}

// NewPreflight creates a new preflight service
func NewPreflight() *Preflight {
	return &Preflight{}
}

// CheckLogPreflight performs preflight checks for a log file
func (ps *Preflight) CheckLogPreflight(logPath string) (*PreflightResponse, error) {
	// Use default access log path if logPath is empty
	if logPath == "" {
		defaultLogPath := nginx.GetAccessLogPath()
		if defaultLogPath != "" {
			logPath = defaultLogPath
			logger.Debugf("Using default access log path for preflight: %s", logPath)
		}
	}

	// Get searcher to check index status
	searcherService := GetSearcher()
	if searcherService == nil {
		return nil, ErrModernSearcherNotAvailable
	}

	// Check if the specific file is currently being indexed
	processingManager := event.GetProcessingStatusManager()
	currentStatus := processingManager.GetCurrentStatus()

	// First check if the file exists and get file info
	var fileInfo *os.FileInfo
	if logPath != "" {
		// Validate log path before accessing it
		if !utils.IsValidLogPath(logPath) {
			return &PreflightResponse{
				Available:   false,
				IndexStatus: string(indexer.IndexStatusError),
				Message:     fmt.Sprintf("Invalid log path: %s", logPath),
				FileInfo: &FileInfo{
					Exists:   false,
					Readable: false,
				},
			}, nil
		}

		if stat, err := os.Stat(logPath); os.IsNotExist(err) {
			// File doesn't exist - check for historical data
			return ps.handleMissingFile(logPath, searcherService)
		} else if err != nil {
			// Permission or other file system error - map to error status
			return &PreflightResponse{
				Available:   false,
				IndexStatus: string(indexer.IndexStatusError),
				Message:     fmt.Sprintf("Cannot access log file %s: %v", logPath, err),
				FileInfo: &FileInfo{
					Exists:   true,
					Readable: false,
				},
			}, nil
		} else {
			fileInfo = &stat
		}
	}

	// Check if searcher is healthy
	searcherHealthy := searcherService.IsHealthy()

	// Get detailed file status from log file manager
	return ps.buildPreflightResponse(logPath, fileInfo, searcherHealthy, &currentStatus)
}

// handleMissingFile handles the case when a log file doesn't exist
func (ps *Preflight) handleMissingFile(logPath string, searcherService *searcher.Searcher) (*PreflightResponse, error) {
	searcherHealthy := searcherService.IsHealthy()
	logFileManager := GetLogFileManager()

	if logFileManager != nil {
		logGroup, err := logFileManager.GetLogByPath(logPath)
		if err == nil && logGroup != nil && logGroup.LastIndexed > 0 {
			// File has historical index data
			response := &PreflightResponse{
				Available:   searcherHealthy,
				IndexStatus: string(indexer.IndexStatusIndexed),
				Message:     "File indexed (historical data available)",
				FileInfo: &FileInfo{
					Exists:   false,
					Readable: false,
				},
			}
			if logGroup.HasTimeRange {
				response.TimeRange = &TimeRange{
					Start: logGroup.TimeRangeStart,
					End:   logGroup.TimeRangeEnd,
				}
			}
			return response, nil
		}
	}

	// File doesn't exist and no historical data
	return &PreflightResponse{
		Available:   false,
		IndexStatus: string(indexer.IndexStatusNotIndexed),
		Message:     "Log file does not exist",
		FileInfo: &FileInfo{
			Exists:   false,
			Readable: false,
		},
	}, nil
}

// buildPreflightResponse builds the preflight response for existing files
func (ps *Preflight) buildPreflightResponse(logPath string, fileInfo *os.FileInfo, searcherHealthy bool, currentStatus *event.ProcessingStatusData) (*PreflightResponse, error) {
	logFileManager := GetLogFileManager()
	var indexStatus string = string(indexer.IndexStatusNotIndexed)
	var available bool = false

	response := &PreflightResponse{}

	if logFileManager != nil && logPath != "" {
		logGroup, err := logFileManager.GetLogByPath(logPath)
		if err == nil && logGroup != nil {
			// Determine status based on indexing state
			if logGroup.LastIndexed > 0 {
				indexStatus = string(indexer.IndexStatusIndexed)
				available = searcherHealthy
			} else if currentStatus.NginxLogIndexing {
				indexStatus = string(indexer.IndexStatusIndexing)
				available = false
			} else {
				indexStatus = string(indexer.IndexStatusNotIndexed)
				available = false
			}

			response.Available = available
			response.IndexStatus = indexStatus

			// Add time range if available
			if logGroup.HasTimeRange {
				response.TimeRange = &TimeRange{
					Start: logGroup.TimeRangeStart,
					End:   logGroup.TimeRangeEnd,
				}
			}
		} else {
			// File not in database or error getting it
			if currentStatus.NginxLogIndexing {
				indexStatus = string(indexer.IndexStatusQueued)
			} else {
				indexStatus = string(indexer.IndexStatusNotIndexed)
			}
			available = false

			response.Available = available
			response.IndexStatus = indexStatus
			response.Message = "Log file not indexed yet"
		}
	} else {
		// Fallback to basic status
		response.Available = searcherHealthy
		response.IndexStatus = string(indexer.IndexStatusNotIndexed)
	}

	// Add file information if available
	if fileInfo != nil {
		response.FileInfo = &FileInfo{
			Exists:       true,
			Readable:     true,
			Size:         (*fileInfo).Size(),
			LastModified: (*fileInfo).ModTime().Unix(),
		}
	}

	logger.Debugf("Preflight response: log_path=%s, available=%v, index_status=%s",
		logPath, response.Available, response.IndexStatus)

	return response, nil
}