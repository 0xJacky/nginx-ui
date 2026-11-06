package logstore

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"strings"

	"github.com/klauspost/compress/zstd"
)

var (
	blockMagic = [4]byte{'B', 'L', 'K', '1'}
	crcTable   = crc32.MakeTable(crc32.Castagnoli)

	errCorruptBlock = errors.New("logstore: corrupt block")
)

// Payload kinds. A dictionary column has a values payload (the dictionary)
// and an ids payload; every other column has only a values payload, and the
// raw column a gram filter besides (grams.go).
const (
	payloadValues uint8 = 0
	payloadIDs    uint8 = 1
	payloadGrams  uint8 = 2
)

// payloadInfo locates one compressed payload of a block.
type payloadInfo struct {
	col     Column
	kind    uint8
	width   uint8 // id width in bytes, ids payload only
	rawLen  uint64
	compLen uint64
	offset  int64 // absolute file offset, filled in when the block is loaded
}

// blockMeta is the in-memory description of a block on disk.
type blockMeta struct {
	offset       int64
	length       int64
	rows         int
	minTs, maxTs int64
	sourceEnd    int64
	firstOrdinal int64 // ordinal of the block's first row within its segment
	payloads     []payloadInfo
	index        [numColumns][3]int16
}

func (m *blockMeta) payload(col Column, kind uint8) (payloadInfo, bool) {
	i := m.index[col][kind]
	if i < 0 {
		return payloadInfo{}, false
	}
	return m.payloads[i], true
}

func (m *blockMeta) buildIndex() {
	for c := range m.index {
		m.index[c] = [3]int16{-1, -1, -1}
	}
	for i, p := range m.payloads {
		if p.col < numColumns && p.kind <= payloadGrams {
			m.index[p.col][p.kind] = int16(i)
		}
	}
}

// dictBuilder accumulates one dictionary column of a block.
type dictBuilder struct {
	ids    map[string]uint32
	values []string
	rowIDs []uint32
}

func (d *dictBuilder) add(v string) {
	id, ok := d.ids[v]
	if !ok {
		id = uint32(len(d.values))
		// Parsed values often point into the whole line; clone so the block
		// does not pin the parser's buffers.
		v = strings.Clone(v)
		d.ids[v] = id
		d.values = append(d.values, v)
	}
	d.rowIDs = append(d.rowIDs, id)
}

// blockBuilder accumulates rows column by column until they are encoded.
//
// It never retains a Row: memory is the raw bytes, one id per row and column,
// and the distinct values of each dictionary column.
type blockBuilder struct {
	rows         int
	minTs, maxTs int64
	sourceEnd    int64
	ts           []int64
	status       []uint16
	bytesSent    []uint64
	reqMs        []uint64
	upMs         []uint64 // 0 = absent, else ms+1
	raw          []byte
	dicts        [numColumns]*dictBuilder
}

func newBlockBuilder() *blockBuilder {
	b := &blockBuilder{}
	for c := ColIP; c < numColumns; c++ {
		b.dicts[c] = &dictBuilder{ids: make(map[string]uint32)}
	}
	return b
}

func (b *blockBuilder) add(r *Row, sourceEnd int64) {
	if b.rows == 0 || r.Timestamp < b.minTs {
		b.minTs = r.Timestamp
	}
	if b.rows == 0 || r.Timestamp > b.maxTs {
		b.maxTs = r.Timestamp
	}
	b.rows++
	b.sourceEnd = max(b.sourceEnd, sourceEnd)

	b.ts = append(b.ts, r.Timestamp)
	b.status = append(b.status, uint16(min(max(r.Status, 0), math.MaxUint16)))
	b.bytesSent = append(b.bytesSent, uint64(max(r.BytesSent, 0)))
	b.reqMs = append(b.reqMs, secondsToMs(r.RequestTime))
	if r.UpstreamTime != nil {
		b.upMs = append(b.upMs, secondsToMs(*r.UpstreamTime)+1)
	} else {
		b.upMs = append(b.upMs, 0)
	}
	b.raw = append(b.raw, sanitizeRaw(r.Raw)...)
	b.raw = append(b.raw, '\n')

	for c := ColIP; c < numColumns; c++ {
		b.dicts[c].add(dictValue(r, c))
	}
}

func secondsToMs(v float64) uint64 {
	if v <= 0 || math.IsNaN(v) {
		return 0
	}
	return uint64(math.Round(v * 1000))
}

