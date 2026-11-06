package logstore

import (
	"bytes"
	"fmt"
	"math/bits"
	"net/netip"
	"sort"
	"strings"
	"sync"
)

// Query selects rows. Every non-empty criterion must hold (AND); values inside
// one criterion are alternatives (OR).
type Query struct {
	Groups []string // main log paths
	Files  []string // physical file paths

	// From and To bound the timestamp to [From, To). Zero leaves that side
	// unbounded.
	From, To int64

	// Text matches the raw line. It is split on whitespace, with double
	// quotes keeping a phrase together; every term must occur in the line,
	// compared case-insensitively (ASCII) as a substring.
	Text string

	IPs         []string
	Methods     []string
	RegionCodes []string
	Provinces   []string
	Cities      []string
	Browsers    []string
	OSs         []string
	Devices     []string
	Levels      []string // error log severities
	Statuses    []int

	// Substring filters, case-insensitive (ASCII); any needle may match.
	PathContains      []string
	UserAgentContains []string
	RefererContains   []string

	MinBytes, MaxBytes             *int64
	MinRequestTime, MaxRequestTime *float64 // seconds

	// Clauses come from the search box syntax (ParseSearchText) and combine
	// with everything above.
	Clauses []Clause
}

type exactFilter struct {
	col    Column
	values map[string]struct{}
}

type containsFilter struct {
	col     Column
	needles []string // lower case
}

// compiledQuery is a Query prepared for per-block evaluation.
type compiledQuery struct {
	groups, files map[string]struct{}
	from, to      int64
	exact         []exactFilter
	contains      []containsFilter
	statuses      *bitset // indexed by status code
	minBytes      *uint64
	maxBytes      *uint64
	minReqMs      *uint64
	maxReqMs      *uint64
	terms         [][]byte // lower case
	notTerms      [][]byte // lower case, excluded
	dictClauses   []dictClause
	rangeClauses  []rangeClause
	matchNone     bool // a criterion that can never match
}

func toSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	m := make(map[string]struct{}, len(values))
	for _, v := range values {
		m[v] = struct{}{}
	}
	return m
}

func compile(q *Query) *compiledQuery {
	c := &compiledQuery{
		groups: toSet(q.Groups),
		files:  toSet(q.Files),
		from:   q.From,
		to:     q.To,
	}
	for _, f := range []struct {
		col    Column
		values []string
	}{
		{ColIP, q.IPs}, {ColMethod, q.Methods}, {ColRegionCode, q.RegionCodes}, {ColProvince, q.Provinces},
		{ColCity, q.Cities}, {ColBrowser, q.Browsers}, {ColOS, q.OSs}, {ColDeviceType, q.Devices},
		{ColLevel, q.Levels},
	} {
		if len(f.values) > 0 {
			c.exact = append(c.exact, exactFilter{col: f.col, values: toSet(f.values)})
		}
	}
	for _, f := range []struct {
		col     Column
		needles []string
	}{{ColPath, q.PathContains}, {ColUserAgent, q.UserAgentContains}, {ColReferer, q.RefererContains}} {
		var needles []string
		for _, n := range f.needles {
			if n != "" {
				needles = append(needles, lowerASCII(n))
			}
		}
		if len(needles) > 0 {
			c.contains = append(c.contains, containsFilter{col: f.col, needles: needles})
		}
	}
	if len(q.Statuses) > 0 {
		set := newBitset(1<<16, false)
		valid := false
		for _, s := range q.Statuses {
			if s >= 0 && s < 1<<16 {
				set.set(s)
				valid = true
			}
		}
		c.statuses = &set
		c.matchNone = c.matchNone || !valid
	}
	if q.MinBytes != nil {
		v := uint64(max(*q.MinBytes, 0))
		c.minBytes = &v
	}
	if q.MaxBytes != nil {
		if *q.MaxBytes < 0 {
			c.matchNone = true
		}
		v := uint64(max(*q.MaxBytes, 0))
		c.maxBytes = &v
	}
	if q.MinRequestTime != nil {
		v := secondsToMs(*q.MinRequestTime)
		c.minReqMs = &v
	}
	if q.MaxRequestTime != nil {
		v := secondsToMs(*q.MaxRequestTime)
		c.maxReqMs = &v
	}
	for _, t := range splitTerms(q.Text) {
		c.terms = append(c.terms, []byte(lowerASCII(t)))
	}
	for _, cl := range q.Clauses {
		c.addClause(cl)
	}
	return c
}

