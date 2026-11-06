package logstore

import (
	"os"
	"path/filepath"
)

// Compact merges runs of adjacent small blocks of one file's segment into
// blocks of up to BlockRows rows. Incremental ingestion flushes whatever it
// read on each run, so a busy log accumulates many small blocks; each costs a
// header, a dictionary per column and a separate zstd frame.
//
// The segment is rewritten into a new generation and swapped in atomically.
// Queries already running keep reading the old file until they finish.
// Compact reports whether it rewrote anything.
func (s *Store) Compact(file string) (bool, error) {
	seg := s.Segment(file)
	if seg == nil {
		return false, nil
	}
	seg.writeMu.Lock()
	defer seg.writeMu.Unlock()

	st := seg.state.Load()
	if st == nil {
		return false, nil
	}
	runs := planCompaction(st.blocks, s.opts.BlockRows)
	if len(runs) == len(st.blocks) {
		return false, nil
	}

	// The new generation is written under a temporary name and renamed into
	// place only once complete, so a crash mid-compaction never leaves a
	// truncated newest generation for Open to prefer over the intact one.
	finalPath := filepath.Join(seg.dir, segmentFileName(seg.id, st.gen+1))
	next, err := createSegmentFileAt(finalPath+tmpExt, st.gen+1, seg.Group, seg.File)
	if err != nil {
		return false, err
	}
	fail := func(err error) (bool, error) {
		next.h.remove.Store(true)
		next.h.release()
		return false, err
	}

	for _, run := range runs {
		var data []byte
		if len(run) == 1 {
			// A block that is not merged is copied byte for byte.
			m := run[0]
			data = make([]byte, m.length)
			if _, err := st.h.f.ReadAt(data, m.offset); err != nil {
				return fail(err)
			}
		} else {
			b := newBlockBuilder()
			for _, m := range run {
				r := newBlockReader(s, seg, st, m)
				for i := 0; i < m.rows; i++ {
					row, err := r.row(i, allColumns())
					if err != nil {
						return fail(err)
					}
					b.add(row, m.sourceEnd)
				}
			}
			data = b.encode(s.enc)
		}
		meta, err := parseBlockHeader(data, next.size)
		if err != nil {
			return fail(err)
		}
		if _, err := next.h.f.WriteAt(data, next.size); err != nil {
			return fail(err)
		}
		meta.firstOrdinal = next.rows()
		next.blocks = append(next.blocks, meta)
		next.size += int64(len(data))
	}
	if err := next.h.f.Sync(); err != nil {
		return fail(err)
	}
	// Close before renaming: Windows refuses to rename an open file.
	if err := next.h.f.Close(); err != nil {
		return fail(err)
	}
	if err := os.Rename(next.h.path, finalPath); err != nil {
		_ = os.Remove(next.h.path)
		return false, err
	}
	f, err := os.OpenFile(finalPath, os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	next.h = newFileHandle(f, finalPath)

	// Publish the new generation, then retire the old file: it is removed
	// once the last query reading it finishes. A crash before the removal
	// leaves both generations, and Open keeps the newer one.
	seg.state.Store(next)
	st.h.remove.Store(true)
	st.h.release()
	syncDir(seg.dir)
	return true, nil
}

// planCompaction groups adjacent blocks into runs whose combined rows fit in
// blockRows. A block already at least half full is left on its own.
func planCompaction(blocks []*blockMeta, blockRows int) [][]*blockMeta {
	var runs [][]*blockMeta
	var cur []*blockMeta
	rows := 0
	flush := func() {
		if len(cur) > 0 {
			runs = append(runs, cur)
			cur, rows = nil, 0
		}
	}
	for _, b := range blocks {
		if b.rows >= blockRows/2 {
			flush()
			runs = append(runs, []*blockMeta{b})
			continue
		}
		if rows+b.rows > blockRows {
			flush()
		}
		cur = append(cur, b)
		rows += b.rows
	}
	flush()
	return runs
}

// syncDir makes a rename or create durable where the platform allows it.
func syncDir(dir string) {
	if d, err := os.Open(filepath.Clean(dir)); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}