// encode serializes the block:
//
//	"BLK1" | uvarint headerLen | header | payload* | crc32c(header+payloads)
func (b *blockBuilder) encode(enc *zstd.Encoder) []byte {
	type part struct {
		info payloadInfo
		data []byte
	}
	parts := make([]part, 0, 7+2*int(numColumns-ColIP))
	addPart := func(col Column, kind, width uint8, raw []byte) {
		parts = append(parts, part{
			info: payloadInfo{col: col, kind: kind, width: width, rawLen: uint64(len(raw))},
			data: enc.EncodeAll(raw, nil),
		})
	}

	scratch := make([]byte, 0, b.rows*binary.MaxVarintLen64)

	prev := b.minTs
	for _, t := range b.ts {
		scratch = binary.AppendVarint(scratch, t-prev)
		prev = t
	}
	addPart(ColTimestamp, payloadValues, 0, scratch)

	status := make([]byte, 2*b.rows)
	for i, s := range b.status {
		binary.LittleEndian.PutUint16(status[2*i:], s)
	}
	addPart(ColStatus, payloadValues, 0, status)

	for _, c := range []struct {
		col  Column
		vals []uint64
	}{{ColBytesSent, b.bytesSent}, {ColRequestTime, b.reqMs}, {ColUpstreamTime, b.upMs}} {
		scratch = scratch[:0]
		for _, v := range c.vals {
			scratch = binary.AppendUvarint(scratch, v)
		}
		addPart(c.col, payloadValues, 0, scratch)
	}

	addPart(ColRaw, payloadValues, 0, b.raw)
	addPart(ColRaw, payloadGrams, 0, buildGramFilter(b.raw))

	for c := ColIP; c < numColumns; c++ {
		d := b.dicts[c]
		scratch = binary.AppendUvarint(scratch[:0], uint64(len(d.values)))
		for _, v := range d.values {
			scratch = binary.AppendUvarint(scratch, uint64(len(v)))
			scratch = append(scratch, v...)
		}
		addPart(c, payloadValues, 0, scratch)

		width := idWidth(len(d.values))
		ids := make([]byte, int(width)*len(d.rowIDs))
		for i, id := range d.rowIDs {
			switch width {
			case 1:
				ids[i] = byte(id)
			case 2:
				binary.LittleEndian.PutUint16(ids[2*i:], uint16(id))
			default:
				binary.LittleEndian.PutUint32(ids[4*i:], id)
			}
		}
		addPart(c, payloadIDs, width, ids)
	}

	header := binary.AppendUvarint(nil, uint64(b.rows))
	header = binary.AppendVarint(header, b.minTs)
	header = binary.AppendVarint(header, b.maxTs)
	header = binary.AppendVarint(header, b.sourceEnd)
	header = binary.AppendUvarint(header, uint64(len(parts)))
	total := 0
	for i := range parts {
		p := &parts[i]
		p.info.compLen = uint64(len(p.data))
		header = append(header, byte(p.info.col), p.info.kind, p.info.width)
		header = binary.AppendUvarint(header, p.info.rawLen)
		header = binary.AppendUvarint(header, p.info.compLen)
		total += len(p.data)
	}

	out := make([]byte, 0, len(blockMagic)+binary.MaxVarintLen64+len(header)+total+4)
	out = append(out, blockMagic[:]...)
	out = binary.AppendUvarint(out, uint64(len(header)))
	body := len(out)
	out = append(out, header...)
	for _, p := range parts {
		out = append(out, p.data...)
	}
	return binary.LittleEndian.AppendUint32(out, crc32.Checksum(out[body:], crcTable))
}

func idWidth(distinct int) uint8 {
	switch {
	case distinct <= 1<<8:
		return 1
	case distinct <= 1<<16:
		return 2
	default:
		return 4
	}
}

