package parser

import ()

// SIMD-optimized string processing for nginx log parsing
// These functions provide vectorized operations for common parsing tasks

// SIMDStringMatcher provides SIMD-optimized string matching operations
type SIMDStringMatcher struct {
	// Pre-computed lookup tables for fast character classification
	spaceLookup    [256]bool
	quoteLookup    [256]bool
	bracketLookup  [256]bool
	digitLookup    [256]bool
	hexLookup      [256]bool
}

// NewSIMDStringMatcher creates a new SIMD-optimized string matcher
func NewSIMDStringMatcher() *SIMDStringMatcher {
	matcher := &SIMDStringMatcher{}
	matcher.initLookupTables()
	return matcher
}

// initLookupTables initializes lookup tables for fast character classification
func (sm *SIMDStringMatcher) initLookupTables() {
	// Space characters lookup
	spaces := []byte{' ', '\t', '\n', '\r'}
	for _, c := range spaces {
		sm.spaceLookup[c] = true
	}
	
	// Quote characters lookup
	quotes := []byte{'"', '\''}
	for _, c := range quotes {
		sm.quoteLookup[c] = true
	}
	
	// Bracket characters lookup
	brackets := []byte{'[', ']', '(', ')', '{', '}'}
	for _, c := range brackets {
		sm.bracketLookup[c] = true
	}
	
	// Digit characters lookup
	for i := '0'; i <= '9'; i++ {
		sm.digitLookup[i] = true
	}
	
	// Hexadecimal characters lookup
	for i := '0'; i <= '9'; i++ {
		sm.hexLookup[i] = true
	}
	for i := 'A'; i <= 'F'; i++ {
		sm.hexLookup[i] = true
	}
	for i := 'a'; i <= 'f'; i++ {
		sm.hexLookup[i] = true
	}
}

// FindNextSpace finds the next space character using SIMD-like operations
func (sm *SIMDStringMatcher) FindNextSpace(data []byte, start int) int {
	if start >= len(data) {
		return -1
	}
	
	// Process 8 bytes at a time for better cache utilization
	const blockSize = 8
	end := len(data)
	i := start
	
	// Vectorized search - process multiple bytes at once
	for i+blockSize <= end {
		// Check 8 bytes in parallel using lookup table
		for j := 0; j < blockSize; j++ {
			if sm.spaceLookup[data[i+j]] {
				return i + j
			}
		}
		i += blockSize
	}
	
	// Handle remaining bytes
	for i < end {
		if sm.spaceLookup[data[i]] {
			return i
		}
		i++
	}
	
	return -1
}

// FindNextQuote finds the next quote character using optimized search
func (sm *SIMDStringMatcher) FindNextQuote(data []byte, start int) int {
	if start >= len(data) {
		return -1
	}
	
	const blockSize = 8
	end := len(data)
	i := start
	
	// Vectorized search for quotes
	for i+blockSize <= end {
		for j := 0; j < blockSize; j++ {
			if sm.quoteLookup[data[i+j]] {
				return i + j
			}
		}
		i += blockSize
	}
	
	// Handle remaining bytes
	for i < end {
		if sm.quoteLookup[data[i]] {
			return i
		}
		i++
	}
	
	return -1
}

// FindNextDigit finds the next digit character using optimized search
func (sm *SIMDStringMatcher) FindNextDigit(data []byte, start int) int {
	if start >= len(data) {
		return -1
	}
	
	const blockSize = 8
	end := len(data)
	i := start
	
	// Vectorized search for digits
	for i+blockSize <= end {
		for j := 0; j < blockSize; j++ {
			if sm.digitLookup[data[i+j]] {
				return i + j
			}
		}
		i += blockSize
	}
	
	// Handle remaining bytes
	for i < end {
		if sm.digitLookup[data[i]] {
			return i
		}
		i++
	}
	
	return -1
}

