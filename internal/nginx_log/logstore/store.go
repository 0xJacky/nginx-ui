package logstore

import (
	"encoding/json"
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
	// TextScanParallelism caps how many blocks scan their raw text at once.
	// Each holds the block's decompressed lines and a lower-cased copy, about
	// twice the raw size of 64k lines, so free-text queries on a many-core
	// host would otherwise multiply that by every worker. Default 2.
	TextScanParallelism int
}

// Store is a set of segments, one per physical log file, grouped by the main
// log path they belong to.
type Store struct {
	dir  string
	opts Options
	enc  *zstd.Encoder
	dec  *zstd.Decoder

	mu        sync.RWMutex
	segments  map[string]*Segment // by id
	byPath    map[string]*Segment // by current file path
	byContent map[string]*Segment // by group and fingerprint
	closed    bool

	// textSlots bounds concurrent raw text scans; see TextScanParallelism.
	textSlots chan struct{}
}

// Open opens or creates a store rooted at dir.
func Open(dir string, opts Options) (*Store, error) {
	if opts.BlockRows <= 0 {
		opts.BlockRows = DefaultBlockRows
	}
	if opts.Parallelism <= 0 {
		opts.Parallelism = runtime.GOMAXPROCS(0)
	}
	if opts.TextScanParallelism <= 0 {
		opts.TextScanParallelism = 2
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

	s := &Store{
		dir: dir, opts: opts, enc: enc, dec: dec,
		segments:  make(map[string]*Segment),
		byPath:    make(map[string]*Segment),
		byContent: make(map[string]*Segment),
		textSlots: make(chan struct{}, min(opts.TextScanParallelism, opts.Parallelism)),
	}
	if err := s.load(); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// catalogFile records the current path of every segment whose content moved
// since it was created; the segment header keeps the path it was created at.
const catalogFile = "paths.json"

// load rebuilds the catalog from the segment files on disk. Where a
// compaction left two generations of one segment, the newest one that loads
// wins and the others are removed.
func (s *Store) load() error {
	moved := map[string]string{}
	if data, err := os.ReadFile(filepath.Join(s.dir, catalogFile)); err == nil {
		_ = json.Unmarshal(data, &moved)
	}

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
				group, file, fingerprint, st, err := loadSegmentFile(path, gen)
				if err != nil {
					// A half-written compaction output, or a segment from an
					// older format; an older generation may still hold data.
					_ = os.Remove(path)
					continue
				}
				if current, ok := moved[id]; ok {
					file = current
				}
				seg := &Segment{Group: group, Fingerprint: fingerprint, dir: dir, id: id}
				seg.setPath(file)
				seg.state.Store(st)
				s.register(seg)
				loaded = true
			}
		}
	}
	return nil
}

func contentKey(group, fingerprint string) string { return group + "\x00" + fingerprint }

// register adds a segment to the catalog. The caller holds mu or owns s.
func (s *Store) register(seg *Segment) {
	s.segments[seg.id] = seg
	if p := seg.Path(); p != "" {
		if other := s.byPath[p]; other != nil && other != seg {
			// Two segments claim the path only after a crash between a
			// rotation and the catalog write; the newer content keeps it.
			other.setPath("")
		}
		s.byPath[p] = seg
	}
	if seg.Fingerprint != "" {
		s.byContent[contentKey(seg.Group, seg.Fingerprint)] = seg
	}
}

// unregister removes a segment from the catalog. The caller holds mu.
func (s *Store) unregister(seg *Segment) {
	delete(s.segments, seg.id)
	if p := seg.Path(); p != "" && s.byPath[p] == seg {
		delete(s.byPath, p)
	}
	if seg.Fingerprint != "" && s.byContent[contentKey(seg.Group, seg.Fingerprint)] == seg {
		delete(s.byContent, contentKey(seg.Group, seg.Fingerprint))
	}
}

// saveCatalog persists the current path of every segment. The caller holds mu.
func (s *Store) saveCatalog() error {
	paths := make(map[string]string, len(s.segments))
	for id, seg := range s.segments {
		paths[id] = seg.Path()
	}
	data, err := json.Marshal(paths)
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, catalogFile+tmpExt)
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, catalogFile))
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

