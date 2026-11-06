package indexer

import (
	"context"
	"time"
)

// LogIndexer is what the services, the cron jobs and the API need from an
// indexer. ColumnarIndexer is the implementation in use; ParallelIndexer is
// the Bleve implementation it replaces.
type LogIndexer interface {
	Start(ctx context.Context) error
	Stop() error
	IsHealthy() bool
	IsRunning() bool
	GetConfig() *Config
	GetStats() *IndexStats
	FlushAll() error

	StartBatch() BatchWriterInterface
	IndexLogGroupWithProgress(basePath string, progressConfig *ProgressConfig) (map[string]uint64, *time.Time, *time.Time, error)
	IndexSingleFile(filePath string) (uint64, *time.Time, *time.Time, error)
	IndexSingleFileWithProgress(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error)

	DeleteIndexByLogGroup(basePath string, logFileManager interface{}) error
	DeleteIndexByFiles(filesToDelete []string) error
	DeleteIndexByMainLogPath(mainLogPath string) error
	DestroyAllIndexes(ctx context.Context) error

	CountDocsByMainLogPath(basePath string) (uint64, error)
	CountDocsByFilePath(filePath string) (uint64, error)

	// takeReadPosition hands LogFileManager the offset a full read of a file
	// stopped at, so it can persist LastPosition.
	takeReadPosition(filePath string) (int64, bool)
}

var (
	_ LogIndexer = (*ParallelIndexer)(nil)
	_ LogIndexer = (*ColumnarIndexer)(nil)
)
