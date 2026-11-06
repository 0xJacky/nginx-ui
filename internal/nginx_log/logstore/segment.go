package logstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

const (
	segmentMagic = "NXLSEG"
	// segmentVersion 2 adds the content fingerprint to the header.
	segmentVersion = 2
	segmentExt     = ".seg"
	tmpExt         = ".tmp"
)

// fileHandle is a reference-counted segment file. The segment holds one
// reference for as long as the file is current; every query holds one while
// it reads. A replaced or deleted file is closed, and removed if asked, only
// when its last reader is done, which also keeps Windows from refusing to
// delete an open file.
type fileHandle struct {
	f      *os.File
	path   string
	refs   atomic.Int64
	remove atomic.Bool
}

func newFileHandle(f *os.File, path string) *fileHandle {
	h := &fileHandle{f: f, path: path}
	h.refs.Store(1)
	return h
}

// tryAcquire takes a reference unless the handle has already been released.
func (h *fileHandle) tryAcquire() bool {
	for {
		n := h.refs.Load()
		if n <= 0 {
			return false
		}
		if h.refs.CompareAndSwap(n, n+1) {
			return true
		}
	}
}

func (h *fileHandle) release() {
	if h.refs.Add(-1) != 0 {
		return
	}
	_ = h.f.Close()
	if h.remove.Load() {
		_ = os.Remove(h.path)
	}
}

// segState is an immutable view of a segment. Appends publish a new state that
// shares the file handle; compaction publishes one with a new file.
type segState struct {
	h      *fileHandle
	gen    uint64
	blocks []*blockMeta
	size   int64 // append position
}

func (st *segState) rows() int64 {
	if len(st.blocks) == 0 {
		return 0
	}
	last := st.blocks[len(st.blocks)-1]
	return last.firstOrdinal + int64(last.rows)
}

// Segment holds every block ingested from one log content: the lines of one
// file, wherever rotation moves that file.
//
// A segment written through ContentWriter is identified by its group and the
// fingerprint of the content's first line, so a file renamed by logrotate,
// copied by copytruncate or compressed to .gz is still the same segment and is
// read on from where it stopped. Its path follows the file and can change.
type Segment struct {
	Group       string // main log path of the group the file belongs to
	Fingerprint string // first-line fingerprint, empty for path-keyed segments

	path    atomic.Pointer[string]
	dir     string
	id      string
	writeMu sync.Mutex
	state   atomic.Pointer[segState]
}

// Path is the physical file the segment's content lives in now. It is empty
// once that file is gone and another content took its path.
func (s *Segment) Path() string {
	if p := s.path.Load(); p != nil {
		return *p
	}
	return ""
}

func (s *Segment) setPath(path string) { s.path.Store(&path) }

// SegmentInfo summarizes a segment.
type SegmentInfo struct {
	Group        string
	File         string
	Rows         int64
	Blocks       int
	Bytes        int64
	MinTs, MaxTs int64
	SourceOffset int64
}

// Info reports the segment's current size and coverage.
func (s *Segment) Info() SegmentInfo {
	info := SegmentInfo{Group: s.Group, File: s.Path()}
	st := s.acquire()
	if st == nil {
		return info
	}
	defer st.h.release()
	info.Rows = st.rows()
	info.Blocks = len(st.blocks)
	info.Bytes = st.size
	for i, b := range st.blocks {
		if i == 0 || b.minTs < info.MinTs {
			info.MinTs = b.minTs
		}
		if i == 0 || b.maxTs > info.MaxTs {
			info.MaxTs = b.maxTs
		}
		info.SourceOffset = max(info.SourceOffset, b.sourceEnd)
	}
	return info
}

// SourceOffset is the byte offset in the source log just after the last line
// stored in this segment. Incremental ingestion resumes from here.
func (s *Segment) SourceOffset() int64 {
	return s.Info().SourceOffset
}

// acquire returns the current state with a reference held, or nil once the
// segment has been deleted. The caller must release st.h.
func (s *Segment) acquire() *segState {
	for {
		st := s.state.Load()
		if st == nil {
			return nil
		}
		if st.h.tryAcquire() {
			return st
		}
		// The handle was retired between the load and the acquire; the
		// replacement state is already published, so load again.
	}
}

func segmentFileName(id string, gen uint64) string {
	return fmt.Sprintf("%s.%d%s", id, gen, segmentExt)
}

func encodeSegmentHeader(group, file, fingerprint string) []byte {
	h := append([]byte(segmentMagic), segmentVersion)
	for _, v := range []string{group, file, fingerprint} {
		h = binary.AppendUvarint(h, uint64(len(v)))
		h = append(h, v...)
	}
	return h
}

// createSegmentFile writes a new, empty segment generation.
func createSegmentFile(dir, id string, gen uint64, group, file, fingerprint string) (*segState, error) {
	return createSegmentFileAt(filepath.Join(dir, segmentFileName(id, gen)), gen, group, file, fingerprint)
}

func createSegmentFileAt(path string, gen uint64, group, file, fingerprint string) (*segState, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, err
	}
	header := encodeSegmentHeader(group, file, fingerprint)
	if _, err := f.Write(header); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return &segState{h: newFileHandle(f, path), gen: gen, size: int64(len(header))}, nil
}

