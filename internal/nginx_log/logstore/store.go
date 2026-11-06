package logstore

import (
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// DefaultBlockRows is the number of rows a block holds before it is sealed.
const DefaultBlockRows = 64 * 1024

// Options configures a Store.
type Options struct {
	// BlockRows caps the rows per block; it also bounds ingest memory, which
	// is about one block per file being written. Default DefaultBlockRows.
	BlockRows int
	// Parallelism is the number of blocks a query evaluates at once. Callers
	// in containers should pass the cgroup CPU budget. Default GOMAXPROCS.
	Parallelism int
	// SyncWrites fsyncs every appended block.
	SyncWrites bool
}

// Store is a set of segments, one per physical log file, grouped by the main
// log path they belong to.
type Store struct {
	dir  string
	opts Options
	enc  *zstd.Encoder
	dec  *zstd.Decoder

	mu       sync.RWMutex
	segments map[string]*Segment // by file path
	closed   bool
}

// Open opens or creates a store rooted at dir.
func Open(dir string, opts Options) (*Store, error) {
	if opts.BlockRows <= 0 {
		opts.BlockRows = DefaultBlockRows
	}
	if opts.Parallelism <= 0 {
		opts.Parallelism = runtime.GOMAXPROCS(0)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	enc, err := zstd.NewWriter(nil,
		zstd.WithEncoderLevel(zstd.SpeedDefault),
		zstd.WithEncoderConcurrency(min(opts.Parallelism, 4)),
		zstd.WithLowerEncoderMem(true))
	if err != nil {
		return nil, err
	}
	dec, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(opts.Parallelism))
	if err != nil {
		_ = enc.Close()
		return nil, err
	}

	s := &Store{dir: dir, opts: opts, enc: enc, dec: dec, segments: make(map[string]*Segment)}
	if err := s.load(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// load rebuilds the catalog from the segment files on disk. Where a
// compaction left two generations of one segment, the newest one that loads
// wins and the others are removed.
func (s *Store) load() error {
	groupDirs, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	for _, gd := range groupDirs {
		if !gd.IsDir() {
			continue
		}
		dir := filepath.Join(s.dir, gd.Name())
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		gens := map[string][]uint64{}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), tmpExt) {
				// An unfinished compaction output.
				_ = os.Remove(filepath.Join(dir, e.Name()))
				continue
			}
			id, gen, ok := parseSegmentFileName(e.Name())
			if ok {
				gens[id] = append(gens[id], gen)
			}
		}
		for id, list := range gens {
			sort.Slice(list, func(i, j int) bool { return list[i] > list[j] })
			var loaded bool
			for _, gen := range list {
				path := filepath.Join(dir, segmentFileName(id, gen))
				if loaded {
					_ = os.Remove(path)
					continue
				}
				group, file, st, err := loadSegmentFile(path, gen)
				if err != nil {
					// A half-written compaction output; an older generation
					// still holds the data.
					_ = os.Remove(path)
					continue
				}
				seg := &Segment{Group: group, File: file, dir: dir, id: id}
				seg.state.Store(st)
				s.segments[file] = seg
				loaded = true
			}
		}
	}
	return nil
}

func parseSegmentFileName(name string) (id string, gen uint64, ok bool) {
	base, found := strings.CutSuffix(name, segmentExt)
	if !found {
		return "", 0, false
	}
	id, genStr, found := strings.Cut(base, ".")
	if !found {
		return "", 0, false
	}
	gen, err := strconv.ParseUint(genStr, 10, 64)
	return id, gen, err == nil
}

func hashName(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return fmt.Sprintf("%016x", h.Sum64())
}

// ErrClosed is returned by operations on a closed store.
var ErrClosed = errors.New("logstore: store closed")

// segment returns the segment for file, creating it inside group if needed.
func (s *Store) segment(group, file string) (*Segment, error) {
	s.mu.RLock()
	seg, ok := s.segments[file]
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return nil, ErrClosed
	}
	if ok {
		if seg.Group != group {
			return nil, fmt.Errorf("logstore: %s already belongs to group %s", file, seg.Group)
		}
		return seg, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if seg, ok := s.segments[file]; ok {
		return seg, nil
	}
	dir := filepath.Join(s.dir, hashName(group))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	seg = &Segment{Group: group, File: file, dir: dir, id: hashName(file)}
	st, err := createSegmentFile(dir, seg.id, 0, group, file)
	if err != nil {
		return nil, err
	}
	seg.state.Store(st)
	s.segments[file] = seg
	return seg, nil
}

