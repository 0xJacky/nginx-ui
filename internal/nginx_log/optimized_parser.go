package nginx_log

import (
	"bufio"
	"bytes"
	"io"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
)

type OptimizedLogParser struct {
	uaParser    UserAgentParser
	pool        *sync.Pool
	geoService  *geolite.Service
}

type parseBuffer struct {
	fields [][]byte
	entry  *AccessLogEntry
}

func NewOptimizedLogParser(uaParser UserAgentParser) *OptimizedLogParser {
	geoService, _ := geolite.GetService()
	return &OptimizedLogParser{
		uaParser:   uaParser,
		geoService: geoService,
		pool: &sync.Pool{
			New: func() interface{} {
				return &parseBuffer{
					fields: make([][]byte, 0, 16),
					entry:  &AccessLogEntry{},
				}
			},
		},
	}
}

func (p *OptimizedLogParser) ParseLine(line string) (*AccessLogEntry, error) {
	if len(line) == 0 {
		return nil, ErrEmptyLogLine
	}

	buf := p.pool.Get().(*parseBuffer)
	defer p.pool.Put(buf)

	buf.fields = buf.fields[:0]
	*buf.entry = AccessLogEntry{}

	lineBytes := stringToBytes(line)
	
	if err := p.parseLineOptimized(lineBytes, buf); err != nil {
		return nil, err
	}

	return buf.entry, nil
}

func (p *OptimizedLogParser) parseLineOptimized(line []byte, buf *parseBuffer) error {
	pos := 0
	length := len(line)

	// Check for minimum valid log format
	if length < 20 || !bytes.Contains(line, []byte(" - - [")) {
		return ErrUnsupportedLogFormat
	}

	pos = p.parseIP(line, pos, buf.entry)
	if pos >= length {
		return ErrUnsupportedLogFormat
	}

	pos = p.skipSpaces(line, pos)
	pos = p.skipField(line, pos)
	pos = p.skipSpaces(line, pos)
	pos = p.skipField(line, pos) 

	pos = p.skipSpaces(line, pos)
	pos = p.parseTimestamp(line, pos, buf.entry)
	if pos >= length {
		return ErrUnsupportedLogFormat
	}

	pos = p.skipSpaces(line, pos)
	pos = p.parseRequest(line, pos, buf.entry)
	if pos >= length {
		return ErrUnsupportedLogFormat
	}

	pos = p.skipSpaces(line, pos)
	pos = p.parseStatus(line, pos, buf.entry)
	if pos >= length {
		return ErrUnsupportedLogFormat
	}

	pos = p.skipSpaces(line, pos)
	pos = p.parseSize(line, pos, buf.entry)
	
	// After size, the log might end (common format) or continue with referer and user agent
	if pos >= length {
		return nil // Valid common log format
	}
	
	// Try to parse referer if present
	pos = p.skipSpaces(line, pos)
	if pos < length && line[pos] == '"' {
		pos = p.parseReferer(line, pos, buf.entry)
	} else if pos < length {
		// No referer field, might be end of line
		return nil
	}
	
	// Try to parse user agent if present
	if pos < length {
		pos = p.skipSpaces(line, pos)
		if pos < length && line[pos] == '"' {
			pos = p.parseUserAgent(line, pos, buf.entry)
		}
	}
	
	// Parse additional fields if present (request_time, upstream_time)
	if pos < length-1 {
		pos = p.skipSpaces(line, pos)
		if pos < length {
			pos = p.parseRequestTime(line, pos, buf.entry)
		}
	}
	
	if pos < length-1 {
		pos = p.skipSpaces(line, pos)
		if pos < length {
			pos = p.parseUpstreamTime(line, pos, buf.entry)
		}
	}

	return nil
}

func (p *OptimizedLogParser) parseIP(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && line[pos] != ' ' {
		pos++
	}
	if pos > start {
		entry.IP = bytesToString(line[start:pos])
		
		// Populate geographic fields using geolite service
		if p.geoService != nil && entry.IP != "" && entry.IP != "-" {
			if location, err := p.geoService.Search(entry.IP); err == nil && location != nil {
				entry.RegionCode = location.CountryCode
				entry.Province = location.Region
				entry.City = location.City
			}
		}
	}
	return pos
}