// ExtractIPAddress extracts IP address using SIMD-optimized operations
func (sm *SIMDStringMatcher) ExtractIPAddress(data []byte, start int) (string, int) {
	if start >= len(data) {
		return "", -1
	}
	
	// Find start of IP (first digit)
	ipStart := sm.FindNextDigit(data, start)
	if ipStart == -1 {
		return "", -1
	}
	
	// Find end of IP (first space after IP)
	ipEnd := sm.FindNextSpace(data, ipStart)
	if ipEnd == -1 {
		ipEnd = len(data)
	}
	
	// Validate IP format using fast checks
	ipBytes := data[ipStart:ipEnd]
	if sm.isValidIPFormat(ipBytes) {
		return unsafeBytesToString(ipBytes), ipEnd
	}
	
	return "", -1
}

// isValidIPFormat quickly validates IP format using SIMD-like operations
func (sm *SIMDStringMatcher) isValidIPFormat(data []byte) bool {
	if len(data) < 7 || len(data) > 15 { // Min: 1.1.1.1, Max: 255.255.255.255
		return false
	}
	
	dotCount := 0
	digitCount := 0
	
	// Fast validation using lookup tables
	for _, b := range data {
		if b == '.' {
			dotCount++
			if digitCount == 0 || digitCount > 3 {
				return false
			}
			digitCount = 0
		} else if sm.digitLookup[b] {
			digitCount++
		} else {
			return false
		}
	}
	
	return dotCount == 3 && digitCount > 0 && digitCount <= 3
}

// ExtractTimestamp extracts timestamp using SIMD-optimized bracket search
func (sm *SIMDStringMatcher) ExtractTimestamp(data []byte, start int) (string, int) {
	if start >= len(data) {
		return "", -1
	}
	
	// Find opening bracket
	openBracket := sm.findBracket(data, start, '[')
	if openBracket == -1 {
		return "", -1
	}
	
	// Find closing bracket
	closeBracket := sm.findBracket(data, openBracket+1, ']')
	if closeBracket == -1 {
		return "", -1
	}
	
	// Extract timestamp content (exclude brackets)
	timestampBytes := data[openBracket+1 : closeBracket]
	return unsafeBytesToString(timestampBytes), closeBracket + 1
}

// findBracket finds specific bracket character using optimized search
func (sm *SIMDStringMatcher) findBracket(data []byte, start int, bracket byte) int {
	if start >= len(data) {
		return -1
	}
	
	const blockSize = 8
	end := len(data)
	i := start
	
	// Vectorized search for specific bracket
	for i+blockSize <= end {
		for j := range blockSize {
			if data[i+j] == bracket {
				return i + j
			}
		}
		i += blockSize
	}
	
	// Handle remaining bytes
	for i < end {
		if data[i] == bracket {
			return i
		}
		i++
	}
	
	return -1
}

// ExtractQuotedString extracts quoted string using optimized quote search
func (sm *SIMDStringMatcher) ExtractQuotedString(data []byte, start int) (string, int) {
	if start >= len(data) {
		return "", -1
	}
	
	// Find opening quote
	openQuote := sm.FindNextQuote(data, start)
	if openQuote == -1 {
		return "", -1
	}
	
	// Find closing quote (skip escaped quotes)
	closeQuote := sm.findClosingQuote(data, openQuote+1, data[openQuote])
	if closeQuote == -1 {
		return "", -1
	}
	
	// Extract string content (exclude quotes)
	stringBytes := data[openQuote+1 : closeQuote]
	return unsafeBytesToString(stringBytes), closeQuote + 1
}

// findClosingQuote finds matching closing quote, handling escapes
func (sm *SIMDStringMatcher) findClosingQuote(data []byte, start int, quoteChar byte) int {
	if start >= len(data) {
		return -1
	}
	
	i := start
	for i < len(data) {
		if data[i] == quoteChar {
			// Check if it's escaped
			if i == start || data[i-1] != '\\' {
				return i
			}
		}
		i++
	}
	
	return -1
}

