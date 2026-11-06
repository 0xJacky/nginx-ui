package logstore

import (
	"container/heap"
	"context"
	"fmt"
	"hash/maphash"
	"sort"
	"strconv"
	"sync"
)

// task is one block a query has to look at.
type task struct {
	seg *Segment
	st  *segState
	m   *blockMeta
}

// snapshot pins every segment the query can touch and lists the blocks that
// overlap its time range. The returned release must be called when done.
func (s *Store) snapshot(c *compiledQuery) ([]task, func(), error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, nil, ErrClosed
	}
	segs := make([]*Segment, 0, len(s.segments))
	for _, seg := range s.segments {
		if c.segmentMatches(seg) {
			segs = append(segs, seg)
		}
	}
	s.mu.RUnlock()

	var held []*segState
	var tasks []task
	for _, seg := range segs {
		st := seg.acquire()
		if st == nil {
			continue
		}
		held = append(held, st)
		for _, m := range st.blocks {
			if c.blockOverlaps(m) {
				tasks = append(tasks, task{seg: seg, st: st, m: m})
			}
		}
	}
	release := func() {
		for _, st := range held {
			st.h.release()
		}
	}
	return tasks, release, nil
}

// run evaluates the query on every task with bounded parallelism and calls fn
// with each block's selection. fn runs concurrently and must synchronize any
// state it shares. Blocks with an empty selection are not passed to fn.
func (s *Store) run(ctx context.Context, c *compiledQuery, tasks []task, fn func(r *blockReader, sel bitset) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	work := make(chan task)
	var (
		wg       sync.WaitGroup
		errOnce  sync.Once
		firstErr error
	)
	fail := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}
	for range min(s.opts.Parallelism, max(len(tasks), 1)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range work {
				if ctx.Err() != nil {
					continue
				}
				r := newBlockReader(s, t.seg, t.st, t.m)
				sel, err := c.evaluate(r)
				if err != nil {
					fail(err)
					continue
				}
				if sel.empty() {
					continue
				}
				if err := fn(r, sel); err != nil {
					fail(err)
				}
			}
		}()
	}
