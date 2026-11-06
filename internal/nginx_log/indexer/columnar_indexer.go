package indexer

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
	"github.com/uozi-tech/cosy/logger"
)

// logstoreDir is the store's directory under the index path.
const logstoreDir = "logstore"

// ColumnarIndexer writes parsed access log lines to a logstore.Store. See
// internal/nginx_log/logstore/DESIGN.md for why it replaces the Bleve
// ParallelIndexer.
//
// There is no worker pool, job queue or memory quota: a file's rows are
// encoded into blocks as they are parsed, and the memory a file being indexed
// holds is bounded by one block.
type ColumnarIndexer struct {
	config *Config

	mu      sync.RWMutex // guards store
	store   *logstore.Store
	running atomic.Bool

	cancel context.CancelFunc
	wg     sync.WaitGroup

	optimizing    atomic.Bool
	lastOptimized atomic.Int64
	docsIndexed   atomic.Int64
	startedAt     time.Time

	readPositions   map[string]int64
	readPositionsMu sync.Mutex
}

// NewColumnarIndexer creates an indexer that stores rows under
// config.IndexPath/logstore. Call Start before use.
func NewColumnarIndexer(config *Config) *ColumnarIndexer {
	if config == nil {
		config = DefaultIndexerConfig()
	}
	return &ColumnarIndexer{config: config, readPositions: make(map[string]int64)}
}

// StoreDir returns where the store of an index path lives.
func StoreDir(indexPath string) string {
	return filepath.Join(indexPath, logstoreDir)
}

// Start opens the store. Starting a running indexer is a no-op, so the
// rebuild flow can call it again after DestroyAllIndexes.
func (ci *ColumnarIndexer) Start(ctx context.Context) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	if ci.running.Load() {
		return nil
	}

	store, err := logstore.Open(StoreDir(ci.config.IndexPath), logstore.Options{
		Parallelism: max(availableCPUs(), 1),
	})
	if err != nil {
		return fmt.Errorf("open log store: %w", err)
	}
	ci.store = store
	ci.startedAt = time.Now()
	ci.running.Store(true)

	if ci.config.OptimizeInterval > 0 {
		routineCtx, cancel := context.WithCancel(ctx)
		ci.cancel = cancel
		ci.wg.Add(1)
		go ci.compactionRoutine(routineCtx)
	}
	return nil
}

// Stop closes the store.
func (ci *ColumnarIndexer) Stop() error {
	ci.mu.Lock()
	if !ci.running.Swap(false) {
		ci.mu.Unlock()
		return fmt.Errorf("indexer already stopped")
	}
	if ci.cancel != nil {
		ci.cancel()
	}
	store := ci.store
	ci.mu.Unlock()

	ci.wg.Wait()
	return store.Close()
}

// Store returns the store the indexer writes to, or nil before Start.
func (ci *ColumnarIndexer) Store() *logstore.Store {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	return ci.store
}

func (ci *ColumnarIndexer) liveStore() (*logstore.Store, error) {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	if !ci.running.Load() || ci.store == nil {
		return nil, fmt.Errorf("indexer not healthy")
	}
	return ci.store, nil
}

// IsHealthy reports whether the store is open.
func (ci *ColumnarIndexer) IsHealthy() bool { return ci.running.Load() }

// IsRunning reports whether the store is open.
func (ci *ColumnarIndexer) IsRunning() bool { return ci.running.Load() }

// GetConfig returns the configuration.
func (ci *ColumnarIndexer) GetConfig() *Config { return ci.config }

// FlushAll is a no-op: every batch is flushed by the read that wrote it.
func (ci *ColumnarIndexer) FlushAll() error {
	if !ci.running.Load() {
		return fmt.Errorf("indexer not running")
	}
	return nil
}

// fileConcurrency is how many files of one log group are indexed at once.
func (ci *ColumnarIndexer) fileConcurrency() int {
	if ci.config.FileGroupConcurrency > 0 {
		return ci.config.FileGroupConcurrency
	}
	return max(ci.config.WorkerCount, 1)
}

// StartBatch returns a batch that writes each document to the segment of its
// physical file.
func (ci *ColumnarIndexer) StartBatch() BatchWriterInterface {
	return &storeBatch{ci: ci, writers: map[string]*logstore.Writer{}}
}

// IndexDocuments writes documents and flushes them.
func (ci *ColumnarIndexer) IndexDocuments(_ context.Context, docs []*Document) error {
	batch := ci.StartBatch()
	for _, doc := range docs {
		if err := batch.Add(doc); err != nil {
			return err
		}
	}
	_, err := batch.Flush()
	return err
}

