package logstore

import (
	"bytes"
	"fmt"
)

// blockReader decodes the columns of one block on demand and keeps what it
// decoded for the rest of the block's evaluation.
type blockReader struct {
	store *Store
	seg   *Segment
	st    *segState
	m     *blockMeta

	ts     []int64
	status []uint16
	uv     [numColumns][]uint64
	dicts  [numColumns][]string
	ids    [numColumns][]uint32
	raw    []byte
	lines  []int // start offset of every line, plus one past the end
}

func newBlockReader(s *Store, seg *Segment, st *segState, m *blockMeta) *blockReader {
	return &blockReader{store: s, seg: seg, st: st, m: m}
}

func (r *blockReader) read(col Column, kind uint8) ([]byte, payloadInfo, error) {
	p, ok := r.m.payload(col, kind)
	if !ok {
		return nil, p, fmt.Errorf("%w: missing column %s", errCorruptBlock, col)
	}
	if p.rawLen == 0 {
		return nil, p, nil
	}
	comp := make([]byte, p.compLen)
	if _, err := r.st.h.f.ReadAt(comp, p.offset); err != nil {
		return nil, p, err
	}
	out, err := r.store.dec.DecodeAll(comp, make([]byte, 0, p.rawLen))
	if err != nil {
		return nil, p, fmt.Errorf("logstore: decode %s of %s: %w", col, r.seg.Path(), err)
	}
	if uint64(len(out)) != p.rawLen {
		return nil, p, fmt.Errorf("%w: %s length", errCorruptBlock, col)
	}
	return out, p, nil
}

func (r *blockReader) timestamps() ([]int64, error) {
	if r.ts != nil {
		return r.ts, nil
	}
	raw, _, err := r.read(ColTimestamp, payloadValues)
	if err != nil {
		return nil, err
	}
	r.ts, err = decodeTimestamps(raw, r.m.rows, r.m.minTs)
	return r.ts, err
}

func (r *blockReader) statuses() ([]uint16, error) {
	if r.status != nil {
		return r.status, nil
	}
	raw, _, err := r.read(ColStatus, payloadValues)
	if err != nil {
		return nil, err
	}
	r.status, err = decodeStatus(raw, r.m.rows)
	return r.status, err
}

// uvarints decodes bytes_sent, request_time (ms) or upstream_time (ms+1).
func (r *blockReader) uvarints(col Column) ([]uint64, error) {
	if r.uv[col] != nil {
		return r.uv[col], nil
	}
	raw, _, err := r.read(col, payloadValues)
	if err != nil {
		return nil, err
	}
	r.uv[col], err = decodeUvarints(raw, r.m.rows)
	return r.uv[col], err
}

func (r *blockReader) dict(col Column) ([]string, error) {
	if r.dicts[col] != nil {
		return r.dicts[col], nil
	}
	raw, _, err := r.read(col, payloadValues)
	if err != nil {
		return nil, err
	}
	r.dicts[col], err = decodeDict(raw)
	return r.dicts[col], err
}

func (r *blockReader) idColumn(col Column) ([]uint32, error) {
	if r.ids[col] != nil {
		return r.ids[col], nil
	}
	raw, p, err := r.read(col, payloadIDs)
	if err != nil {
		return nil, err
	}
	r.ids[col], err = decodeIDs(raw, p.width, r.m.rows)
	return r.ids[col], err
}

// rawText returns the block's raw lines, newline-terminated, and the start
// offset of every line.
func (r *blockReader) rawText() ([]byte, []int, error) {
	if r.lines != nil {
		return r.raw, r.lines, nil
	}
	raw, _, err := r.read(ColRaw, payloadValues)
	if err != nil {
		return nil, nil, err
	}
	lines := make([]int, 0, r.m.rows+1)
	for off := 0; off < len(raw); {
		lines = append(lines, off)
		i := bytes.IndexByte(raw[off:], '\n')
		if i < 0 {
			return nil, nil, fmt.Errorf("%w: raw column", errCorruptBlock)
		}
		off += i + 1
	}
	if len(lines) != r.m.rows {
		return nil, nil, fmt.Errorf("%w: raw has %d lines, want %d", errCorruptBlock, len(lines), r.m.rows)
	}
	r.raw, r.lines = raw, append(lines, len(raw))
	return r.raw, r.lines, nil
}

func (r *blockReader) rawLine(i int) (string, error) {
	raw, lines, err := r.rawText()
	if err != nil {
		return "", err
	}
	return string(raw[lines[i] : lines[i+1]-1]), nil
}

// row materializes one row, decoding only the columns in want.
func (r *blockReader) row(i int, want *columnSet) (*Row, error) {
	row := &Row{}
	if want.has(ColTimestamp) {
		ts, err := r.timestamps()
		if err != nil {
			return nil, err
		}
		row.Timestamp = ts[i]
	}
	if want.has(ColStatus) {
		st, err := r.statuses()
		if err != nil {
			return nil, err
		}
		row.Status = int(st[i])
	}
	if want.has(ColBytesSent) {
		v, err := r.uvarints(ColBytesSent)
		if err != nil {
			return nil, err
		}
		row.BytesSent = int64(v[i])
	}
	if want.has(ColRequestTime) {
		v, err := r.uvarints(ColRequestTime)
		if err != nil {
			return nil, err
		}
		row.RequestTime = float64(v[i]) / 1000
	}
	if want.has(ColUpstreamTime) {
		v, err := r.uvarints(ColUpstreamTime)
		if err != nil {
			return nil, err
		}
		if v[i] > 0 {
			up := float64(v[i]-1) / 1000
			row.UpstreamTime = &up
		}
	}
	if want.has(ColRaw) {
		line, err := r.rawLine(i)
		if err != nil {
			return nil, err
		}
		row.Raw = line
	}
	for c := ColIP; c < numColumns; c++ {
		if !want.has(c) {
			continue
		}
		dict, err := r.dict(c)
		if err != nil {
			return nil, err
		}
		ids, err := r.idColumn(c)
		if err != nil {
			return nil, err
		}
		if int(ids[i]) >= len(dict) {
			return nil, fmt.Errorf("%w: %s id out of range", errCorruptBlock, c)
		}
		setDictValue(row, c, dict[ids[i]])
	}
	return row, nil
}

// columnSet is a set of columns.
type columnSet uint32

func allColumns() *columnSet {
	s := columnSet(1<<numColumns - 1)
	return &s
}

func (s *columnSet) add(c Column)      { *s |= 1 << c }
func (s *columnSet) has(c Column) bool { return *s&(1<<c) != 0 }