// dictClause is a search box filter on a dictionary column, evaluated once
// per dictionary value.
type dictClause struct {
	col   Column
	match func(string) bool
}

// rangeClause is a search box filter on a numeric column.
type rangeClause struct {
	col    Column
	lo, hi uint64
	negate bool
}

func (c *compiledQuery) addClause(cl Clause) {
	switch cl.Kind {
	case ClauseText:
		if cl.Text == "" {
			return
		}
		term := []byte(lowerASCII(cl.Text))
		if cl.Negate {
			c.notTerms = append(c.notTerms, term)
		} else {
			c.terms = append(c.terms, term)
		}
	case ClauseRange:
		c.rangeClauses = append(c.rangeClauses, rangeClause{col: cl.Column, lo: cl.Lo, hi: cl.Hi, negate: cl.Negate})
	case ClauseContains, ClauseEquals, ClauseIP:
		var match func(string) bool
		switch cl.Kind {
		case ClauseContains:
			needle := lowerASCII(cl.Text)
			match = func(v string) bool { return containsFoldASCII(v, needle) }
		case ClauseEquals:
			match = func(v string) bool { return strings.EqualFold(v, cl.Text) }
		default:
			prefix := cl.Prefix
			match = func(v string) bool {
				a, err := netip.ParseAddr(v)
				return err == nil && prefix.Contains(a.Unmap())
			}
		}
		if cl.Negate {
			inner := match
			match = func(v string) bool { return !inner(v) }
		}
		c.dictClauses = append(c.dictClauses, dictClause{col: cl.Column, match: match})
	}
}

// splitTerms splits on whitespace, keeping double-quoted phrases together.
func splitTerms(s string) []string {
	var out []string
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		if s[0] == '"' {
			if end := strings.IndexByte(s[1:], '"'); end >= 0 {
				if phrase := s[1 : end+1]; phrase != "" {
					out = append(out, phrase)
				}
				s = s[end+2:]
				continue
			}
			s = s[1:]
			continue
		}
		end := strings.IndexFunc(s, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' })
		if end < 0 {
			end = len(s)
		}
		out = append(out, s[:end])
		s = s[end:]
	}
	return out
}

func (c *compiledQuery) segmentMatches(seg *Segment) bool {
	if c.groups != nil {
		if _, ok := c.groups[seg.Group]; !ok {
			return false
		}
	}
	if c.files != nil {
		if _, ok := c.files[seg.Path()]; !ok {
			return false
		}
	}
	return true
}

// blockOverlaps reports whether the block can hold rows in [from, to).
func (c *compiledQuery) blockOverlaps(m *blockMeta) bool {
	if c.from != 0 && m.maxTs < c.from {
		return false
	}
	if c.to != 0 && m.minTs >= c.to {
		return false
	}
	return true
}

