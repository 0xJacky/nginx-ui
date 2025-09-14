package parser

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"runtime"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

// OptimizedParser provides high-performance log parsing with zero-copy optimizations
type OptimizedParser struct {
	config     *Config
	uaParser   UserAgentParser
	geoService GeoIPService
	pool       *sync.Pool
	detector   *FormatDetector
	stats      *ParseStats
	mu         sync.RWMutex
}

// ParseStats tracks parsing performance metrics
type ParseStats struct {
	TotalLines     int64
	SuccessLines   int64
	ErrorLines     int64
	TotalBytes     int64
	ParseDuration  time.Duration
	LinesPerSecond float64
	BytesPerSecond float64
	LastUpdated    time.Time
}

// parseBuffer holds reusable parsing buffers
type parseBuffer struct {
	fields    [][]byte
	entry     *AccessLogEntry
	lineBytes []byte
}

// NewOptimizedParser creates a new high-performance parser
func NewOptimizedParser(config *Config, uaParser UserAgentParser, geoService GeoIPService) *OptimizedParser {
	if config == nil {
		config = DefaultParserConfig()
	}

	return &OptimizedParser{
		config:     config,
		uaParser:   uaParser,
		geoService: geoService,
		detector:   NewFormatDetector(),
		stats:      &ParseStats{},
		pool: &sync.Pool{
			New: func() interface{} {
				return &parseBuffer{
					fields:    make([][]byte, 0, 16),
					entry:     &AccessLogEntry{},
					lineBytes: make([]byte, 0, config.MaxLineLength),
				}
			},
		},
	}
}

// ParseLine parses a single log line with zero-copy optimizations
func (p *OptimizedParser) ParseLine(line string) (*AccessLogEntry, error) {
	if len(line) == 0 {
		return nil, errors.New(ErrEmptyLogLine)
	}

	if len(line) > p.config.MaxLineLength {
		return nil, errors.New(ErrLineTooLong)
	}

	buf := p.pool.Get().(*parseBuffer)
	defer p.pool.Put(buf)

	// Reset buffer state
	buf.fields = buf.fields[:0]
	*buf.entry = AccessLogEntry{}

	// Zero-copy conversion to bytes
	buf.lineBytes = stringToBytes(line)

	if err := p.parseLineOptimized(buf.lineBytes, buf); err != nil {
		if p.config.StrictMode {
			return nil, err
		}
		// In non-strict mode, create a minimal entry with raw line
		buf.entry.Raw = line
		buf.entry.Timestamp = time.Now().Unix()
		buf.entry.ID = p.generateEntryID(line)
		// Create a copy to avoid sharing the pooled object
		entryCopy := *buf.entry
		return &entryCopy, nil
	}

	// Generate unique ID for the entry
	buf.entry.ID = p.generateEntryID(line)
	buf.entry.Raw = line

	// Create a copy of the entry to avoid sharing the pooled object
	entryCopy := *buf.entry
	return &entryCopy, nil
}

// ParseLines parses multiple log lines with parallel processing
func (p *OptimizedParser) ParseLines(lines []string) *ParseResult {
	return p.ParseLinesWithContext(context.Background(), lines)
}

// ParseLinesWithContext parses lines with context support for cancellation
func (p *OptimizedParser) ParseLinesWithContext(ctx context.Context, lines []string) *ParseResult {
	startTime := time.Now()
	result := &ParseResult{
		Entries:   make([]*AccessLogEntry, 0, len(lines)),
		Processed: len(lines),
	}

	if len(lines) == 0 {
		result.Duration = time.Since(startTime)
		return result
	}

	// For small datasets, use single-threaded parsing
	if len(lines) < p.config.BatchSize {
		return p.parseLinesSingleThreaded(ctx, lines, startTime)
	}

	// Use parallel processing for larger datasets
	return p.parseLinesParallel(ctx, lines, startTime)
}