// loadSegmentFile opens a segment file, reads its block directory and drops a
// torn or corrupt tail left by a crash.
func loadSegmentFile(path string, gen uint64) (group, file, fingerprint string, st *segState, err error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return "", "", "", nil, err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
		}
	}()

	fi, err := f.Stat()
	if err != nil {
		return "", "", "", nil, err
	}
	size := fi.Size()

	head := make([]byte, min(size, 64*1024))
	if _, err := f.ReadAt(head, 0); err != nil && !errors.Is(err, io.EOF) {
		return "", "", "", nil, err
	}
	if len(head) < len(segmentMagic)+1 || string(head[:len(segmentMagic)]) != segmentMagic {
		return "", "", "", nil, fmt.Errorf("logstore: %s is not a segment", path)
	}
	if head[len(segmentMagic)] != segmentVersion {
		return "", "", "", nil, fmt.Errorf("logstore: %s has unsupported version %d", path, head[len(segmentMagic)])
	}
	r := headerReader{buf: head[len(segmentMagic)+1:]}
	readString := func() string {
		l := r.uvarint()
		if r.err != nil || l > uint64(len(r.buf)) {
			r.err = errCorruptBlock
			return ""
		}
		s := string(r.buf[:l])
		r.buf = r.buf[l:]
		return s
	}
	group = readString()
	file = readString()
	fingerprint = readString()
	if r.err != nil {
		return "", "", "", nil, fmt.Errorf("logstore: %s has a corrupt header", path)
	}
	off := int64(len(head) - len(r.buf))

	var blocks []*blockMeta
	var ordinal int64
	prefix := make([]byte, len(blockMagic)+binary.MaxVarintLen64)
	for off < size {
		meta, ok := readBlockMeta(f, off, size, prefix)
		if !ok {
			break
		}
		meta.firstOrdinal = ordinal
		ordinal += int64(meta.rows)
		blocks = append(blocks, meta)
		off += meta.length
	}

	// Only the last block can be torn by a crash during an append; verify its
	// checksum and drop it if the write did not complete.
	if n := len(blocks); n > 0 && !blockChecksumOK(f, blocks[n-1]) {
		off = blocks[n-1].offset
		blocks = blocks[:n-1]
	}
	if off < size {
		if err := f.Truncate(off); err != nil {
			return "", "", "", nil, err
		}
	}
	return group, file, fingerprint, &segState{h: newFileHandle(f, path), gen: gen, blocks: blocks, size: off}, nil
}

func readBlockMeta(f *os.File, off, size int64, prefix []byte) (*blockMeta, bool) {
	n, _ := f.ReadAt(prefix, off)
	if n < len(blockMagic)+1 || [4]byte(prefix[:4]) != blockMagic {
		return nil, false
	}
	headerLen, ln := binary.Uvarint(prefix[4:n])
	if ln <= 0 || headerLen > 1<<20 {
		return nil, false
	}
	buf := make([]byte, 4+ln+int(headerLen))
	if _, err := f.ReadAt(buf, off); err != nil {
		return nil, false
	}
	meta, err := parseBlockHeader(buf, off)
	if err != nil || meta.offset+meta.length > size {
		return nil, false
	}
	return meta, true
}

func blockChecksumOK(f *os.File, m *blockMeta) bool {
	buf := make([]byte, m.length)
	if _, err := f.ReadAt(buf, m.offset); err != nil {
		return false
	}
	_, n := binary.Uvarint(buf[4:])
	body := buf[4+n : len(buf)-4]
	return crc32.Checksum(body, crcTable) == binary.LittleEndian.Uint32(buf[len(buf)-4:])
}

// appendBlock writes an encoded block at the end of the segment and publishes
// it to readers. The caller must hold writeMu.
func (s *Segment) appendBlock(data []byte, sync bool) error {
	st := s.state.Load()
	if st == nil {
		return ErrSegmentDeleted
	}
	meta, err := parseBlockHeader(data, st.size)
	if err != nil {
		return err
	}
	if _, err := st.h.f.WriteAt(data, st.size); err != nil {
		return err
	}
	if sync {
		if err := st.h.f.Sync(); err != nil {
			return err
		}
	}
	meta.firstOrdinal = st.rows()
	// Readers hold the old slice header and never look past its length, so
	// appending in place is safe.
	s.state.Store(&segState{h: st.h, gen: st.gen, blocks: append(st.blocks, meta), size: st.size + int64(len(data))})
	return nil
}

// rollback removes the given blocks, which must be the segment's newest, and
// truncates the file to where the first of them started.
func (s *Segment) rollback(blocks []*blockMeta) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	st := s.state.Load()
	if st == nil {
		return nil // deleted: nothing left to roll back
	}
	first := -1
	for i, b := range st.blocks {
		if b == blocks[0] {
			first = i
			break
		}
	}
	if first < 0 || len(st.blocks)-first != len(blocks) {
		return fmt.Errorf("logstore: cannot roll back %s: its blocks were compacted or are not the newest", s.Path())
	}
	size := blocks[0].offset
	if err := st.h.f.Truncate(size); err != nil {
		return err
	}
	// Copy, so a later append cannot overwrite entries that readers of the
	// old state still see.
	kept := append([]*blockMeta(nil), st.blocks[:first]...)
	s.state.Store(&segState{h: st.h, gen: st.gen, blocks: kept, size: size})
	return nil
}

// ErrSegmentDeleted is returned when writing to a segment that was deleted.
var ErrSegmentDeleted = errors.New("logstore: segment deleted")

// retire unpublishes the segment and deletes its file once readers are done.
func (s *Segment) retire() {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	st := s.state.Swap(nil)
	if st == nil {
		return
	}
	st.h.remove.Store(true)
	st.h.release()
}
