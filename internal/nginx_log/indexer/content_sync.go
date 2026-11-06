package indexer

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/utils"
	"github.com/uozi-tech/cosy/logger"
)

// Content tracking of log files, after the log analytics plugins.
//
// A file's content is identified by a fingerprint of its first line, and the
// store records, per content, how far into the (decompressed) content it has
// read. A file renamed by logrotate, copied by copytruncate or compressed to
// .gz is the same content under another path: it is read on from where the
// store stopped instead of being indexed again. A plain file is read only up
// to its last newline, so a line nginx is still writing waits for the next
// round. Because the store is append-only, the offset must be exact: a failed
// read rolls back what it wrote, and a content shorter than its offset was
// rewritten and is replaced.
const (
	// fingerprintProbe bounds how much of a file is read to find its first
	// line. A first line longer than this is fingerprinted by its prefix.
	fingerprintProbe = 64 * 1024
	// fingerprintLen is the number of hex characters kept of the hash.
	fingerprintLen = 16
)

// contentLocks serializes the reads of one content. Two paths can hold the
// same content for a moment (copytruncate, or gzip writing x.gz next to x),
// and both would otherwise append the same lines from the same offset.
var contentLocks sync.Map // group+fingerprint -> *sync.Mutex

func lockContent(group, fingerprint string) func() {
	v, _ := contentLocks.LoadOrStore(group+"\x00"+fingerprint, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// isGzip reports whether the file starts with the gzip magic number. The name
// is not trusted: rotation may compress without renaming, or rename without
// compressing.
func isGzip(f *os.File) bool {
	var magic [2]byte
	n, _ := f.ReadAt(magic[:], 0)
	return n == 2 && magic[0] == 0x1f && magic[1] == 0x8b
}

// openContent returns the content of f from offset on: the file itself for a
// plain file, the decompressed stream for a gzip file.
func openContent(f *os.File, compressed bool, offset int64) (io.Reader, func(), error) {
	if !compressed {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, nil, err
		}
		return f, func() {}, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, err
	}
	gz, err := gzip.NewReader(bufio.NewReader(f))
	if err != nil {
		return nil, nil, err
	}
	if offset > 0 {
		if _, err := io.CopyN(io.Discard, gz, offset); err != nil {
			_ = gz.Close()
			return nil, nil, fmt.Errorf("skip to offset %d: %w", offset, err)
		}
	}
	return gz, func() { _ = gz.Close() }, nil
}

// contentFingerprint hashes the first complete line of a content. It returns
// "" when the content has no complete line yet.
func contentFingerprint(f *os.File, compressed bool) (string, error) {
	r, closeFn, err := openContent(f, compressed, 0)
	if err != nil {
		return "", err
	}
	defer closeFn()
	buf := make([]byte, fingerprintProbe)
	n, err := io.ReadFull(r, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", err
	}
	buf = buf[:n]
	if i := bytes.IndexByte(buf, '\n'); i >= 0 {
		buf = buf[:i]
	} else if n < fingerprintProbe {
		return "", nil // the first line is not finished yet
	}
	sum := sha256.Sum256(buf)
	return hex.EncodeToString(sum[:])[:fingerprintLen], nil
}

// gzipContentSize is the uncompressed size modulo 2^32 that a gzip file
// records in its trailer.
func gzipContentSize(f *os.File, size int64) (uint32, bool) {
	if size < 18 {
		return 0, false
	}
	var tail [4]byte
	if _, err := f.ReadAt(tail[:], size-4); err != nil {
		return 0, false
	}
	return binary.LittleEndian.Uint32(tail[:]), true
}

// lastLineEnd returns the offset just after the last newline in [start, size)
// of a plain file, or start when there is none.
func lastLineEnd(f *os.File, start, size int64) (int64, error) {
	const chunk = 64 * 1024
	buf := make([]byte, chunk)
	for end := size; end > start; {
		from := max(start, end-chunk)
		n, err := f.ReadAt(buf[:end-from], from)
		if err != nil && !errors.Is(err, io.EOF) {
			return start, err
		}
		if i := bytes.LastIndexByte(buf[:n], '\n'); i >= 0 {
			return from + int64(i) + 1, nil
		}
		end = from
	}
	return start, nil
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// syncFile brings the store up to date with one physical file and returns
// the documents it added and their time range.
func (ci *ColumnarIndexer) syncFile(filePath string, progressTracker *ProgressTracker) (uint64, *time.Time, *time.Time, error) {
	store, err := ci.liveStore()
	if err != nil {
		return 0, nil, nil, err
	}
	if !utils.IsValidLogPath(filePath) {
		return 0, nil, nil, fmt.Errorf("invalid log path: %s", filePath)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to open log file %s: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}
	size := info.Size()
	compressed := isGzip(f)
	group := getMainLogPathFromFile(filePath)

	if progressTracker != nil {
		progressTracker.SetFileSize(filePath, size)
		progressTracker.SetFileEstimate(filePath, max(size/150, 100))
	}

	fingerprint, err := contentFingerprint(f, compressed)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("failed to read the start of %s: %w", filePath, err)
	}
	if fingerprint == "" {
		return 0, nil, nil, nil // no complete line yet
	}

	unlock := lockContent(group, fingerprint)
	defer unlock()

	start := int64(0)
	if seg := store.Content(group, fingerprint); seg != nil {
		start = seg.SourceOffset()
		if !compressed && start > size {
			// Shorter than what was read of it: the file was rewritten with
			// a line that happens to match, so its rows are replaced.
			store.DeleteSegment(seg)
			start = 0
		}
	}

	// Claim the content for this path before anything is read, so a renamed
	// file is filed under its new path even when nothing new is in it.
	w, err := store.ContentWriter(group, filePath, fingerprint)
	if err != nil {
		return 0, nil, nil, err
	}

	end := size
	switch {
	case compressed:
		// A compressed copy that holds exactly what was read already needs
		// no decompression: the trailer records the content size.
		if want, ok := gzipContentSize(f, size); ok && start > 0 && start < 1<<32 && want == uint32(start) {
			return 0, nil, nil, nil
		}
	default:
		if end, err = lastLineEnd(f, start, size); err != nil {
			return 0, nil, nil, err
		}
		if end <= start {
			ci.recordReadPosition(filePath, start)
			return 0, nil, nil, nil
		}
	}

	r, closeContent, err := openContent(f, compressed, start)
	if err != nil {
		return 0, nil, nil, err
	}
	defer closeContent()
	if !compressed {
		r = io.LimitReader(r, end-start)
	}
	counted := &countingReader{r: r}

	logger.Infof("Indexing %s from offset %d", filePath, start)
	var (
		docs             uint64
		minTime, maxTime *time.Time
		invalid          int
		lines            int64
	)
	add := func(doc *LogDocument) error {
		if !isValidLogEntry(doc) {
			invalid++
			return nil
		}
		ts := time.Unix(doc.Timestamp, 0)
		if minTime == nil || ts.Before(*minTime) {
			minTime = &ts
		}
		if maxTime == nil || ts.After(*maxTime) {
			maxTime = &ts
		}
		if err := w.Append(LogDocumentToRow(doc), 0); err != nil {
			return err
		}
		docs++
		return nil
	}
	progress := func(n int) {
		lines += int64(n)
		if progressTracker != nil {
			progressTracker.UpdateFileProgress(filePath, lines, min(start+counted.n, size))
		}
	}
	if ci.errorLog(group) {
		err = parseErrorLog(counted, filePath, group, add, progress)
	} else {
		_, _, err = ParseLogStreamBatches(context.Background(), counted, filePath, func(batch []*LogDocument) error {
			for _, doc := range batch {
				if err := add(doc); err != nil {
					return err
				}
			}
			progress(len(batch))
			return nil
		})
	}
	if err == nil {
		w.SetSourceEnd(start + counted.n)
		err = w.Flush()
	}
	if err != nil {
		if abortErr := w.Abort(); abortErr != nil {
			logger.Warnf("Failed to discard the partial index of %s: %v", filePath, abortErr)
		}
		return 0, nil, nil, fmt.Errorf("failed to index %s: %w", filePath, err)
	}
	ci.docsIndexed.Add(int64(docs))

	if !compressed {
		ci.recordReadPosition(filePath, start+counted.n)
	}
	if invalid > 0 {
		logger.Warnf("File %s: filtered out %d invalid entries", filePath, invalid)
	}
	logger.Infof("Finished %s: %d new documents, read to offset %d", filePath, docs, start+counted.n)
	return docs, minTime, maxTime, nil
}

// pruneVanished drops the rows of a group's contents whose file is gone:
// logrotate deleted the oldest archive, or new content took the path of a
// content that was not found under any other name.
func (ci *ColumnarIndexer) pruneVanished(group string, present []string) {
	store, err := ci.liveStore()
	if err != nil {
		return
	}
	alive := make(map[string]bool, len(present))
	for _, p := range present {
		alive[p] = true
	}
	for _, seg := range store.Segments(group) {
		if !alive[seg.Path()] {
			logger.Infof("Dropping the index of %s content that is no longer on disk", group)
			store.DeleteSegment(seg)
		}
	}
}

// parseErrorLog reads nginx error log entries. A line that does not start
// like an entry, such as the continuation of a multi-line message, is
// skipped.
func parseErrorLog(r io.Reader, filePath, group string, add func(*LogDocument) error, progress func(int)) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	n := 0
	for scanner.Scan() {
		line := scanner.Text()
		n++
		entry, ok := parser.ParseErrorLine(line)
		if !ok {
			continue
		}
		doc := &LogDocument{
			Timestamp:   entry.Timestamp,
			IP:          entry.Client,
			Method:      entry.Method,
			Path:        entry.Path,
			PathExact:   entry.Path,
			Referer:     entry.Referrer,
			Level:       entry.Level,
			Raw:         line,
			FilePath:    filePath,
			MainLogPath: group,
		}
		if err := add(doc); err != nil {
			return err
		}
		if n%10000 == 0 {
			progress(10000)
		}
	}
	progress(n % 10000)
	return scanner.Err()
}
