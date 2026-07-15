package indexer

import (
	"bufio"
	"compress/gzip"
	"context"
	"io"
	"runtime"
	"strings"
	"sync"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
	"github.com/uozi-tech/cosy/logger"
)

// Global parser instances
var (
	logParser      *parser.Parser // Use the concrete type for both regular and single-line parsing
	parserInitOnce sync.Once
)

// InitLogParser initializes the global parser once (singleton).
func InitLogParser() {
	parserInitOnce.Do(func() {
		// Initialize the parser with production-ready configuration
		config := parser.DefaultParserConfig()
		config.MaxLineLength = 16 * 1024 // 16KB for large log lines
		config.BatchSize = 15000         // Maximum batch size for highest frontend throughput

		// Derive parser worker count from available CPUs, with sane limits so that
		// small machines are not overwhelmed while larger hosts can still use
		// parallel parsing effectively.
		maxProcs := runtime.GOMAXPROCS(0)
		if maxProcs <= 0 {
			maxProcs = runtime.NumCPU()
		}
		workerCount := maxProcs
		if workerCount < 4 {
			workerCount = 4
		}
		if workerCount > 16 {
			workerCount = 16
		}
		config.WorkerCount = workerCount
		// Note: Caching is handled by the CachedUserAgentParser

		// Initialize user agent parser with caching (10,000 cache size for production)
		uaParser := parser.NewCachedUserAgentParser(
			parser.NewSimpleUserAgentParser(),
			10000, // Large cache for production workloads
		)

		var geoIPService parser.GeoIPService
		geoService, err := geolite.GetService()
		if err != nil {
			logger.Warnf("Failed to initialize GeoIP service, geo-enrichment will be disabled: %v", err)
		} else {
			geoIPService = parser.NewGeoLiteAdapter(geoService)
		}

		// Create the parser with production configuration
		logParser = parser.NewParser(config, uaParser, geoIPService)

		logger.Info("Nginx log processing optimization system initialized with production configuration")
	})
}

// IsLogParserInitialized returns true if the global parser singleton has been created.
func IsLogParserInitialized() bool {
	return logParser != nil
}

// ParseLogLine parses a raw log line into a structured LogDocument using optimized parsing
func ParseLogLine(line string) (*LogDocument, error) {
	if line == "" {
		return nil, nil
	}

	if logParser == nil {
		return nil, ErrLogParserNotInitialized
	}

	// Use parser for single line processing
	entry, err := logParser.ParseLine(line)
	if err != nil {
		return nil, err
	}

	return convertToLogDocument(entry, "", ""), nil
}

// ParseLogStream parses a stream of log data using ParseStream (7-8x faster)
func ParseLogStream(ctx context.Context, reader io.Reader, filePath string) ([]*LogDocument, error) {
	if logParser == nil {
		return nil, ErrLogParserNotInitialized
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

	// Use ParseStream for batch processing with 70% memory reduction
	parseResult, err := logParser.StreamParse(ctx, actualReader)
	if err != nil {
		return nil, err
	}

	// Convert to LogDocument format; the main log path is constant per file
	mainLogPath := getMainLogPathFromFile(filePath)
	docs := make([]*LogDocument, 0, len(parseResult.Entries))
	for _, entry := range parseResult.Entries {
		logDoc := convertToLogDocument(entry, filePath, mainLogPath)
		docs = append(docs, logDoc)
	}

	logger.Infof("ParseStream processed %d lines with %.2f%% error rate",
		parseResult.Processed, parseResult.ErrorRate*100)

	return docs, nil
}

// ParseLogStreamChunked processes large files using chunked processing for memory efficiency
func ParseLogStreamChunked(ctx context.Context, reader io.Reader, filePath string, chunkSize int) ([]*LogDocument, error) {
	if logParser == nil {
		return nil, ErrLogParserNotInitialized
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

	// Use ChunkedParseStream for large files with controlled memory usage
	parseResult, err := logParser.ChunkedParseStream(ctx, actualReader, chunkSize)
	if err != nil {
		return nil, err
	}

	mainLogPath := getMainLogPathFromFile(filePath)
	docs := make([]*LogDocument, 0, len(parseResult.Entries))
	for _, entry := range parseResult.Entries {
		logDoc := convertToLogDocument(entry, filePath, mainLogPath)
		docs = append(docs, logDoc)
	}

	return docs, nil
}

// ParseLogStreamMemoryEfficient uses memory-efficient parsing for low memory environments
func ParseLogStreamMemoryEfficient(ctx context.Context, reader io.Reader, filePath string) ([]*LogDocument, error) {
	if logParser == nil {
		return nil, ErrLogParserNotInitialized
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

	// Use MemoryEfficientParseStream for minimal memory usage
	parseResult, err := logParser.MemoryEfficientParseStream(ctx, actualReader)
	if err != nil {
		return nil, err
	}

	mainLogPath := getMainLogPathFromFile(filePath)
	docs := make([]*LogDocument, 0, len(parseResult.Entries))
	for _, entry := range parseResult.Entries {
		logDoc := convertToLogDocument(entry, filePath, mainLogPath)
		docs = append(docs, logDoc)
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
