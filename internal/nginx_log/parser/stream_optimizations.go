package parser

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"time"
	"unsafe"
)

// OptimizedParseStream provides a high-performance streaming parser with zero-allocation optimizations
func (p *OptimizedParser) OptimizedParseStream(ctx context.Context, reader io.Reader) (*ParseResult, error) {
	startTime := time.Now()
	
	// Pre-allocate result with estimated capacity to reduce reallocations
	result := &ParseResult{
		Entries: make([]*AccessLogEntry, 0, 10000), // Pre-allocate for better performance
	}

	// Use a larger buffer for better I/O performance
	const bufferSize = 64 * 1024 // 64KB buffer
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, bufferSize), p.config.MaxLineLength)

	// Pre-allocate batch slice with capacity
	batch := make([]string, 0, p.config.BatchSize)
	contextCheckCounter := 0
	const contextCheckFreq = 100 // Check context every 100 lines instead of every line

	// Stream processing with optimized batching
	for scanner.Scan() {
		// Reduce context checking frequency for better performance
		contextCheckCounter++
		if contextCheckCounter >= contextCheckFreq {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}
			contextCheckCounter = 0
		}

		// Get line bytes from scanner
		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		// Convert bytes to string with proper copying to avoid corruption
		line := string(lineBytes)
		batch = append(batch, line)
		result.Processed++

		// Process full batches
		if len(batch) >= p.config.BatchSize {
			if err := p.processBatchOptimized(ctx, batch, result); err != nil {
				return result, err
			}
			// Reset batch slice but keep capacity
			batch = batch[:0]
		}
	}

	// Process remaining lines
	if len(batch) > 0 {
		if err := p.processBatchOptimized(ctx, batch, result); err != nil {
			return result, err
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return result, err
	}

	// Finalize result
	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	return result, nil
}

// processBatchOptimized processes a batch of lines with memory-efficient operations
func (p *OptimizedParser) processBatchOptimized(ctx context.Context, batch []string, result *ParseResult) error {
	batchResult := p.ParseLinesWithContext(ctx, batch)
	
	// Pre-grow the result.Entries slice to avoid multiple reallocations
	currentLen := len(result.Entries)
	newLen := currentLen + len(batchResult.Entries)
	
	// Grow the slice efficiently
	if cap(result.Entries) < newLen {
		newEntries := make([]*AccessLogEntry, newLen, newLen*2) // Double capacity for future growth
		copy(newEntries, result.Entries)
		result.Entries = newEntries
	} else {
		result.Entries = result.Entries[:newLen]
	}
	
	// Copy batch results efficiently
	copy(result.Entries[currentLen:], batchResult.Entries)
	
	result.Succeeded += batchResult.Succeeded
	result.Failed += batchResult.Failed
	
	return nil
}

// StreamBuffer provides a reusable buffer for streaming operations
type StreamBuffer struct {
	data   []byte
	offset int
}

// NewStreamBuffer creates a new stream buffer with the specified capacity
func NewStreamBuffer(capacity int) *StreamBuffer {
	return &StreamBuffer{
		data: make([]byte, 0, capacity),
	}
}

// ReadLine reads a single line from the buffer, reusing memory where possible
func (sb *StreamBuffer) ReadLine(reader io.Reader) ([]byte, error) {
	// Reset buffer for reuse
	sb.data = sb.data[:0]
	sb.offset = 0
	
	buf := make([]byte, 1)
	for {
		n, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF && len(sb.data) > 0 {
				return sb.data, nil
			}
			return nil, err
		}
		if n == 0 {
			continue
		}
		
		if buf[0] == '\n' {
			return sb.data, nil
		}
		
		sb.data = append(sb.data, buf[0])
	}
}