// parseBlockHeader decodes the header that starts at buf[0], for a block that
// starts at file offset off. It returns the metadata and the full block
// length. buf must hold at least the magic, the length prefix and the header.
func parseBlockHeader(buf []byte, off int64) (*blockMeta, error) {
	if len(buf) < len(blockMagic) || [4]byte(buf[:4]) != blockMagic {
		return nil, errCorruptBlock
	}
	p := buf[4:]
	headerLen, n := binary.Uvarint(p)
	if n <= 0 || headerLen > uint64(len(p)-n) {
		return nil, errCorruptBlock
	}
	prefix := 4 + n
	h := p[n : n+int(headerLen)]

	r := headerReader{buf: h}
	m := &blockMeta{offset: off}
	m.rows = int(r.uvarint())
	m.minTs = r.varint()
	m.maxTs = r.varint()
	m.sourceEnd = r.varint()
	count := r.uvarint()
	if r.err != nil || count > 4*uint64(numColumns) {
		return nil, errCorruptBlock
	}
	pos := off + int64(prefix) + int64(headerLen)
	m.payloads = make([]payloadInfo, count)
	for i := range m.payloads {
		pi := payloadInfo{col: Column(r.byte()), kind: r.byte(), width: r.byte()}
		pi.rawLen = r.uvarint()
		pi.compLen = r.uvarint()
		pi.offset = pos
		pos += int64(pi.compLen)
		m.payloads[i] = pi
	}
	if r.err != nil {
		return nil, errCorruptBlock
	}
	m.length = pos + 4 - off
	m.buildIndex()
	return m, nil
}

type headerReader struct {
	buf []byte
	err error
}

func (r *headerReader) uvarint() uint64 {
	v, n := binary.Uvarint(r.buf)
	if n <= 0 {
		r.err = errCorruptBlock
		return 0
	}
	r.buf = r.buf[n:]
	return v
}

func (r *headerReader) varint() int64 {
	v, n := binary.Varint(r.buf)
	if n <= 0 {
		r.err = errCorruptBlock
		return 0
	}
	r.buf = r.buf[n:]
	return v
}

func (r *headerReader) byte() uint8 {
	if len(r.buf) == 0 {
		r.err = errCorruptBlock
		return 0
	}
	b := r.buf[0]
	r.buf = r.buf[1:]
	return b
}

// decodeDict parses a dictionary payload.
func decodeDict(raw []byte) ([]string, error) {
	count, n := binary.Uvarint(raw)
	if n <= 0 || count > uint64(len(raw)) {
		return nil, fmt.Errorf("%w: dictionary", errCorruptBlock)
	}
	raw = raw[n:]
	// One allocation backs every value of the dictionary.
	all := string(raw)
	values := make([]string, count)
	off := 0
	for i := range values {
		l, n := binary.Uvarint(raw[off:])
		if n <= 0 || uint64(len(raw)-off-n) < l {
			return nil, fmt.Errorf("%w: dictionary", errCorruptBlock)
		}
		off += n
		values[i] = all[off : off+int(l)]
		off += int(l)
	}
	return values, nil
}

func decodeIDs(raw []byte, width uint8, rows int) ([]uint32, error) {
	if len(raw) != int(width)*rows {
		return nil, fmt.Errorf("%w: ids", errCorruptBlock)
	}
	ids := make([]uint32, rows)
	switch width {
	case 1:
		for i := range ids {
			ids[i] = uint32(raw[i])
		}
	case 2:
		for i := range ids {
			ids[i] = uint32(binary.LittleEndian.Uint16(raw[2*i:]))
		}
	case 4:
		for i := range ids {
			ids[i] = binary.LittleEndian.Uint32(raw[4*i:])
		}
	default:
		return nil, fmt.Errorf("%w: id width %d", errCorruptBlock, width)
	}
	return ids, nil
}

func decodeUvarints(raw []byte, rows int) ([]uint64, error) {
	out := make([]uint64, rows)
	for i := range out {
		v, n := binary.Uvarint(raw)
		if n <= 0 {
			return nil, fmt.Errorf("%w: uvarint column", errCorruptBlock)
		}
		out[i] = v
		raw = raw[n:]
	}
	return out, nil
}

func decodeTimestamps(raw []byte, rows int, minTs int64) ([]int64, error) {
	out := make([]int64, rows)
	prev := minTs
	for i := range out {
		d, n := binary.Varint(raw)
		if n <= 0 {
			return nil, fmt.Errorf("%w: timestamp column", errCorruptBlock)
		}
		prev += d
		out[i] = prev
		raw = raw[n:]
	}
	return out, nil
}

func decodeStatus(raw []byte, rows int) ([]uint16, error) {
	if len(raw) != 2*rows {
		return nil, fmt.Errorf("%w: status column", errCorruptBlock)
	}
	out := make([]uint16, rows)
	for i := range out {
		out[i] = binary.LittleEndian.Uint16(raw[2*i:])
	}
	return out, nil
}
