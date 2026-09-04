package indexer

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/0xJacky/Nginx-UI/internal/cgroup"
	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
	"github.com/uozi-tech/cosy/logger"
)

// logParser is the process-wide parser singleton, used for both batch and
// single-line parsing.
//
// It is an atomic pointer rather than a plain global guarded by sync.Once so
// that ReleaseLogParser can drop it: the parser owns a GeoIP handle and two
// 10,000-entry caches, and after a graceful handover the retired process would
// otherwise keep them reachable - and therefore resident - forever.
var (
	logParser    atomic.Pointer[parser.Parser]
	parserInitMu sync.Mutex
)

// getLogParser returns the current parser singleton, or nil when none is installed.
func getLogParser() *parser.Parser {
	return logParser.Load()
}

// geoIPOverride replaces the GeoLite-backed geo lookup when set.
//
// The slot defaults to nil, and only internal/demo ever fills it, so a
// production binary always resolves geo data from the real city database.
// Geo is baked into the indexed document, so this must be installed before
// InitLogParser runs.
var geoIPOverride parser.GeoIPService

// SetGeoIPService installs a geo lookup override. Call once, at boot, before
// any indexing starts.
func SetGeoIPService(service parser.GeoIPService) {
	geoIPOverride = service
}

// maxParserWorkerCount caps the per-file parse fan-out. Parsing is only one
// stage of the pipeline, and every worker keeps a parse buffer alive.
const maxParserWorkerCount = 8

// InitLogParser initializes the global parser once (singleton).
func InitLogParser() {
	parserInitMu.Lock()
	defer parserInitMu.Unlock()

	if logParser.Load() != nil {
		return
	}

	// Initialize the parser with production-ready configuration
	config := parser.DefaultParserConfig()
	config.MaxLineLength = 16 * 1024 // 16KB for large log lines
	config.BatchSize = 15000         // Maximum batch size for highest frontend throughput

	// Derive parser worker count from the CPUs this process may actually use,
	// with sane limits so that small machines are not overwhelmed while larger
	// hosts can still use parallel parsing effectively.
	//
	// cgroup.AvailableCPUs, not GOMAXPROCS: inside an LXC/Docker container the
	// affinity mask reports every host CPU while the cgroup bandwidth
	// controller throttles the process to a fraction of one, so GOMAXPROCS
	// would start up to 16 parse goroutines per file on a container that is
	// only allowed a single core.
	workerCount := cgroup.AvailableCPUs()
	if workerCount < 2 {
		workerCount = 2
	}
	if workerCount > maxParserWorkerCount {
		workerCount = maxParserWorkerCount
	}
	config.WorkerCount = workerCount
	// Note: Caching is handled by the CachedUserAgentParser

	// Initialize user agent parser with caching (10,000 cache size for production)
	uaParser := parser.NewCachedUserAgentParser(
		parser.NewSimpleUserAgentParser(),
		10000, // Large cache for production workloads
	)

	// Access logs repeat the same IPs heavily; cache lookups so the
	// per-line hot path avoids repeated GeoIP database queries
	var geoIPService parser.GeoIPService
	if geoIPOverride != nil {
		geoIPService = parser.NewCachedGeoIPService(geoIPOverride, 10000)
	} else if geoService, err := geolite.GetService(); err != nil {
		logger.Warnf("Failed to initialize GeoIP service, geo-enrichment will be disabled: %v", err)
	} else {
		geoIPService = parser.NewCachedGeoIPService(parser.NewGeoLiteAdapter(geoService), 10000)
	}

	// Create the parser with production configuration
	logParser.Store(parser.NewParser(config, uaParser, geoIPService))

	logger.Info("Nginx log processing optimization system initialized with production configuration")
}

// ReleaseLogParser drops the parser singleton and the GeoIP handle and caches
// it owns.
//
// After a graceful handover the retired process stays alive as a connection
// proxy for the new binary, so anything left reachable from a package global
// can never be collected. Releasing the parser lets that memory go back to the
// OS instead of doubling the resident set of the container for the lifetime of
// the process.
func ReleaseLogParser() {
	parserInitMu.Lock()
	defer parserInitMu.Unlock()
	logParser.Store(nil)
}

// IsLogParserInitialized returns true if the global parser singleton has been created.
func IsLogParserInitialized() bool {
	return getLogParser() != nil
}

// ParseLogLine parses a raw log line into a structured LogDocument using optimized parsing
func ParseLogLine(line string) (*LogDocument, error) {
	if line == "" {
		return nil, nil
	}

	activeParser := getLogParser()
	if activeParser == nil {
		return nil, ErrLogParserNotInitialized
	}

	// Use parser for single line processing
	entry, err := activeParser.ParseLine(line)
	if err != nil {
		return nil, err
	}

	return convertToLogDocument(entry, "", ""), nil
}

