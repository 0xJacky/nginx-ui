package logstore

import (
	"encoding/binary"
	"math/bits"
	"sync"
)

// A gram filter lets a free-text search skip a block without reading its raw
// text. It records which lower-cased trigrams (three consecutive bytes of one
// line) occur in the block, hashed into a bitmap of about eight bits per
// distinct trigram. A term can only occur in a line that holds every trigram
// of the term, so a term with one trigram missing from the bitmap is absent
// from the block. The bitmap admits false positives, a block scanned in vain,
// and never false negatives, so substring semantics are kept exactly.
//
// The filter is the gram payload of the raw column:
//
//	log2(bits) | bitmap, little-endian 64-bit words
//
// Readers that predate it ignore the payload kind, and a block without it is
// scanned as before.

const (
	gramBitsPerGram = 8
	gramMinLog2     = 13 // 1 KiB
	gramMaxLog2     = 22 // 512 KiB
	gramSpace       = 1 << 24
)

// gramSeenPool recycles the exact trigram sets blocks are sealed with.
var gramSeenPool = sync.Pool{New: func() any { s := make([]uint64, gramSpace/64); return &s }}

func gramOf(a, b, c byte) uint32 { return uint32(a)<<16 | uint32(b)<<8 | uint32(c) }

// gramSlot hashes a trigram into a bitmap of 1<<log2 bits.
func gramSlot(g uint32, log2 uint8) uint32 { return (g * 0x9E3779B1) >> (32 - log2) }

func lowerByte(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + 'a' - 'A'
	}
	return b
}

// buildGramFilter returns the gram payload of '\n'-terminated raw text.
func buildGramFilter(raw []byte) []byte {
	seenp := gramSeenPool.Get().(*[]uint64)
	defer gramSeenPool.Put(seenp)
	seen := *seenp

	distinct := 0
	var a, b byte
	run := 0 // bytes of the current line seen so far
	for _, c := range raw {
		if c == '\n' {
			run = 0
			continue
		}
		c = lowerByte(c)
		if run >= 2 {
			g := gramOf(a, b, c)
			if w, m := g>>6, uint64(1)<<(g&63); seen[w]&m == 0 {
				seen[w] |= m
				distinct++
			}
		}
		a, b = b, c
		run++
	}

	log2 := uint8(gramMinLog2)
	if want := distinct * gramBitsPerGram; want > 1<<gramMinLog2 {
		log2 = uint8(min(bits.Len(uint(want-1)), gramMaxLog2))
	}
	filter := make([]uint64, (1<<log2)/64)
	for w, word := range seen {
		for word != 0 {
			g := uint32(w<<6 | bits.TrailingZeros64(word))
			slot := gramSlot(g, log2)
			filter[slot>>6] |= 1 << (slot & 63)
			word &= word - 1
		}
		seen[w] = 0
	}

	out := make([]byte, 1+8*len(filter))
	out[0] = log2
	for i, word := range filter {
		binary.LittleEndian.PutUint64(out[1+8*i:], word)
	}
	return out
}

// gramFilter is a decoded gram payload.
type gramFilter struct {
	log2 uint8
	bits []byte
}

// parseGramFilter decodes a gram payload. A payload it cannot read is
// reported as absent, which only costs a scan.
func parseGramFilter(p []byte) (gramFilter, bool) {
	if len(p) < 1 || p[0] < gramMinLog2 || p[0] > gramMaxLog2 || len(p) != 1+(1<<p[0])/8 {
		return gramFilter{}, false
	}
	return gramFilter{log2: p[0], bits: p[1:]}, true
}

// mayContain reports whether a line of the block can hold the lower-case
// term. A term shorter than a trigram always may.
func (f gramFilter) mayContain(grams []uint32) bool {
	for _, g := range grams {
		slot := gramSlot(g, f.log2)
		if f.bits[slot>>3]&(1<<(slot&7)) == 0 {
			return false
		}
	}
	return true
}

// termGrams lists the distinct trigrams of a lower-case term.
func termGrams(term []byte) []uint32 {
	var out []uint32
	for i := 0; i+3 <= len(term); i++ {
		g := gramOf(term[i], term[i+1], term[i+2])
		dup := false
		for _, o := range out {
			dup = dup || o == g
		}
		if !dup {
			out = append(out, g)
		}
	}
	return out
}