// Segment returns the segment for a physical file, or nil.
func (s *Store) Segment(file string) *Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.segments[file]
}

// Segments lists every segment, optionally restricted to one group.
func (s *Store) Segments(group string) []*Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Segment, 0, len(s.segments))
	for _, seg := range s.segments {
		if group == "" || seg.Group == group {
			out = append(out, seg)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out
}

// DeleteFile drops everything ingested from one physical file.
func (s *Store) DeleteFile(file string) {
	s.mu.Lock()
	seg := s.segments[file]
	delete(s.segments, file)
	s.mu.Unlock()
	if seg != nil {
		seg.retire()
		// Removes the group directory once its last segment is gone.
		_ = os.Remove(seg.dir)
	}
}

// DeleteGroup drops every file of a log group.
func (s *Store) DeleteGroup(group string) {
	for _, seg := range s.Segments(group) {
		s.DeleteFile(seg.File)
	}
}

// Close releases the store. Queries still running keep their files open
// until they finish.
func (s *Store) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	segs := s.segments
	s.segments = map[string]*Segment{}
	s.mu.Unlock()

	for _, seg := range segs {
		seg.writeMu.Lock()
		if st := seg.state.Swap(nil); st != nil {
			st.h.release()
		}
		seg.writeMu.Unlock()
	}
	if s.enc != nil {
		_ = s.enc.Close()
	}
	if s.dec != nil {
		s.dec.Close()
	}
	return nil
}

// Writer appends rows to one file's segment. A Writer is not safe for
// concurrent use; use one per file.
type Writer struct {
	store    *Store
	seg      *Segment
	b        *blockBuilder
	appended []*blockMeta // blocks this writer flushed, for Abort
}

// Writer returns a writer that appends rows read from file, a member of group.
func (s *Store) Writer(group, file string) (*Writer, error) {
	seg, err := s.segment(group, file)
	if err != nil {
		return nil, err
	}
	return &Writer{store: s, seg: seg, b: newBlockBuilder()}, nil
}

// Append buffers a row. sourceEnd is the byte offset in the source file just
// after the row's line; it is what SourceOffset reports once flushed.
//
// A full block is sealed when the next row arrives rather than right away,
// so the final SetSourceEnd of a read always has buffered rows to land on.
func (w *Writer) Append(r *Row, sourceEnd int64) error {
	if w.b.rows >= w.store.opts.BlockRows {
		if err := w.Flush(); err != nil {
			return err
		}
	}
	w.b.add(r, sourceEnd)
	return nil
}

// Flush seals the buffered rows into a block, making them visible to queries.
func (w *Writer) Flush() error {
	if w.b.rows == 0 {
		return nil
	}
	data := w.b.encode(w.store.enc)
	w.b = newBlockBuilder()

	w.seg.writeMu.Lock()
	defer w.seg.writeMu.Unlock()
	if err := w.seg.appendBlock(data, w.store.opts.SyncWrites); err != nil {
		return err
	}
	st := w.seg.state.Load()
	w.appended = append(w.appended, st.blocks[len(st.blocks)-1])
	return nil
}

// Buffered reports rows appended but not yet flushed.
func (w *Writer) Buffered() int { return w.b.rows }

// SetSourceEnd records that the source file has been read up to offset. It
// applies to the rows buffered now, so call it before the final Flush of a
// read. Blocks flushed earlier in the same read keep the offset they had.
func (w *Writer) SetSourceEnd(offset int64) {
	w.b.sourceEnd = max(w.b.sourceEnd, offset)
}

// Abort discards the buffered rows and removes every block this writer has
// flushed, so a read that failed halfway leaves no partial rows behind to be
// duplicated by the retry. Queries running at that moment may fail once.
func (w *Writer) Abort() error {
	w.b = newBlockBuilder()
	if len(w.appended) == 0 {
		return nil
	}
	defer func() { w.appended = nil }()
	return w.seg.rollback(w.appended)
}

// DeleteAll drops every segment. The store stays open.
func (s *Store) DeleteAll() {
	for _, seg := range s.Segments("") {
		s.DeleteFile(seg.File)
	}
}