feed:
	for _, t := range tasks {
		select {
		case work <- t:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}

// execute runs fn over the query's blocks, then calls finish while the files
// are still pinned. finish may be nil.
func (s *Store) execute(ctx context.Context, q *Query, order func([]task), fn func(r *blockReader, sel bitset) error, finish ...func() error) error {
	c := compile(q)
	tasks, release, err := s.snapshot(c)
	if err != nil {
		return err
	}
	defer release()
	if order != nil {
		order(tasks)
	}
	if err := s.run(ctx, c, tasks, fn); err != nil {
		return err
	}
	for _, f := range finish {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}

// Count returns the number of matching rows.
func (s *Store) Count(ctx context.Context, q *Query) (uint64, error) {
	var mu sync.Mutex
	var total uint64
	err := s.execute(ctx, q, nil, func(_ *blockReader, sel bitset) error {
		n := uint64(sel.count())
		mu.Lock()
		total += n
		mu.Unlock()
		return nil
	})
	return total, err
}

// TermCount is one value of a facet.
type TermCount struct {
	Term  string
	Count int
}

// FacetResult is the value distribution of one field over the matching rows.
type FacetResult struct {
	Field    string
	Terms    []TermCount // the most frequent values, at most the requested size
	Distinct int         // number of distinct non-empty values
	Missing  int         // rows with an empty value
	Other    int         // rows whose value is not in Terms
}

// Facets returns the top values of each field over the matching rows, and the
// number of matching rows. Supported fields are the dictionary columns and
// status.
func (s *Store) Facets(ctx context.Context, q *Query, fields []string, size int) (map[string]*FacetResult, uint64, error) {
	cols := make([]Column, len(fields))
	for i, f := range fields {
		col, ok := ColumnByName(f)
		if !ok || !col.countable() {
			return nil, 0, fmt.Errorf("logstore: cannot facet on %q", f)
		}
		cols[i] = col
	}

	var mu sync.Mutex
	var total uint64
	counts := make([]map[string]int, len(cols))
	for i := range counts {
		counts[i] = map[string]int{}
	}

	err := s.execute(ctx, q, nil, func(r *blockReader, sel bitset) error {
		local := make([]map[string]int, len(cols))
		for fi, col := range cols {
			m, err := blockValueCounts(r, sel, col)
			if err != nil {
				return err
			}
			local[fi] = m
		}
		n := uint64(sel.count())
		mu.Lock()
		defer mu.Unlock()
		total += n
		for fi, m := range local {
			for v, c := range m {
				counts[fi][v] += c
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	out := make(map[string]*FacetResult, len(fields))
	for fi, f := range fields {
		res := &FacetResult{Field: f}
		terms := make([]TermCount, 0, len(counts[fi]))
		sum := 0
		for v, c := range counts[fi] {
			if v == "" {
				res.Missing += c
				continue
			}
			terms = append(terms, TermCount{Term: v, Count: c})
			sum += c
		}
		sort.Slice(terms, func(i, j int) bool {
			if terms[i].Count != terms[j].Count {
				return terms[i].Count > terms[j].Count
			}
			return terms[i].Term < terms[j].Term
		})
		res.Distinct = len(terms)
		if size > 0 && len(terms) > size {
			terms = terms[:size]
		}
		res.Terms = terms
		res.Other = sum
		for _, t := range terms {
			res.Other -= t.Count
		}
		out[f] = res
	}
	return out, total, nil
}

// blockValueCounts counts the selected rows of a block by value. It counts by
// dictionary id first, so the map is touched once per distinct value.
func blockValueCounts(r *blockReader, sel bitset, col Column) (map[string]int, error) {
	if col == ColStatus {
		st, err := r.statuses()
		if err != nil {
			return nil, err
		}
		byCode := map[uint16]int{}
		sel.forEach(func(i int) { byCode[st[i]]++ })
		m := make(map[string]int, len(byCode))
		for code, c := range byCode {
			m[strconv.Itoa(int(code))] = c
		}
		return m, nil
	}
	dict, err := r.dict(col)
	if err != nil {
		return nil, err
	}
	ids, err := r.idColumn(col)
	if err != nil {
		return nil, err
	}
	perID := make([]int, len(dict))
	sel.forEach(func(i int) { perID[ids[i]]++ })
	m := make(map[string]int)
	for id, c := range perID {
		if c > 0 {
			m[dict[id]] += c
		}
	}
	return m, nil
}

var hashSeed = maphash.MakeSeed()

// Distinct counts the distinct non-empty values of a field over the matching
// rows. The count is exact: values are tracked as 64-bit hashes, so memory is
// about 16 bytes per distinct value.
func (s *Store) Distinct(ctx context.Context, q *Query, field string) (uint64, error) {
	col, ok := ColumnByName(field)
	if !ok || !col.countable() {
		return 0, fmt.Errorf("logstore: cannot count distinct %q", field)
	}
	var mu sync.Mutex
	seen := map[uint64]struct{}{}
	err := s.execute(ctx, q, nil, func(r *blockReader, sel bitset) error {
		hashes, err := blockDistinctHashes(r, sel, col)
		if err != nil {
			return err
		}
		mu.Lock()
		for _, h := range hashes {
			seen[h] = struct{}{}
		}
		mu.Unlock()
		return nil
	})
	return uint64(len(seen)), err
}

func blockDistinctHashes(r *blockReader, sel bitset, col Column) ([]uint64, error) {
	if col == ColStatus {
		m, err := blockValueCounts(r, sel, col)
		if err != nil {
			return nil, err
		}
		out := make([]uint64, 0, len(m))
		for v := range m {
			out = append(out, maphash.String(hashSeed, v))
		}
		return out, nil
	}
	dict, err := r.dict(col)
	if err != nil {
		return nil, err
	}
	ids, err := r.idColumn(col)
	if err != nil {
		return nil, err
	}
	used := make([]bool, len(dict))
	sel.forEach(func(i int) { used[ids[i]] = true })
	out := make([]uint64, 0, len(dict))
	for id, u := range used {
		if u && dict[id] != "" {
			out = append(out, maphash.String(hashSeed, dict[id]))
		}
	}
	return out, nil
}

// Stats summarizes the numeric columns of the matching rows.
type Stats struct {
	Count          uint64
	TotalBytes     int64
	MinBytes       int64
	MaxBytes       int64
	TotalReqTime   float64
	MinReqTime     float64
	MaxReqTime     float64
	UniqueIPs      int
	UniquePaths    int
	StatusCodeDist map[string]int
	MethodDist     map[string]int
}

// Stats computes traffic and timing totals, distinct IPs and paths, and the
// status and method distributions over every matching row. Unlike the Bleve
// scan it replaces, it never samples.
func (s *Store) Stats(ctx context.Context, q *Query) (*Stats, error) {
	var mu sync.Mutex
	out := &Stats{StatusCodeDist: map[string]int{}, MethodDist: map[string]int{}}
	ips, paths := map[uint64]struct{}{}, map[uint64]struct{}{}
	var minReqMs, maxReqMs uint64
	first := true

	err := s.execute(ctx, q, nil, func(r *blockReader, sel bitset) error {
		bs, err := r.uvarints(ColBytesSent)
		if err != nil {
			return err
		}
		rt, err := r.uvarints(ColRequestTime)
		if err != nil {
			return err
		}
		var sumBytes, sumRt uint64
		bMin, bMax := ^uint64(0), uint64(0)
		rMin, rMax := ^uint64(0), uint64(0)
		sel.forEach(func(i int) {
			sumBytes += bs[i]
			sumRt += rt[i]
			bMin, bMax = min(bMin, bs[i]), max(bMax, bs[i])
			rMin, rMax = min(rMin, rt[i]), max(rMax, rt[i])
		})
		status, err := blockValueCounts(r, sel, ColStatus)
		if err != nil {
			return err
		}
		methods, err := blockValueCounts(r, sel, ColMethod)
		if err != nil {
			return err
		}
		ipHashes, err := blockDistinctHashes(r, sel, ColIP)
		if err != nil {
			return err
		}
		pathHashes, err := blockDistinctHashes(r, sel, ColPath)
		if err != nil {
			return err
		}
		n := uint64(sel.count())

		mu.Lock()
		defer mu.Unlock()
		out.Count += n
		out.TotalBytes += int64(sumBytes)
		out.TotalReqTime += float64(sumRt) / 1000
		if first {
			out.MinBytes, out.MaxBytes = int64(bMin), int64(bMax)
			minReqMs, maxReqMs = rMin, rMax
			first = false
		} else {
			out.MinBytes, out.MaxBytes = min(out.MinBytes, int64(bMin)), max(out.MaxBytes, int64(bMax))
			minReqMs, maxReqMs = min(minReqMs, rMin), max(maxReqMs, rMax)
		}
		for k, v := range status {
			out.StatusCodeDist[k] += v
		}
		for k, v := range methods {
			out.MethodDist[k] += v
		}
		for _, h := range ipHashes {
			ips[h] = struct{}{}
		}
		for _, h := range pathHashes {
			paths[h] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out.MinReqTime, out.MaxReqTime = float64(minReqMs)/1000, float64(maxReqMs)/1000
	out.UniqueIPs, out.UniquePaths = len(ips), len(paths)
	return out, nil
}

// SortKey is a hit's position in a search's order. Use it as SearchOptions.After
// to fetch the next page.
type SortKey struct {
	Num     int64  // numeric sort columns; request_time in ms
	Str     string // dictionary sort columns
	File    string
	Ordinal int64
}

// SearchOptions controls Search.
type SearchOptions struct {
	SortBy  string // a numeric or dictionary column; default "timestamp"
	Desc    bool
	Offset  int
	Limit   int
	Columns []string // columns to materialize; empty means all
	// After returns only hits ranked strictly after this key, ignoring
	// Offset. It makes deep paging O(page) instead of O(offset).
	After *SortKey
}

// Hit is one row of a search result.
type Hit struct {
	Group   string
	File    string
	Ordinal int64 // position of the row among the rows ingested from File
	Row     *Row
	Key     SortKey
}

// SearchResult is a page of rows plus the exact number of matching rows.
type SearchResult struct {
	Total uint64
	Hits  []*Hit
}

type candidate struct {
	key SortKey
	t   task
	row int
}

// ranker orders candidates: by key in the requested direction, then by file
// and ordinal ascending, so every row has a unique, stable position.
type ranker struct {
	numeric bool
	desc    bool
}

func (rk ranker) before(a, b *SortKey) bool {
	if rk.numeric {
		if a.Num != b.Num {
			return (a.Num > b.Num) == rk.desc
		}
	} else if a.Str != b.Str {
		return (a.Str > b.Str) == rk.desc
	}
	if a.File != b.File {
		return a.File < b.File
	}
	return a.Ordinal < b.Ordinal
}

// worstHeap keeps the best k candidates with the worst one on top.
type worstHeap struct {
	rk    ranker
	items []candidate
}

func (h *worstHeap) Len() int           { return len(h.items) }
func (h *worstHeap) Less(i, j int) bool { return h.rk.before(&h.items[j].key, &h.items[i].key) }
func (h *worstHeap) Swap(i, j int)      { h.items[i], h.items[j] = h.items[j], h.items[i] }
func (h *worstHeap) Push(x any)         { h.items = append(h.items, x.(candidate)) }
func (h *worstHeap) Pop() any {
	n := len(h.items) - 1
	c := h.items[n]
	h.items = h.items[:n]
	return c
}

func (h *worstHeap) offer(c candidate, k int) {
	if len(h.items) < k {
		heap.Push(h, c)
		return
	}
	if h.rk.before(&c.key, &h.items[0].key) {
		h.items[0] = c
		heap.Fix(h, 0)
	}
}

// Search returns the rows ranked offset..offset+limit by the sort column and
// the exact number of matching rows.
func (s *Store) Search(ctx context.Context, q *Query, opts SearchOptions) (*SearchResult, error) {
	sortCol := ColTimestamp
	if opts.SortBy != "" {
		col, ok := ColumnByName(opts.SortBy)
		if !ok || col == ColRaw {
			return nil, fmt.Errorf("logstore: cannot sort by %q", opts.SortBy)
		}
		sortCol = col
	}
	want := allColumns()
	if len(opts.Columns) > 0 {
		var cs columnSet
		for _, name := range opts.Columns {
			if col, ok := ColumnByName(name); ok {
				cs.add(col)
			}
		}
		want = &cs
	}

	offset := max(opts.Offset, 0)
	if opts.After != nil {
		offset = 0
	}
	k := offset + max(opts.Limit, 0)
	rk := ranker{numeric: !sortCol.isDict(), desc: opts.Desc}

	var mu sync.Mutex
	var total uint64
	best := &worstHeap{rk: rk}

	// Visiting blocks in sort order lets a full heap reject whole blocks by
	// their time range once the sort column is the timestamp.
	order := func(tasks []task) {
		if sortCol != ColTimestamp {
			return
		}
		sort.Slice(tasks, func(i, j int) bool {
			if opts.Desc {
				return tasks[i].m.maxTs > tasks[j].m.maxTs
			}
			return tasks[i].m.minTs < tasks[j].m.minTs
		})
	}

	res := &SearchResult{}
	// Materialize the winners before the snapshot releases their files.
	materialize := func() error {
		ranked := best.items
		sort.Slice(ranked, func(i, j int) bool { return rk.before(&ranked[i].key, &ranked[j].key) })
		if offset >= len(ranked) {
			return nil
		}
		ranked = ranked[offset:]
		res.Hits = make([]*Hit, 0, len(ranked))
		readers := map[*blockMeta]*blockReader{}
		for _, c := range ranked {
			r := readers[c.t.m]
			if r == nil {
				r = newBlockReader(s, c.t.seg, c.t.st, c.t.m)
				readers[c.t.m] = r
			}
			row, err := r.row(c.row, want)
			if err != nil {
				return err
			}
			res.Hits = append(res.Hits, &Hit{Group: c.t.seg.Group, File: c.t.seg.Path(), Ordinal: c.key.Ordinal, Row: row, Key: c.key})
		}
		return nil
	}

	err := s.execute(ctx, q, order, func(r *blockReader, sel bitset) error {
		n := uint64(sel.count())
		mu.Lock()
		total += n
		var worst *SortKey
		if k > 0 && best.Len() == k {
			w := best.items[0].key
			worst = &w
		}
		mu.Unlock()
		if k == 0 {
			return nil
		}
		if worst != nil && sortCol == ColTimestamp && blockCannotBeat(r.m, worst, opts.Desc) {
			return nil
		}

		keyOf, err := sortKeyFunc(r, sortCol)
		if err != nil {
			return err
		}
		t := task{seg: r.seg, st: r.st, m: r.m}
		local := &worstHeap{rk: rk}
		sel.forEach(func(i int) {
			key := keyOf(i)
			key.File, key.Ordinal = r.seg.Path(), r.m.firstOrdinal+int64(i)
			if opts.After != nil && !rk.before(opts.After, &key) {
				return
			}
			local.offer(candidate{key: key, t: t, row: i}, k)
		})

		mu.Lock()
		for _, c := range local.items {
			best.offer(c, k)
		}
		mu.Unlock()
		return nil
	}, materialize)
	if err != nil {
		return nil, err
	}
	res.Total = total
	return res, nil
}

// blockCannotBeat reports whether every row of a block ranks after worst for
// a timestamp sort. Ties are left to the row-level comparison.
func blockCannotBeat(m *blockMeta, worst *SortKey, desc bool) bool {
	if desc {
		return m.maxTs < worst.Num
	}
	return m.minTs > worst.Num
}

func sortKeyFunc(r *blockReader, col Column) (func(i int) SortKey, error) {
	switch col {
	case ColTimestamp:
		ts, err := r.timestamps()
		if err != nil {
			return nil, err
		}
		return func(i int) SortKey { return SortKey{Num: ts[i]} }, nil
	case ColStatus:
		st, err := r.statuses()
		if err != nil {
			return nil, err
		}
		return func(i int) SortKey { return SortKey{Num: int64(st[i])} }, nil
	case ColBytesSent, ColRequestTime, ColUpstreamTime:
		v, err := r.uvarints(col)
		if err != nil {
			return nil, err
		}
		return func(i int) SortKey { return SortKey{Num: int64(v[i])} }, nil
	default:
		dict, err := r.dict(col)
		if err != nil {
			return nil, err
		}
		ids, err := r.idColumn(col)
		if err != nil {
			return nil, err
		}
		return func(i int) SortKey { return SortKey{Str: dict[ids[i]]} }, nil
	}
}

// BlockView exposes the decoded columns of one block to Scan. Rows lists the
// matching row indexes; the accessors take those indexes.
type BlockView struct {
	Group string
	File  string
	Rows  []int

	ts     []int64
	status []uint16
	uv     map[Column][]uint64
	dicts  map[Column][]string
	ids    map[Column][]uint32
}

// Timestamp returns the unix time of row i.
func (v *BlockView) Timestamp(i int) int64 { return v.ts[i] }

// Status returns the status code of row i.
func (v *BlockView) Status(i int) int { return int(v.status[i]) }

// BytesSent returns bytes_sent of row i.
func (v *BlockView) BytesSent(i int) int64 { return int64(v.uv[ColBytesSent][i]) }

// RequestTime returns request_time of row i in seconds.
func (v *BlockView) RequestTime(i int) float64 { return float64(v.uv[ColRequestTime][i]) / 1000 }

// String returns a dictionary column of row i.
func (v *BlockView) String(col Column, i int) string { return v.dicts[col][v.ids[col][i]] }

// StringID returns the block-local dictionary id of a column for row i, and
// the dictionary. It lets callers aggregate by id before touching strings.
func (v *BlockView) StringID(col Column, i int) (uint32, []string) {
	return v.ids[col][i], v.dicts[col]
}

// Scan calls fn for every block that has matching rows, with the named
// columns decoded. Blocks are decoded in parallel; fn calls are serialized,
// so fn may update shared state without locking. Block order is unspecified.
func (s *Store) Scan(ctx context.Context, q *Query, columns []string, fn func(v *BlockView) error) error {
	cols := make([]Column, 0, len(columns))
	for _, name := range columns {
		col, ok := ColumnByName(name)
		if !ok || col == ColRaw || col == ColUpstreamTime {
			return fmt.Errorf("logstore: cannot scan %q", name)
		}
		cols = append(cols, col)
	}
	var mu sync.Mutex
	return s.execute(ctx, q, nil, func(r *blockReader, sel bitset) error {
		v := &BlockView{Group: r.seg.Group, File: r.seg.Path(), uv: map[Column][]uint64{}, dicts: map[Column][]string{}, ids: map[Column][]uint32{}}
		v.Rows = make([]int, 0, sel.count())
		sel.forEach(func(i int) { v.Rows = append(v.Rows, i) })
		for _, col := range cols {
			var err error
			switch col {
			case ColTimestamp:
				v.ts, err = r.timestamps()
			case ColStatus:
				v.status, err = r.statuses()
			case ColBytesSent, ColRequestTime:
				v.uv[col], err = r.uvarints(col)
			default:
				if v.dicts[col], err = r.dict(col); err == nil {
					v.ids[col], err = r.idColumn(col)
				}
			}
			if err != nil {
				return err
			}
		}
		mu.Lock()
		defer mu.Unlock()
		return fn(v)
	})
}