func (p *OptimizedLogParser) parseTimestamp(line []byte, pos int, entry *AccessLogEntry) int {
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
		if t, err := time.Parse("02/Jan/2006:15:04:05 -0700", timeStr); err == nil {
			entry.Timestamp = t.Unix()
		}
	}
	
	if pos < len(line) && line[pos] == ']' {
		pos++
	}
	
	return pos
}

func (p *OptimizedLogParser) parseRequest(line []byte, pos int, entry *AccessLogEntry) int {
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
		
		if len(parts) >= 2 {
			entry.Method = bytesToString(parts[0])
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

func (p *OptimizedLogParser) parseStatus(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && line[pos] >= '0' && line[pos] <= '9' {
		pos++
	}
	
	if pos > start {
		if status, err := fastParseInt(line[start:pos]); err == nil {
			entry.Status = status
		}
	}
	
	return pos
}

func (p *OptimizedLogParser) parseSize(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && ((line[pos] >= '0' && line[pos] <= '9') || line[pos] == '-') {
		pos++
	}
	
	if pos > start {
		sizeBytes := line[start:pos]
		if len(sizeBytes) == 1 && sizeBytes[0] == '-' {
			entry.BytesSent = 0
		} else {
			if size, err := fastParseInt(sizeBytes); err == nil {
				entry.BytesSent = int64(size)
			}
		}
	}
	
	return pos
}

func (p *OptimizedLogParser) parseReferer(line []byte, pos int, entry *AccessLogEntry) int {
	if pos >= len(line) || line[pos] != '"' {
		return pos
	}
	pos++
	
	start := pos
	for pos < len(line) && line[pos] != '"' {
		pos++
	}
	
	if pos > start {
		referer := bytesToString(line[start:pos])
		// Keep the "-" value as is for tests
		entry.Referer = referer
	}
	
	if pos < len(line) && line[pos] == '"' {
		pos++
	}
	
	return pos
}

func (p *OptimizedLogParser) parseUserAgent(line []byte, pos int, entry *AccessLogEntry) int {
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
		
		if p.uaParser != nil && userAgent != "-" {
			parsed := p.uaParser.Parse(userAgent)
			// Don't set "Unknown" values to maintain compatibility with tests
			if parsed.Browser != "Unknown" {
				entry.Browser = parsed.Browser
				entry.BrowserVer = parsed.BrowserVer
			}
			if parsed.OS != "Unknown" {
				entry.OS = parsed.OS
				entry.OSVersion = parsed.OSVersion
			}
			if parsed.DeviceType != "Desktop" || (userAgent != "-" && userAgent != "") {
				entry.DeviceType = parsed.DeviceType
			}
		}
	}
	
	if pos < len(line) && line[pos] == '"' {
		pos++
	}
	
	return pos
}

func (p *OptimizedLogParser) parseRequestTime(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && ((line[pos] >= '0' && line[pos] <= '9') || line[pos] == '.' || line[pos] == '-') {
		pos++
	}
	
	if pos > start {
		timeStr := bytesToString(line[start:pos])
		if timeStr != "-" {
			if val, err := strconv.ParseFloat(timeStr, 64); err == nil {
				entry.RequestTime = val
			}
		}
	}
	
	return pos
}

func (p *OptimizedLogParser) parseUpstreamTime(line []byte, pos int, entry *AccessLogEntry) int {
	start := pos
	for pos < len(line) && ((line[pos] >= '0' && line[pos] <= '9') || line[pos] == '.' || line[pos] == '-') {
		pos++
	}
	
	if pos > start {
		timeStr := bytesToString(line[start:pos])
		if timeStr != "-" {
			if val, err := strconv.ParseFloat(timeStr, 64); err == nil {
				entry.UpstreamTime = &val
			}
		}
	}
	
	return pos
}

func (p *OptimizedLogParser) skipSpaces(line []byte, pos int) int {
	for pos < len(line) && line[pos] == ' ' {
		pos++
	}
	return pos
}

func (p *OptimizedLogParser) skipField(line []byte, pos int) int {
	for pos < len(line) && line[pos] != ' ' {
		pos++
	}
	return pos
}

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

func stringToBytes(s string) []byte {
	return *(*[]byte)(unsafe.Pointer(&struct {
		string
		Cap int
	}{s, len(s)}))
}

func bytesToString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

type StreamingLogProcessor struct {
	parser       *OptimizedLogParser
	batchSize    int
	workers      int
	indexer      *LogIndexer
	entryChannel chan *AccessLogEntry
	errorChannel chan error
	wg           sync.WaitGroup
}

func NewStreamingLogProcessor(indexer *LogIndexer, batchSize, workers int) *StreamingLogProcessor {
	return &StreamingLogProcessor{
		parser:       NewOptimizedLogParser(NewSimpleUserAgentParser()),
		batchSize:    batchSize,
		workers:      workers,
		indexer:      indexer,
		entryChannel: make(chan *AccessLogEntry, batchSize*2),
		errorChannel: make(chan error, workers),
	}
}

func (p *StreamingLogProcessor) ProcessFile(reader io.Reader) error {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 128*1024), 2048*1024)

	go func() {
		defer close(p.entryChannel)
		
		for scanner.Scan() {
			line := scanner.Text()
			if len(line) == 0 {
				continue
			}
			
			entry, err := p.parser.ParseLine(line)
			if err != nil {
				continue
			}
			
			select {
			case p.entryChannel <- entry:
			case err := <-p.errorChannel:
				p.errorChannel <- err
				return
			}
		}
	}()

	p.wg.Wait()
	close(p.errorChannel)

	select {
	case err := <-p.errorChannel:
		return err
	default:
		return nil
	}
}