// createSegment makes and registers an empty segment. The caller holds mu.
func (s *Store) createSegment(group, file, fingerprint string) (*Segment, error) {
	dir := filepath.Join(s.dir, hashName(group))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	id := hashName(file)
	if fingerprint != "" {
		id = hashName(contentKey(group, fingerprint))
	}
	seg := &Segment{Group: group, Fingerprint: fingerprint, dir: dir, id: id}
	seg.setPath(file)
	st, err := createSegmentFile(dir, id, 0, group, file, fingerprint)
	if err != nil {
		return nil, err
	}
	seg.state.Store(st)
	s.register(seg)
	return seg, nil
}

// segment returns the path-keyed segment for file, creating it if needed.
func (s *Store) segment(group, file string) (*Segment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if seg := s.byPath[file]; seg != nil {
		if seg.Group != group {
			return nil, fmt.Errorf("logstore: %s already belongs to group %s", file, seg.Group)
		}
		return seg, nil
	}
	return s.createSegment(group, file, "")
}

// Content returns the segment holding the content with this first-line
// fingerprint in group, or nil.
func (s *Store) Content(group, fingerprint string) *Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byContent[contentKey(group, fingerprint)]
}

// ContentWriter returns a writer for the content identified by fingerprint,
// which currently lives in file. The content's segment is created when it is
// new and moved to file when the content was seen under another path: the
// rotated file keeps its rows and is read on from where it stopped. A segment
// that held file before, with another content, loses the path but keeps its
// rows in the group.
func (s *Store) ContentWriter(group, file, fingerprint string) (*Writer, error) {
	if fingerprint == "" {
		return nil, fmt.Errorf("logstore: content of %s has no fingerprint", file)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	seg := s.byContent[contentKey(group, fingerprint)]
	if seg == nil {
		if previous := s.byPath[file]; previous != nil {
			previous.setPath("")
			delete(s.byPath, file)
		}
		var err error
		if seg, err = s.createSegment(group, file, fingerprint); err != nil {
			return nil, err
		}
		if err := s.saveCatalog(); err != nil {
			return nil, err
		}
	} else if seg.Path() != file {
		if previous := s.byPath[file]; previous != nil && previous != seg {
			previous.setPath("")
		}
		if old := seg.Path(); old != "" && s.byPath[old] == seg {
			delete(s.byPath, old)
		}
		seg.setPath(file)
		s.byPath[file] = seg
		if err := s.saveCatalog(); err != nil {
			return nil, err
		}
	}
	return &Writer{store: s, seg: seg, b: newBlockBuilder()}, nil
}

// Segment returns the segment whose content lives in a physical file, or nil.
func (s *Store) Segment(file string) *Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byPath[file]
}

// Segments lists every segment, optionally restricted to one group. It
// includes segments whose file is gone (empty Path).
func (s *Store) Segments(group string) []*Segment {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Segment, 0, len(s.segments))
	for _, seg := range s.segments {
		if group == "" || seg.Group == group {
			out = append(out, seg)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path() != out[j].Path() {
			return out[i].Path() < out[j].Path()
		}
		return out[i].id < out[j].id
	})
	return out
}

// DeleteSegment drops a segment and its rows.
func (s *Store) DeleteSegment(seg *Segment) {
	s.mu.Lock()
	if s.segments[seg.id] != seg {
		s.mu.Unlock()
		return
	}
	s.unregister(seg)
	_ = s.saveCatalog()
	s.mu.Unlock()
	seg.retire()
	// Removes the group directory once its last segment is gone.
	_ = os.Remove(seg.dir)
}

// DeleteFile drops the rows of the content that lives in file.
func (s *Store) DeleteFile(file string) {
	if seg := s.Segment(file); seg != nil {
		s.DeleteSegment(seg)
	}
}

// DeleteGroup drops every segment of a log group, including the content of
// files that are gone.
func (s *Store) DeleteGroup(group string) {
	for _, seg := range s.Segments(group) {
		s.DeleteSegment(seg)
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
	s.byPath = map[string]*Segment{}
	s.byContent = map[string]*Segment{}
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
		s.DeleteSegment(seg)
	}
}