// ParseStream parses log entries from an io.Reader with streaming support
func (p *OptimizedParser) ParseStream(ctx context.Context, reader io.Reader) (*ParseResult, error) {
	startTime := time.Now()
	result := &ParseResult{
		Entries: make([]*AccessLogEntry, 0),
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, p.config.BufferSize), p.config.MaxLineLength)

	batch := make([]string, 0, p.config.BatchSize)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		line := scanner.Text()
		if len(line) == 0 {
			continue
		}

		batch = append(batch, line)
		result.Processed++

		if len(batch) >= p.config.BatchSize {
			batchResult := p.ParseLinesWithContext(ctx, batch)
			result.Entries = append(result.Entries, batchResult.Entries...)
			result.Succeeded += batchResult.Succeeded
			result.Failed += batchResult.Failed
			batch = batch[:0]
		}
	}

	// Process remaining lines in batch
	if len(batch) > 0 {
		batchResult := p.ParseLinesWithContext(ctx, batch)
		result.Entries = append(result.Entries, batchResult.Entries...)
		result.Succeeded += batchResult.Succeeded
		result.Failed += batchResult.Failed
	}

	if err := scanner.Err(); err != nil {
		return result, err
	}

	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	p.updateStats(result)
	return result, nil
}

// parseLinesSingleThreaded handles small datasets with single-threaded parsing
func (p *OptimizedParser) parseLinesSingleThreaded(ctx context.Context, lines []string, startTime time.Time) *ParseResult {
	result := &ParseResult{
		Entries:   make([]*AccessLogEntry, 0, len(lines)),
		Processed: len(lines),
	}

	for _, line := range lines {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result
		default:
		}

		if entry, err := p.ParseLine(line); err == nil {
			result.Entries = append(result.Entries, entry)
			result.Succeeded++
		} else {
			result.Failed++
		}
	}

	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	return result
}

// parseLinesParallel handles large datasets with parallel processing
func (p *OptimizedParser) parseLinesParallel(ctx context.Context, lines []string, startTime time.Time) *ParseResult {
	numWorkers := p.config.WorkerCount
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	if numWorkers > len(lines)/10 {
		numWorkers = len(lines)/10 + 1
	}

	result := &ParseResult{
		Processed: len(lines),
	}

	// Create channels for work distribution
	lineChan := make(chan string, numWorkers*2)
	resultChan := make(chan *AccessLogEntry, len(lines))
	errorChan := make(chan error, len(lines))

	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range lineChan {
				if entry, err := p.ParseLine(line); err == nil {
					resultChan <- entry
				} else {
					errorChan <- err
				}
			}
		}()
	}

	// Send lines to workers
	go func() {
		defer close(lineChan)
		for _, line := range lines {
			select {
			case <-ctx.Done():
				return
			case lineChan <- line:
			}
		}
	}()

	// Wait for workers and close result channels
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// Collect results
	entries := make([]*AccessLogEntry, 0, len(lines))
	var errorCount int

	for {
		select {
		case <-ctx.Done():
			result.Duration = time.Since(startTime)
			return result
		case entry, ok := <-resultChan:
			if !ok {
				resultChan = nil
			} else {
				entries = append(entries, entry)
			}
		case _, ok := <-errorChan:
			if !ok {
				errorChan = nil
			} else {
				errorCount++
			}
		}

		if resultChan == nil && errorChan == nil {
			break
		}
	}

	result.Entries = entries
	result.Succeeded = len(entries)
	result.Failed = errorCount
	result.Duration = time.Since(startTime)

	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	return result
}

// parseLineOptimized performs optimized parsing of a single line
func (p *OptimizedParser) parseLineOptimized(line []byte, buf *parseBuffer) error {
	pos := 0
	length := len(line)

	if length < 20 {
		return errors.New(ErrUnsupportedLogFormat)
	}

	// Parse IP address
	pos = p.parseIP(line, pos, buf.entry)
	if pos >= length {
		return errors.New(ErrUnsupportedLogFormat)
	}

	// Skip remote user fields (- -)
	pos = p.skipSpaces(line, pos)
	pos = p.skipField(line, pos) // remote user
	pos = p.skipSpaces(line, pos)
	pos = p.skipField(line, pos) // remote logname

	// Parse timestamp
	pos = p.skipSpaces(line, pos)
	pos = p.parseTimestamp(line, pos, buf.entry)
	if pos >= length {
		return errors.New(ErrUnsupportedLogFormat)
	}

	// Parse request
	pos = p.skipSpaces(line, pos)
	pos = p.parseRequest(line, pos, buf.entry)
	if pos >= length {
		return errors.New(ErrUnsupportedLogFormat)
	}

	// Parse status code
	pos = p.skipSpaces(line, pos)
	pos = p.parseStatus(line, pos, buf.entry)
	if pos >= length {
		return errors.New(ErrUnsupportedLogFormat)
	}

	// Parse response size
	pos = p.skipSpaces(line, pos)
	pos = p.parseSize(line, pos, buf.entry)

	// Parse optional fields if they exist
	if pos < length {
		pos = p.skipSpaces(line, pos)
		if pos < length && line[pos] == '"' {
			pos = p.parseReferer(line, pos, buf.entry)
		}
	}

	if pos < length {
		pos = p.skipSpaces(line, pos)
		if pos < length && line[pos] == '"' {
			pos = p.parseUserAgent(line, pos, buf.entry)
		}
	}

	if pos < length {
		pos = p.skipSpaces(line, pos)
		if pos < length {
			pos = p.parseRequestTime(line, pos, buf.entry)
		}
	}

	if pos < length {
		pos = p.skipSpaces(line, pos)
		if pos < length {
			_ = p.parseUpstreamTime(line, pos, buf.entry)
		}
	}

	return nil
}