// IndexLogGroupWithProgress brings the store up to date with every file of a
// log group. Content read before is read on from where it stopped, wherever
// rotation moved it, and the rows of content whose file is gone are dropped.
// The counts are the documents added by this call.
func (ci *ColumnarIndexer) IndexLogGroupWithProgress(basePath string, progressConfig *ProgressConfig) (map[string]uint64, *time.Time, *time.Time, error) {
	if !ci.IsHealthy() {
		return nil, nil, nil, fmt.Errorf("indexer not healthy")
	}
	counts, minTime, maxTime, err := indexLogGroupFiles(basePath, progressConfig, ci.fileConcurrency(), ci.syncFile)
	// Only a sync of a whole group knows which of its contents are gone.
	if err == nil && getMainLogPathFromFile(basePath) == basePath {
		if files, listErr := groupFiles(basePath); listErr == nil && len(files) > 0 {
			ci.pruneVanished(basePath, files)
		}
	}
	return counts, minTime, maxTime, err
}

// IndexSingleFile indexes one physical file.
func (ci *ColumnarIndexer) IndexSingleFile(filePath string) (uint64, *time.Time, *time.Time, error) {
	return ci.IndexSingleFileWithProgress(filePath, nil)
}

// IndexSingleFileWithProgress brings the store up to date with one physical
// file, reading only what the store has not seen of its content.
func (ci *ColumnarIndexer) IndexSingleFileWithProgress(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error) {
	if !ci.IsHealthy() {
		return 0, nil, nil, fmt.Errorf("indexer not healthy")
	}
	return ci.syncFile(filePath, progressTracker)
}

// DeleteIndexByLogGroup drops every file of a log group: all segments
// written under the group, plus any file the metadata lists for it.
func (ci *ColumnarIndexer) DeleteIndexByLogGroup(basePath string, logFileManager interface{}) error {
	store, err := ci.liveStore()
	if err != nil {
		return err
	}
	var files []string
	if lfm, ok := logFileManager.(GroupFileProvider); ok {
		if files, err = lfm.GetFilePathsForGroup(basePath); err != nil {
			return fmt.Errorf("failed to get file paths for log group %s: %w", basePath, err)
		}
	}
	for _, seg := range store.Segments(basePath) {
		files = append(files, seg.Path())
	}
	store.DeleteGroup(basePath)
	for _, f := range files {
		store.DeleteFile(f)
	}
	ci.forgetReadPositions(files)
	logger.Infof("Deleted the index of log group %s", basePath)
	return nil
}

// DeleteIndexByFiles drops everything ingested from the given files.
func (ci *ColumnarIndexer) DeleteIndexByFiles(filesToDelete []string) error {
	store, err := ci.liveStore()
	if err != nil {
		return err
	}
	for _, f := range filesToDelete {
		store.DeleteFile(f)
	}
	ci.forgetReadPositions(filesToDelete)
	return nil
}

// DeleteIndexByMainLogPath drops every file of a log group.
func (ci *ColumnarIndexer) DeleteIndexByMainLogPath(mainLogPath string) error {
	store, err := ci.liveStore()
	if err != nil {
		return err
	}
	var files []string
	for _, seg := range store.Segments(mainLogPath) {
		files = append(files, seg.Path())
	}
	store.DeleteGroup(mainLogPath)
	ci.forgetReadPositions(files)
	return nil
}

// DestroyAllIndexes drops every segment. The indexer keeps running.
func (ci *ColumnarIndexer) DestroyAllIndexes(context.Context) error {
	store, err := ci.liveStore()
	if err != nil {
		return err
	}
	store.DeleteAll()
	ci.readPositionsMu.Lock()
	ci.readPositions = make(map[string]int64)
	ci.readPositionsMu.Unlock()
	return nil
}

// CountDocsByMainLogPath returns the rows stored for a log group.
func (ci *ColumnarIndexer) CountDocsByMainLogPath(basePath string) (uint64, error) {
	store, err := ci.liveStore()
	if err != nil {
		return 0, err
	}
	var total uint64
	for _, seg := range store.Segments(basePath) {
		total += uint64(seg.Info().Rows)
	}
	return total, nil
}

// CountDocsByFilePath returns the rows stored for one physical file.
func (ci *ColumnarIndexer) CountDocsByFilePath(filePath string) (uint64, error) {
	store, err := ci.liveStore()
	if err != nil {
		return 0, err
	}
	if seg := store.Segment(filePath); seg != nil {
		return uint64(seg.Info().Rows), nil
	}
	return 0, nil
}

// Optimize merges the small blocks incremental runs leave behind.
func (ci *ColumnarIndexer) Optimize() error {
	store, err := ci.liveStore()
	if err != nil {
		return err
	}
	if !ci.optimizing.CompareAndSwap(false, true) {
		return fmt.Errorf("optimization already in progress")
	}
	defer ci.optimizing.Store(false)

	var errs []error
	for _, seg := range store.Segments("") {
		if _, err := store.CompactSegment(seg); err != nil {
			errs = append(errs, fmt.Errorf("compact %s: %w", seg.Path(), err))
		}
	}
	ci.lastOptimized.Store(time.Now().Unix())
	return errors.Join(errs...)
}

