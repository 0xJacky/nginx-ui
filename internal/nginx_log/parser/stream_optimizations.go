package parser

import (
	"bufio"
	"context"
	"io"
	"time"
)

// StreamParseBatches reads the stream line by line and parses it in batches
// of p.config.BatchSize, invoking fn with each parsed batch as soon as it is
// ready. Only one batch is held in memory at a time, so peak memory stays
// bounded regardless of input size. The entries passed to fn are owned by the
// callback; they are not reused by the parser.
//
// The returned ParseResult carries the counters (Processed/Succeeded/Failed)
// but a nil Entries slice.
func (p *Parser) StreamParseBatches(ctx context.Context, reader io.Reader, fn func(entries []*AccessLogEntry) error) (*ParseResult, error) {
	startTime := time.Now()
	result := &ParseResult{}

	// Use a larger buffer for better I/O performance
	const bufferSize = 64 * 1024 // 64KB buffer
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, bufferSize), p.config.MaxLineLength)

	batch := make([]string, 0, p.config.BatchSize)
	contextCheckCounter := 0
	const contextCheckFreq = 100 // Check context every 100 lines instead of every line

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		batchResult := p.ParseLinesWithContext(ctx, batch)
		batch = batch[:0]

		result.Succeeded += batchResult.Succeeded
		result.Failed += batchResult.Failed

		if fn != nil && len(batchResult.Entries) > 0 {
			return fn(batchResult.Entries)
		}
		return nil
	}

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

		lineBytes := scanner.Bytes()
		if len(lineBytes) == 0 {
			continue
		}

		// Copy the line: scanner reuses its buffer between iterations
		batch = append(batch, string(lineBytes))
		result.Processed++

		if len(batch) >= p.config.BatchSize {
			if err := flush(); err != nil {
				return result, err
			}
		}
	}

	// Process remaining lines
	if err := flush(); err != nil {
		return result, err
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return result, err
	}

	result.Duration = time.Since(startTime)
	if result.Processed > 0 {
		result.ErrorRate = float64(result.Failed) / float64(result.Processed)
	}

	return result, nil
}

// StreamParse parses the whole stream and returns all entries in one slice.
// Prefer StreamParseBatches for large inputs: this variant accumulates every
// entry in memory and is only appropriate for bounded inputs such as
// incremental tails.
func (p *Parser) StreamParse(ctx context.Context, reader io.Reader) (*ParseResult, error) {
	entries := make([]*AccessLogEntry, 0, 10000)
	result, err := p.StreamParseBatches(ctx, reader, func(batch []*AccessLogEntry) error {
		entries = append(entries, batch...)
		return nil
	})
	result.Entries = entries
	return result, err
}

// ChunkedParseStream is a deprecated alias of StreamParse kept for
// compatibility; the chunkSize parameter is ignored. The single streaming
// implementation already reads with a bounded buffer.
func (p *Parser) ChunkedParseStream(ctx context.Context, reader io.Reader, _ int) (*ParseResult, error) {
	return p.StreamParse(ctx, reader)
}

// MemoryEfficientParseStream is a deprecated alias of StreamParse kept for
// compatibility. Use StreamParseBatches for genuinely bounded-memory parsing.
func (p *Parser) MemoryEfficientParseStream(ctx context.Context, reader io.Reader) (*ParseResult, error) {
	return p.StreamParse(ctx, reader)
}