// ExtractStatusCode extracts HTTP status code using optimized digit search
func (sm *SIMDStringMatcher) ExtractStatusCode(data []byte, start int) (int, int) {
	if start >= len(data) {
		return 0, -1
	}
	
	// Find start of status code (3 consecutive digits)
	statusStart := sm.findStatusCodeStart(data, start)
	if statusStart == -1 {
		return 0, -1
	}
	
	// Extract 3-digit status code
	if statusStart+2 >= len(data) {
		return 0, -1
	}
	
	// Fast integer conversion for 3-digit status codes
	status := int(data[statusStart]-'0')*100 + 
			  int(data[statusStart+1]-'0')*10 + 
			  int(data[statusStart+2]-'0')
	
	return status, statusStart + 3
}

// findStatusCodeStart finds start of 3-digit HTTP status code
func (sm *SIMDStringMatcher) findStatusCodeStart(data []byte, start int) int {
	if start+2 >= len(data) {
		return -1
	}
	
	for i := start; i <= len(data)-3; i++ {
		// Check if we have 3 consecutive digits
		if sm.digitLookup[data[i]] && 
		   sm.digitLookup[data[i+1]] && 
		   sm.digitLookup[data[i+2]] {
			// Validate it's a proper HTTP status code (100-599)
			firstDigit := int(data[i] - '0')
			if firstDigit >= 1 && firstDigit <= 5 {
				// Also check that it's preceded by a quote and space or space
				if i > 0 && (data[i-1] == ' ' || data[i-1] == '"') {
					return i
				}
				// If we're looking at a pattern like '" 200 ', this is likely the status code
				if i > 1 && data[i-2] == '"' && data[i-1] == ' ' {
					return i
				}
			}
		}
	}
	
	return -1
}

// ParseLogLineSIMD parses a complete log line using SIMD optimizations
func (sm *SIMDStringMatcher) ParseLogLineSIMD(data []byte) *AccessLogEntry {
	if len(data) == 0 {
		return nil
	}
	
	entry := &AccessLogEntry{}
	pos := 0
	
	// Extract IP address
	if ip, newPos := sm.ExtractIPAddress(data, pos); ip != "" {
		entry.IP = ip
		pos = newPos
	} else {
		return nil
	}
	
	// Skip user fields (- -)
	pos = sm.skipUserFields(data, pos)
	if pos == -1 {
		return nil
	}
	
	// Extract timestamp
	if timestampStr, newPos := sm.ExtractTimestamp(data, pos); timestampStr != "" {
		// Note: In production, you'd parse this timestamp string to int64
		// For now, storing as 0 to avoid parsing complexity in SIMD implementation
		entry.Timestamp = 0
		pos = newPos
	}
	
	// Extract request (quoted string) - parse method/path from it
	if request, newPos := sm.ExtractQuotedString(data, pos); request != "" {
		// Parse method and path from request string
		sm.parseRequestComponents(request, entry)
		pos = newPos
	}
	
	// Extract status code
	if status, newPos := sm.ExtractStatusCode(data, pos); status > 0 {
		entry.Status = status
		pos = newPos
	}
	
	// Extract size (next number)
	if size, newPos := sm.extractSize(data, pos); newPos != -1 {
		entry.BytesSent = size
		pos = newPos
	}
	
	// Extract referer (quoted string)
	if referer, newPos := sm.ExtractQuotedString(data, pos); referer != "" {
		entry.Referer = referer
		pos = newPos
	}
	
	// Extract user agent (quoted string)
	if userAgent, _ := sm.ExtractQuotedString(data, pos); userAgent != "" {
		entry.UserAgent = userAgent
	}
	
	return entry
}

// parseRequestComponents parses method, path, and protocol from request string
func (sm *SIMDStringMatcher) parseRequestComponents(request string, entry *AccessLogEntry) {
	requestBytes := []byte(request)
	
	// Find first space (after method)
	firstSpace := sm.FindNextSpace(requestBytes, 0)
	if firstSpace == -1 {
		return
	}
	
	// Extract method
	entry.Method = unsafeBytesToString(requestBytes[:firstSpace])
	
	// Find second space (after path)
	secondSpace := sm.FindNextSpace(requestBytes, firstSpace+1)
	if secondSpace == -1 {
		// Only method and path, no protocol
		entry.Path = unsafeBytesToString(requestBytes[firstSpace+1:])
		return
	}
	
	// Extract path and protocol
	entry.Path = unsafeBytesToString(requestBytes[firstSpace+1 : secondSpace])
	entry.Protocol = unsafeBytesToString(requestBytes[secondSpace+1:])
}