// ParseLogStreamBatches parses a stream of log data in bounded batches,
// invoking fn with each converted batch of LogDocuments as soon as it is
// ready. Only one batch is held in memory at a time, so peak memory stays
// bounded regardless of file size. Returns the number of processed and
// failed lines.
func ParseLogStreamBatches(ctx context.Context, reader io.Reader, filePath string, fn func(docs []*LogDocument) error) (processed, failed int, err error) {
	activeParser := getLogParser()
	if activeParser == nil {
		return 0, 0, ErrLogParserNotInitialized
	}

	// Auto-detect and handle gzip files
	actualReader, cleanup, err := createReaderForFile(reader, filePath)
	if err != nil {
		logger.Warnf("Error setting up reader for %s: %v", filePath, err)
		actualReader = reader // fallback to original reader
	}
	if cleanup != nil {
		defer cleanup()
	}

	// The main log path is constant for the whole file; compute it once
	mainLogPath := getMainLogPathFromFile(filePath)

	parseResult, err := activeParser.StreamParseBatches(ctx, actualReader, func(entries []*parser.AccessLogEntry) error {
		docs := make([]*LogDocument, 0, len(entries))
		for _, entry := range entries {
			docs = append(docs, convertToLogDocument(entry, filePath, mainLogPath))
		}
		return fn(docs)
	})
	if parseResult == nil {
		return 0, 0, err
	}

	return parseResult.Processed, parseResult.Failed, err
}

// ParseLogStream parses a stream of log data and returns all documents in
// one slice. Prefer ParseLogStreamBatches for whole-file indexing — this
// variant accumulates everything in memory and is only appropriate for
// bounded inputs such as incremental tails.
func ParseLogStream(ctx context.Context, reader io.Reader, filePath string) ([]*LogDocument, error) {
	var docs []*LogDocument
	processed, failed, err := ParseLogStreamBatches(ctx, reader, filePath, func(batch []*LogDocument) error {
		docs = append(docs, batch...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if processed > 0 {
		logger.Infof("ParseStream processed %d lines with %.2f%% error rate",
			processed, float64(failed)/float64(processed)*100)
	}

	return docs, nil
}

// convertToLogDocument converts parser.AccessLogEntry to indexer.LogDocument.
// mainLogPath is passed in by the caller: it is constant for a whole file, and
// this function runs once per log line.
func convertToLogDocument(entry *parser.AccessLogEntry, filePath, mainLogPath string) *LogDocument {
	// Convert parser.AccessLogEntry to indexer.LogDocument
	// This mapping is necessary because the indexer and parser might have different data structures.
	logDoc := &LogDocument{
		Timestamp:   entry.Timestamp,
		IP:          entry.IP,
		RegionCode:  entry.RegionCode,
		Province:    entry.Province,
		City:        entry.City,
		C1:          entry.C1,
		C2:          entry.C2,
		C3:          entry.C3,
		C4:          entry.C4,
		Method:      entry.Method,
		Path:        entry.Path,
		PathExact:   entry.Path, // Use the same for now
		Protocol:    entry.Protocol,
		Status:      entry.Status,
		BytesSent:   entry.BytesSent,
		Referer:     entry.Referer,
		UserAgent:   entry.UserAgent,
		Browser:     entry.Browser,
		BrowserVer:  entry.BrowserVer,
		OS:          entry.OS,
		OSVersion:   entry.OSVersion,
		DeviceType:  entry.DeviceType,
		RequestTime: entry.RequestTime,
		Raw:         entry.Raw,
		FilePath:    filePath,
		MainLogPath: mainLogPath,
	}

	if entry.UpstreamTime != nil {
		logDoc.UpstreamTime = entry.UpstreamTime
	}

	return logDoc
}

// createReaderForFile creates appropriate reader for the file, with gzip detection
func createReaderForFile(reader io.Reader, filePath string) (io.Reader, func(), error) {
	// If not a .gz file, return as-is
	if !strings.HasSuffix(filePath, ".gz") {
		return reader, nil, nil
	}

	// For .gz files, try to detect if it's actually gzip compressed
	bufferedReader := bufio.NewReader(reader)

	// Peek at first 2 bytes to check for gzip magic number (0x1f, 0x8b)
	header, err := bufferedReader.Peek(2)
	if err != nil {
		logger.Warnf("Cannot peek header for %s: %v, treating as plain text", filePath, err)
		return bufferedReader, nil, nil
	}

	// Check for gzip magic number
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		// It's a valid gzip file
		gzReader, err := gzip.NewReader(bufferedReader)
		if err != nil {
			logger.Warnf("Failed to create gzip reader for %s despite valid header: %v, treating as plain text", filePath, err)
			return bufferedReader, nil, nil
		}

		return gzReader, func() { gzReader.Close() }, nil
	} else {
		// No gzip magic header: either the stream was already decompressed by
		// the caller (e.g. incremental indexing pre-decompresses to skip to a
		// byte position) or the file is mislabeled. Both are handled as plain text.
		logger.Debugf("File %s has .gz extension but stream has no gzip magic header, treating as plain text", filePath)
		return bufferedReader, nil, nil
	}
}