// evaluate returns the rows of the block that match, cheapest criteria first.
// A nil error with an empty bitset means nothing in the block matches.
func (c *compiledQuery) evaluate(r *blockReader) (bitset, error) {
	rows := r.m.rows
	if c.matchNone {
		return newBitset(rows, false), nil
	}
	sel := newBitset(rows, true)

	if (c.from != 0 && r.m.minTs < c.from) || (c.to != 0 && r.m.maxTs >= c.to) {
		ts, err := r.timestamps()
		if err != nil {
			return nil, err
		}
		for i, t := range ts {
			if (c.from != 0 && t < c.from) || (c.to != 0 && t >= c.to) {
				sel.clear(i)
			}
		}
		if sel.empty() {
			return sel, nil
		}
	}

	for _, f := range c.exact {
		if err := c.filterDict(r, sel, f.col, func(v string) bool {
			_, ok := f.values[v]
			return ok
		}); err != nil || sel.empty() {
			return sel, err
		}
	}

	if c.statuses != nil {
		st, err := r.statuses()
		if err != nil {
			return nil, err
		}
		for i, s := range st {
			if !c.statuses.get(int(s)) {
				sel.clear(i)
			}
		}
		if sel.empty() {
			return sel, nil
		}
	}

	if err := c.filterRange(r, sel, ColBytesSent, c.minBytes, c.maxBytes); err != nil || sel.empty() {
		return sel, err
	}
	if err := c.filterRange(r, sel, ColRequestTime, c.minReqMs, c.maxReqMs); err != nil || sel.empty() {
		return sel, err
	}

	for _, f := range c.contains {
		if err := c.filterDict(r, sel, f.col, func(v string) bool {
			for _, n := range f.needles {
				if containsFoldASCII(v, n) {
					return true
				}
			}
			return false
		}); err != nil || sel.empty() {
			return sel, err
		}
	}

	for _, dc := range c.dictClauses {
		if err := c.filterDict(r, sel, dc.col, dc.match); err != nil || sel.empty() {
			return sel, err
		}
	}
	for _, rc := range c.rangeClauses {
		if err := c.filterRangeClause(r, sel, rc); err != nil || sel.empty() {
			return sel, err
		}
	}

	if len(c.terms) > 0 || len(c.notTerms) > 0 {
		if err := c.filterText(r, sel); err != nil {
			return nil, err
		}
	}
	return sel, nil
}

// filterRangeClause keeps the rows whose column is within the clause's
// bounds, or outside them when the clause is negated.
func (c *compiledQuery) filterRangeClause(r *blockReader, sel bitset, rc rangeClause) error {
	in := func(v uint64) bool { return (v >= rc.lo && v <= rc.hi) != rc.negate }
	if rc.col == ColStatus {
		st, err := r.statuses()
		if err != nil {
			return err
		}
		sel.forEach(func(i int) {
			if !in(uint64(st[i])) {
				sel.clear(i)
			}
		})
		return nil
	}
	vals, err := r.uvarints(rc.col)
	if err != nil {
		return err
	}
	sel.forEach(func(i int) {
		if !in(vals[i]) {
			sel.clear(i)
		}
	})
	return nil
}

// filterDict evaluates match once per dictionary value, then clears every
// selected row whose id did not match. A block whose dictionary has no match
// is rejected without reading its ids.
func (c *compiledQuery) filterDict(r *blockReader, sel bitset, col Column, match func(string) bool) error {
	dict, err := r.dict(col)
	if err != nil {
		return err
	}
	ok := make([]bool, len(dict))
	anyMatch, allMatch := false, true
	for i, v := range dict {
		ok[i] = match(v)
		anyMatch = anyMatch || ok[i]
		allMatch = allMatch && ok[i]
	}
	if !anyMatch {
		sel.reset()
		return nil
	}
	if allMatch {
		return nil
	}
	ids, err := r.idColumn(col)
	if err != nil {
		return err
	}
	sel.forEach(func(i int) {
		if !ok[ids[i]] {
			sel.clear(i)
		}
	})
	return nil
}

func (c *compiledQuery) filterRange(r *blockReader, sel bitset, col Column, lo, hi *uint64) error {
	if lo == nil && hi == nil {
		return nil
	}
	vals, err := r.uvarints(col)
	if err != nil {
		return err
	}
	sel.forEach(func(i int) {
		v := vals[i]
		if (lo != nil && v < *lo) || (hi != nil && v > *hi) {
			sel.clear(i)
		}
	})
	return nil
}