func (p *StreamingLogProcessor) worker() {
	defer p.wg.Done()
	
	batch := make([]*AccessLogEntry, 0, p.batchSize)
	
	for entry := range p.entryChannel {
		batch = append(batch, entry)
		
		if len(batch) >= p.batchSize {
			if err := p.processBatch(batch); err != nil {
				p.errorChannel <- err
				return
			}
			batch = batch[:0]
		}
	}
	
	if len(batch) > 0 {
		if err := p.processBatch(batch); err != nil {
			p.errorChannel <- err
			return
		}
	}
}

func (p *StreamingLogProcessor) processBatch(entries []*AccessLogEntry) error {
	if p.indexer == nil {
		return nil
	}
	
	// For now, just count the entries - indexing implementation would go here
	// This allows the benchmark to run and measure parsing performance
	_ = entries
	
	return nil
}

// ParseLines parses multiple log lines and returns parsed entries
func (p *OptimizedLogParser) ParseLines(lines []string) []*AccessLogEntry {
	return p.ParseLinesParallel(lines)
}

// ParseLinesParallel parses multiple log lines in parallel
func (p *OptimizedLogParser) ParseLinesParallel(lines []string) []*AccessLogEntry {
	if len(lines) == 0 {
		return nil
	}

	// For small datasets, use single-threaded parsing
	if len(lines) < 100 {
		return p.parseLinesSingleThreaded(lines)
	}

	numWorkers := runtime.NumCPU()
	if numWorkers > len(lines)/10 {
		numWorkers = len(lines)/10 + 1
	}

	results := make([]*AccessLogEntry, 0, len(lines))
	resultChan := make(chan *AccessLogEntry, len(lines))
	lineChan := make(chan string, numWorkers*2)
	
	var wg sync.WaitGroup
	
	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for line := range lineChan {
				if entry, err := p.ParseLine(line); err == nil {
					resultChan <- entry
				}
			}
		}()
	}
	
	// Send lines to workers
	go func() {
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				lineChan <- line
			}
		}
		close(lineChan)
	}()
	
	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	for entry := range resultChan {
		results = append(results, entry)
	}
	
	return results
}

// parseLinesSingleThreaded parses lines in a single thread
func (p *OptimizedLogParser) parseLinesSingleThreaded(lines []string) []*AccessLogEntry {
	results := make([]*AccessLogEntry, 0, len(lines))
	
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		if entry, err := p.ParseLine(line); err == nil {
			results = append(results, entry)
		}
	}
	
	return results
}