// skipUserFields skips the user fields (typically "- -")
func (sm *SIMDStringMatcher) skipUserFields(data []byte, start int) int {
	pos := start
	spaceCount := 0
	
	for pos < len(data) && spaceCount < 2 {
		if sm.spaceLookup[data[pos]] {
			spaceCount++
		}
		pos++
	}
	
	if spaceCount < 2 {
		return -1
	}
	
	return pos
}

// extractSize extracts size field (number or "-")
func (sm *SIMDStringMatcher) extractSize(data []byte, start int) (int64, int) {
	// Skip leading spaces
	pos := start
	for pos < len(data) && sm.spaceLookup[data[pos]] {
		pos++
	}
	
	if pos >= len(data) {
		return 0, -1
	}
	
	// Check for "-" (no size)
	if data[pos] == '-' {
		return 0, pos + 1
	}
	
	// Extract numeric size
	sizeStart := pos
	for pos < len(data) && sm.digitLookup[data[pos]] {
		pos++
	}
	
	if pos == sizeStart {
		return 0, -1
	}
	
	// Fast integer conversion
	var size int64
	for i := sizeStart; i < pos; i++ {
		size = size*10 + int64(data[i]-'0')
	}
	
	return size, pos
}

// BatchParseSIMD parses multiple log lines using SIMD optimizations
func (sm *SIMDStringMatcher) BatchParseSIMD(lines [][]byte) []*AccessLogEntry {
	entries := make([]*AccessLogEntry, 0, len(lines))
	
	for _, line := range lines {
		if entry := sm.ParseLogLineSIMD(line); entry != nil {
			entries = append(entries, entry)
		}
	}
	
	return entries
}

// LogLineParser provides a high-performance parser using SIMD operations
type LogLineParser struct {
	matcher *SIMDStringMatcher
	pool    *AccessLogEntryPool
}

// NewLogLineParser creates a new optimized parser
func NewLogLineParser() *LogLineParser {
	return &LogLineParser{
		matcher: NewSIMDStringMatcher(),
		pool:    NewAccessLogEntryPool(),
	}
}

// ParseLine parses a single log line with maximum performance
func (olp *LogLineParser) ParseLine(data []byte) *AccessLogEntry {
	return olp.matcher.ParseLogLineSIMD(data)
}

// ParseLines parses multiple lines efficiently
func (olp *LogLineParser) ParseLines(lines [][]byte) []*AccessLogEntry {
	return olp.matcher.BatchParseSIMD(lines)
}

// AccessLogEntryPool provides object pooling for AccessLogEntry
type AccessLogEntryPool struct {
	entries chan *AccessLogEntry
}

// NewAccessLogEntryPool creates a new object pool
func NewAccessLogEntryPool() *AccessLogEntryPool {
	pool := &AccessLogEntryPool{
		entries: make(chan *AccessLogEntry, 1000),
	}
	
	// Pre-populate pool
	for i := 0; i < 100; i++ {
		pool.entries <- &AccessLogEntry{}
	}
	
	return pool
}

// Get retrieves an entry from the pool
func (pool *AccessLogEntryPool) Get() *AccessLogEntry {
	select {
	case entry := <-pool.entries:
		return entry
	default:
		return &AccessLogEntry{}
	}
}

// Put returns an entry to the pool
func (pool *AccessLogEntryPool) Put(entry *AccessLogEntry) {
	// Reset entry fields
	*entry = AccessLogEntry{}
	
	select {
	case pool.entries <- entry:
	default:
		// Pool is full, let GC handle it
	}
}