// ChunkedParseStream processes the stream in chunks for better memory usage
func (p *OptimizedParser) ChunkedParseStream(ctx context.Context, reader io.Reader, chunkSize int) (*ParseResult, error) {
	startTime := time.Now()
	result := &ParseResult{
		Entries: make([]*AccessLogEntry, 0, chunkSize),
	}

	buffer := make([]byte, chunkSize)
	remainder := make([]byte, 0, 1024)
	
	for {
		// Check context periodically
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		n, err := reader.Read(buffer)
		if n == 0 && err == io.EOF {
			break
		}
		if err != nil && err != io.EOF {
			return result, err
		}

		// Combine remainder with new data
		data := append(remainder, buffer[:n]...)
		lines := bytes.Split(data, []byte("\n"))
		
		// Keep the last incomplete line as remainder
		if err != io.EOF {
			remainder = append(remainder[:0], lines[len(lines)-1]...)
			lines = lines[:len(lines)-1]
		} else {
			remainder = remainder[:0]
		}

		// Process lines in batches
		batch := make([]string, 0, p.config.BatchSize)
		for _, lineBytes := range lines {
			if len(lineBytes) == 0 {
				continue
			}
			
			line := string(lineBytes)
			batch = append(batch, line)
			result.Processed++
			
			if len(batch) >= p.config.BatchSize {
				if err := p.processBatchOptimized(ctx, batch, result); err != nil {
					return result, err
				}
				batch = batch[:0]
			}
		}
		
		// Process remaining batch
		if len(batch) > 0 {
			if err := p.processBatchOptimized(ctx, batch, result); err != nil {
				return result, err
			}
		}
		
		if err == io.EOF {
			break
		}
	}
	
	// Process any remaining data
	if len(remainder) > 0 {
		line := string(remainder)
		batch := []string{line}
		result.Processed++
		if err := p.processBatchOptimized(ctx, batch, result); err != nil {
			return result, err
		}
	}

	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	return result, nil
}

// unsafeBytesToString converts bytes to string without memory allocation
func unsafeBytesToString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

// LineBuffer provides a reusable line buffer for parsing operations
type LineBuffer struct {
	data []byte
	cap  int
}

// NewLineBuffer creates a new line buffer with the specified capacity
func NewLineBuffer(capacity int) *LineBuffer {
	return &LineBuffer{
		data: make([]byte, 0, capacity),
		cap:  capacity,
	}
}

// Reset resets the buffer for reuse
func (lb *LineBuffer) Reset() {
	lb.data = lb.data[:0]
}

// Grow grows the buffer to accommodate more data
func (lb *LineBuffer) Grow(n int) {
	if cap(lb.data) < len(lb.data)+n {
		newData := make([]byte, len(lb.data), (len(lb.data)+n)*2)
		copy(newData, lb.data)
		lb.data = newData
	}
}

// Append appends data to the buffer
func (lb *LineBuffer) Append(data []byte) {
	lb.Grow(len(data))
	lb.data = append(lb.data, data...)
}

// String returns the buffer content as a string (with copying)
func (lb *LineBuffer) String() string {
	return string(lb.data)
}

// UnsafeString returns the buffer content as a string without copying
func (lb *LineBuffer) UnsafeString() string {
	return unsafeBytesToString(lb.data)
}

// Bytes returns the buffer content as bytes
func (lb *LineBuffer) Bytes() []byte {
	return lb.data
}

// MemoryEfficientParseStream uses minimal memory allocations for streaming
func (p *OptimizedParser) MemoryEfficientParseStream(ctx context.Context, reader io.Reader) (*ParseResult, error) {
	startTime := time.Now()
	result := &ParseResult{
		Entries: make([]*AccessLogEntry, 0, 1000),
	}

	// Use pooled buffers for memory efficiency
	lineBuffer := NewLineBuffer(2048)
	defer lineBuffer.Reset()

	// Use a smaller, more efficient scanner
	scanner := bufio.NewScanner(reader)
	batch := make([]string, 0, p.config.BatchSize)
	lineCount := 0

	for scanner.Scan() {
		// Reduce context check frequency
		lineCount++
		if lineCount%50 == 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			default:
			}
		}

		lineBuffer.Reset()
		lineBuffer.Append(scanner.Bytes())
		
		if lineBuffer.Bytes() == nil || len(lineBuffer.Bytes()) == 0 {
			continue
		}

		// Use safe conversion to avoid corruption
		line := lineBuffer.String()
		batch = append(batch, line)
		result.Processed++

		if len(batch) >= p.config.BatchSize {
			if err := p.processBatchOptimized(ctx, batch, result); err != nil {
				return result, err
			}
			batch = batch[:0]
		}
	}

	// Process remaining lines
	if len(batch) > 0 {
		if err := p.processBatchOptimized(ctx, batch, result); err != nil {
			return result, err
		}
	}

	if err := scanner.Err(); err != nil {
		return result, err
	}

	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	return result, nil
}