func (ci *ColumnarIndexer) compactionRoutine(ctx context.Context) {
	defer ci.wg.Done()
	ticker := time.NewTicker(ci.config.OptimizeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := ci.Optimize(); err != nil {
				logger.Warnf("Log store compaction: %v", err)
			}
		}
	}
}

// GetStats reports what the store holds.
func (ci *ColumnarIndexer) GetStats() *IndexStats {
	stats := &IndexStats{LastOptimized: ci.lastOptimized.Load()}
	store, err := ci.liveStore()
	if err != nil {
		return stats
	}
	for i, seg := range store.Segments("") {
		info := seg.Info()
		stats.TotalDocuments += uint64(info.Rows)
		stats.TotalSize += info.Bytes
		stats.Shards = append(stats.Shards, &ShardInfo{
			ID: i, Path: info.File, DocumentCount: uint64(info.Rows), Size: info.Bytes, LastUpdated: info.MaxTs,
		})
	}
	stats.ShardCount = len(stats.Shards)
	if elapsed := time.Since(ci.startedAt).Seconds(); elapsed > 0 {
		stats.IndexingRate = float64(ci.docsIndexed.Load()) / elapsed
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	stats.MemoryUsage = int64(mem.Alloc)
	return stats
}

func (ci *ColumnarIndexer) recordReadPosition(filePath string, position int64) {
	ci.readPositionsMu.Lock()
	defer ci.readPositionsMu.Unlock()
	ci.readPositions[filePath] = position
}

func (ci *ColumnarIndexer) takeReadPosition(filePath string) (int64, bool) {
	ci.readPositionsMu.Lock()
	defer ci.readPositionsMu.Unlock()
	position, ok := ci.readPositions[filePath]
	if ok {
		delete(ci.readPositions, filePath)
	}
	return position, ok
}

func (ci *ColumnarIndexer) forgetReadPositions(filePaths []string) {
	ci.readPositionsMu.Lock()
	defer ci.readPositionsMu.Unlock()
	for _, f := range filePaths {
		delete(ci.readPositions, f)
	}
}

// storeBatch routes documents to one logstore.Writer per physical file.
// Rows become visible to queries as each writer seals a block and at Flush.
type storeBatch struct {
	ci      *ColumnarIndexer
	writers map[string]*logstore.Writer
	pending int
}

func (b *storeBatch) writer(doc *LogDocument) (*logstore.Writer, error) {
	if w := b.writers[doc.FilePath]; w != nil {
		return w, nil
	}
	store, err := b.ci.liveStore()
	if err != nil {
		return nil, err
	}
	group := doc.MainLogPath
	if group == "" {
		group = getMainLogPathFromFile(doc.FilePath)
	}
	w, err := store.Writer(group, doc.FilePath)
	if err != nil {
		return nil, err
	}
	b.writers[doc.FilePath] = w
	return w, nil
}

// Add buffers a document. Its Fields must carry FilePath.
func (b *storeBatch) Add(doc *Document) error {
	if doc == nil || doc.Fields == nil || doc.Fields.FilePath == "" {
		return fmt.Errorf("%s: document without a file path", ErrInvalidDocument)
	}
	w, err := b.writer(doc.Fields)
	if err != nil {
		return err
	}
	if err := w.Append(LogDocumentToRow(doc.Fields), 0); err != nil {
		return err
	}
	b.pending++
	b.ci.docsIndexed.Add(1)
	return nil
}

// SetSourceEnd records how far filePath was read, for the rows buffered now.
func (b *storeBatch) SetSourceEnd(filePath string, offset int64) {
	if w := b.writers[filePath]; w != nil {
		w.SetSourceEnd(offset)
	}
}

// Flush seals every writer's buffered rows.
func (b *storeBatch) Flush() (*IndexResult, error) {
	start := time.Now()
	processed := b.pending
	var errs []error
	for _, w := range b.writers {
		if err := w.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	b.pending = 0
	result := &IndexResult{Processed: processed, Duration: time.Since(start)}
	if err := errors.Join(errs...); err != nil {
		result.Failed = processed
		result.ErrorRate = 1
		return result, err
	}
	result.Succeeded = processed
	if secs := result.Duration.Seconds(); secs > 0 {
		result.Throughput = float64(processed) / secs
	}
	return result, nil
}

// Size reports the documents added since the last Flush.
func (b *storeBatch) Size() int { return b.pending }

// Reset discards everything this batch wrote, as Abort does.
func (b *storeBatch) Reset() { _ = b.Abort() }

// Abort removes everything this batch wrote, flushed or not.
func (b *storeBatch) Abort() error {
	var errs []error
	for _, w := range b.writers {
		errs = append(errs, w.Abort())
	}
	b.writers = map[string]*logstore.Writer{}
	b.pending = 0
	return errors.Join(errs...)
}