// Fast field parsing methods with zero-copy optimizations
func (p *OptimizedParser) parseIP(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && line[pos] != ' ' {
		pos++
	}
	if pos > start {
		entry.IP = bytesToString(line[start:pos])

		// Populate geographic fields if enabled
		if p.config.EnableGeoIP && p.geoService != nil && entry.IP != "-" {
			if location, err := p.geoService.Search(entry.IP); err == nil && location != nil {
				entry.Province = location.Province
				entry.City = location.City
				// Use the specific RegionCode (e.g., province code 'CA') if available,
				// otherwise, fall back to the CountryCode (e.g., 'US').
				if location.RegionCode != "" {
					entry.RegionCode = location.RegionCode
				} else {
					entry.RegionCode = location.CountryCode
				}
			}
		}
	}
	return pos
}

func (p *OptimizedParser) parseTimestamp(line []byte, pos int, entry *AccessLogEntry) int {
	if pos >= len(line) || line[pos] != '[' {
		return pos
	}
	pos++

	start := pos
	for pos < len(line) && line[pos] != ']' {
		pos++
	}

	if pos > start {
		timeStr := bytesToString(line[start:pos])

		// Debug: log the timestamp string we're trying to parse
		// fmt.Printf("DEBUG: Parsing timestamp string: '%s'\n", timeStr)

		if t, err := time.Parse(p.config.TimeLayout, timeStr); err == nil {
			entry.Timestamp = t.Unix()
		} else {
			// Try alternative common nginx timestamp formats if the default fails
			formats := []string{
				"02/Jan/2006:15:04:05 -0700", // Standard nginx format
				"2006-01-02T15:04:05-07:00",  // ISO 8601 format
				"2006-01-02 15:04:05",        // Simple datetime format
				"02/Jan/2006:15:04:05",       // Without timezone
			}

			parsed := false
			for _, format := range formats {
				if t, err := time.Parse(format, timeStr); err == nil {
					entry.Timestamp = t.Unix()
					parsed = true
					break
				}
			}

			// If all parsing attempts fail, keep timestamp as 0
			if !parsed {
				// Debug: log parsing failure
				// fmt.Printf("DEBUG: Failed to parse timestamp: '%s'\n", timeStr)
			}
		}
	}

	if pos < len(line) && line[pos] == ']' {
		pos++
	}

	return pos
}

func (p *OptimizedParser) parseRequest(line []byte, pos int, entry *AccessLogEntry) int {
	if pos >= len(line) || line[pos] != '"' {
		return pos
	}
	pos++

	start := pos
	for pos < len(line) && line[pos] != '"' {
		pos++
	}

	if pos > start {
		requestLine := line[start:pos]
		parts := bytes.Fields(requestLine)

		if len(parts) >= 1 {
			method := bytesToString(parts[0])
			if ValidHTTPMethods[method] {
				entry.Method = method
			}
		}
		if len(parts) >= 2 {
			entry.Path = bytesToString(parts[1])
		}
		if len(parts) >= 3 {
			entry.Protocol = bytesToString(parts[2])
		}
	}

	if pos < len(line) && line[pos] == '"' {
		pos++
	}

	return pos
}

func (p *OptimizedParser) parseStatus(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && line[pos] >= '0' && line[pos] <= '9' {
		pos++
	}

	if pos > start {
		if status, err := fastParseInt(line[start:pos]); err == nil && status >= 100 && status < 600 {
			entry.Status = status
		}
	}

	return pos
}

func (p *OptimizedParser) parseSize(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && ((line[pos] >= '0' && line[pos] <= '9') || line[pos] == '-') {
		pos++
	}

	if pos > start {
		sizeBytes := line[start:pos]
		if len(sizeBytes) == 1 && sizeBytes[0] == '-' {
			entry.BytesSent = 0
		} else {
			if size, err := fastParseInt(sizeBytes); err == nil && size >= 0 {
				entry.BytesSent = int64(size)
			}
		}
	}

	return pos
}