// filterText keeps rows whose raw line contains every term. It lowers a copy
// of the block's text once and searches it as a whole, so the work is one
// bytes.Index pass per term instead of one per row.
func (c *compiledQuery) filterText(r *blockReader, sel bitset) error {
	r.store.textSlots <- struct{}{}
	defer func() { <-r.store.textSlots }()

	// The text is decoded into a pooled buffer and lower-cased in place, and
	// nothing keeps it once the block is filtered: the few rows a search
	// shows read their lines again. Keeping it on the reader held a block's
	// whole raw column per worker until the worker moved on.
	rawBuf := rawBufPool.Get().(*[]byte)
	defer rawBufPool.Put(rawBuf)
	lower, _, err := r.readInto(ColRaw, payloadValues, *rawBuf)
	if err != nil {
		return err
	}
	*rawBuf = lower
	lines := make([]int, 0, r.m.rows+1)
	for i, b := range lower {
		switch {
		case 'A' <= b && b <= 'Z':
			lower[i] = b + 'a' - 'A'
		case b == '\n':
			lines = append(lines, i+1)
		}
	}
	if len(lines) != r.m.rows {
		return fmt.Errorf("%w: raw has %d lines, want %d", errCorruptBlock, len(lines), r.m.rows)
	}
	lines = append([]int{0}, lines...)
	hits := newBitset(r.m.rows, false)
	markHits := func(term []byte) {
		hits.reset()
		for off := 0; off < len(lower); {
			j := bytes.Index(lower[off:], term)
			if j < 0 {
				break
			}
			pos := off + j
			row := sort.SearchInts(lines, pos+1) - 1
			// A match that runs over the line end belongs to no line.
			if pos+len(term) < lines[row+1] {
				hits.set(row)
			}
			off = max(lines[row+1], pos+1)
		}
	}
	for _, term := range c.terms {
		markHits(term)
		sel.and(hits)
		if sel.empty() {
			return nil
		}
	}
	for _, term := range c.notTerms {
		markHits(term)
		sel.andNot(hits)
		if sel.empty() {
			return nil
		}
	}
	return nil
}

// rawBufPool recycles the buffers free-text filters decode raw text into,
// which are as large as a block's raw column.
var rawBufPool = sync.Pool{New: func() any { b := make([]byte, 0, 1<<20); return &b }}

func lowerASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if 'A' <= s[i] && s[i] <= 'Z' {
			b := []byte(s)
			for j := i; j < len(b); j++ {
				if 'A' <= b[j] && b[j] <= 'Z' {
					b[j] += 'a' - 'A'
				}
			}
			return string(b)
		}
	}
	return s
}

// containsFoldASCII reports whether s contains the lower-case needle,
// ignoring ASCII case. nginx escapes non-ASCII bytes in access logs, so ASCII
// folding covers what appears there.
func containsFoldASCII(s, needle string) bool {
	n := len(needle)
	if n == 0 {
		return true
	}
	for i := 0; i+n <= len(s); i++ {
		j := 0
		for ; j < n; j++ {
			b := s[i+j]
			if 'A' <= b && b <= 'Z' {
				b += 'a' - 'A'
			}
			if b != needle[j] {
				break
			}
		}
		if j == n {
			return true
		}
	}
	return false
}

// bitset is a fixed-size row selection.
type bitset []uint64

func newBitset(n int, full bool) bitset {
	b := make(bitset, (n+63)/64)
	if full {
		for i := range b {
			b[i] = ^uint64(0)
		}
		if r := n % 64; r != 0 {
			b[len(b)-1] = 1<<r - 1
		}
	}
	return b
}

func (b bitset) set(i int)      { b[i>>6] |= 1 << (i & 63) }
func (b bitset) clear(i int)    { b[i>>6] &^= 1 << (i & 63) }
func (b bitset) get(i int) bool { return b[i>>6]&(1<<(i&63)) != 0 }

func (b bitset) reset() {
	for i := range b {
		b[i] = 0
	}
}

func (b bitset) and(o bitset) {
	for i := range b {
		b[i] &= o[i]
	}
}

func (b bitset) andNot(o bitset) {
	for i := range b {
		b[i] &^= o[i]
	}
}

func (b bitset) count() int {
	n := 0
	for _, w := range b {
		n += bits.OnesCount64(w)
	}
	return n
}

func (b bitset) empty() bool {
	for _, w := range b {
		if w != 0 {
			return false
		}
	}
	return true
}

// forEach calls fn for every set bit in ascending order. fn may clear the bit
// it is called for.
func (b bitset) forEach(fn func(i int)) {
	for wi, w := range b {
		for w != 0 {
			t := bits.TrailingZeros64(w)
			fn(wi<<6 | t)
			w &= w - 1
		}
	}
}