func (p *OptimizedParser) parseReferer(line []byte, pos int, entry *AccessLogEntry) int {
	if pos >= len(line) || line[pos] != '"' {
		return pos
	}
	pos++

	start := pos
	for pos < len(line) && line[pos] != '"' {
		pos++
	}

	if pos > start {
		entry.Referer = bytesToString(line[start:pos])
	}

	if pos < len(line) && line[pos] == '"' {
		pos++
	}

	return pos
}

func (p *OptimizedParser) parseUserAgent(line []byte, pos int, entry *AccessLogEntry) int {
	if pos >= len(line) || line[pos] != '"' {
		return pos
	}
	pos++

	start := pos
	for pos < len(line) && line[pos] != '"' {
		pos++
	}

	if pos > start {
		userAgent := bytesToString(line[start:pos])
		entry.UserAgent = userAgent

		if p.config.EnableUA && p.uaParser != nil && userAgent != "-" {
			parsed := p.uaParser.Parse(userAgent)
			if parsed.Browser != "Unknown" && parsed.Browser != "" {
				entry.Browser = parsed.Browser
				entry.BrowserVer = parsed.BrowserVer
			}
			if parsed.OS != "Unknown" && parsed.OS != "" {
				entry.OS = parsed.OS
				entry.OSVersion = parsed.OSVersion
			}
			if parsed.DeviceType != "" {
				entry.DeviceType = parsed.DeviceType
			}
		}
	}

	if pos < len(line) && line[pos] == '"' {
		pos++
	}

	return pos
}

func (p *OptimizedParser) parseRequestTime(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && ((line[pos] >= '0' && line[pos] <= '9') || line[pos] == '.' || line[pos] == '-') {
		pos++
	}

	if pos > start {
		timeStr := bytesToString(line[start:pos])
		if timeStr != "-" {
			if val, err := strconv.ParseFloat(timeStr, 64); err == nil && val >= 0 {
				entry.RequestTime = val
			}
		}
	}

	return pos
}

func (p *OptimizedParser) parseUpstreamTime(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && ((line[pos] >= '0' && line[pos] <= '9') || line[pos] == '.' || line[pos] == '-') {
		pos++
	}

	if pos > start {
		timeStr := bytesToString(line[start:pos])
		if timeStr != "-" {
			if val, err := strconv.ParseFloat(timeStr, 64); err == nil && val >= 0 {
				entry.UpstreamTime = &val
			}
		}
	}

	return pos
}

// Utility methods
func (p *OptimizedParser) skipSpaces(line []byte, pos int) int {
	for pos < len(line) && line[pos] == ' ' {
		pos++
	}
	return pos
}

func (p *OptimizedParser) skipField(line []byte, pos int) int {
	for pos < len(line) && line[pos] != ' ' {
		pos++
	}
	return pos
}

func (p *OptimizedParser) generateEntryID(line string) string {
	hash := md5.Sum([]byte(line))
	return fmt.Sprintf("%x", hash)[:16]
}

func (p *OptimizedParser) updateStats(result *ParseResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalLines += int64(result.Processed)
	p.stats.SuccessLines += int64(result.Succeeded)
	p.stats.ErrorLines += int64(result.Failed)
	p.stats.ParseDuration += result.Duration
	p.stats.LastUpdated = time.Now()

	if result.Duration > 0 {
		p.stats.LinesPerSecond = float64(result.Processed) / result.Duration.Seconds()
	}
}

// GetStats returns current parsing statistics
func (p *OptimizedParser) GetStats() *ParseStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	statsCopy := *p.stats
	return &statsCopy
}

// ResetStats resets parsing statistics
func (p *OptimizedParser) ResetStats() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats = &ParseStats{}
}

// Zero-copy string/byte conversion utilities
func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		Cap int
	}{s, len(s)}))
}

func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// Fast integer parsing without allocations
func fastParseInt(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, strconv.ErrSyntax
	}

	neg := false
	if b[0] == '-' {
		neg = true
		b = b[1:]
		if len(b) == 0 {
			return 0, strconv.ErrSyntax
		}
	}

	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			return 0, strconv.ErrSyntax
		}
		n = n*10 + int(c-'0')
	}

	if neg {
		n = -n
	}

	return n, nil
}
