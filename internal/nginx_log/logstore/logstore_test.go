package logstore

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testRow is a row plus where it was written.
type testRow struct {
	Row
	group, file string
	ordinal     int64
}

var (
	testGroups = []string{"/var/log/nginx/access.log", "/var/log/nginx/api.access.log"}
	testPaths  = []string{"/", "/api/v1/users", "/api/v1/Products", "/static/app.js", "/login", "/search?q=nginx"}
	testUAs    = []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1) Mobile Safari",
		"curl/8.4.0",
		"",
	}
	testBrowsers = []string{"Chrome", "Mobile Safari", "curl", ""}
	testRegions  = []string{"CN", "US", "JP", ""}
	testMethods  = []string{"GET", "GET", "POST", "DELETE"}
	testStatuses = []int{200, 200, 304, 404, 500}
	testReferers = []string{"-", "https://www.google.com/", "https://Example.com/blog"}
)

// genRows builds a deterministic corpus spread over two groups and three
// files, with timestamps that are only roughly ordered, as nginx writes them.
func genRows(n int, seed int64) []*testRow {
	r := rand.New(rand.NewSource(seed))
	files := map[string][]string{
		testGroups[0]: {testGroups[0], testGroups[0] + ".1"},
		testGroups[1]: {testGroups[1]},
	}
	ordinals := map[string]int64{}
	base := int64(1_760_000_000)
	rows := make([]*testRow, n)
	for i := range rows {
		group := testGroups[r.Intn(len(testGroups))]
		file := files[group][r.Intn(len(files[group]))]
		ts := base + int64(i/3) - int64(r.Intn(5))
		ui := r.Intn(len(testUAs))
		path := testPaths[r.Intn(len(testPaths))]
		if r.Intn(5) == 0 {
			path += "/" + strconv.Itoa(r.Intn(1000))
		}
		row := Row{
			Timestamp:      ts,
			IP:             fmt.Sprintf("10.0.%d.%d", r.Intn(4), r.Intn(50)),
			RegionCode:     testRegions[r.Intn(len(testRegions))],
			Province:       []string{"", "Guangdong", "Beijing"}[r.Intn(3)],
			City:           []string{"", "Shenzhen"}[r.Intn(2)],
			C1:             []string{"", "idc"}[r.Intn(2)],
			Method:         testMethods[r.Intn(len(testMethods))],
			Path:           path,
			Protocol:       "HTTP/1.1",
			Status:         testStatuses[r.Intn(len(testStatuses))],
			BytesSent:      int64(r.Intn(100_000)),
			Referer:        testReferers[r.Intn(len(testReferers))],
			UserAgent:      testUAs[ui],
			Browser:        testBrowsers[ui],
			BrowserVersion: strconv.Itoa(100 + r.Intn(3)),
			OS:             []string{"Windows", "iOS", ""}[r.Intn(3)],
			DeviceType:     []string{"desktop", "mobile", "bot"}[r.Intn(3)],
			RequestTime:    float64(r.Intn(3000)) / 1000,
		}
		if r.Intn(2) == 0 {
			up := float64(r.Intn(2000)) / 1000
			row.UpstreamTime = &up
		}
		row.Raw = fmt.Sprintf(`%s - - [%d] "%s %s HTTP/1.1" %d %d "%s" "%s"`,
			row.IP, ts, row.Method, row.Path, row.Status, row.BytesSent, row.Referer, row.UserAgent)
		rows[i] = &testRow{Row: row, group: group, file: file, ordinal: ordinals[file]}
		ordinals[file]++
	}
	return rows
}

func openTestStore(t testing.TB, dir string, blockRows int) *Store {
	t.Helper()
	s, err := Open(dir, Options{BlockRows: blockRows, Parallelism: 4})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// ingest writes rows through one writer per file, flushing every flushEvery
// rows to mimic incremental runs. It returns each file's final source offset.
func ingest(t testing.TB, s *Store, rows []*testRow, flushEvery int) map[string]int64 {
	t.Helper()
	writers := map[string]*Writer{}
	offsets := map[string]int64{}
	for i, r := range rows {
		w := writers[r.file]
		if w == nil {
			var err error
			w, err = s.Writer(r.group, r.file)
			require.NoError(t, err)
			writers[r.file] = w
		}
		offsets[r.file] += int64(len(r.Raw) + 1)
		require.NoError(t, w.Append(&r.Row, offsets[r.file]))
		if flushEvery > 0 && (i+1)%flushEvery == 0 {
			for _, w := range writers {
				require.NoError(t, w.Flush())
			}
		}
	}
	for _, w := range writers {
		require.NoError(t, w.Flush())
	}
	return offsets
}

// --- reference evaluation ---

func anyOf[T comparable](vals []T, v T) bool {
	if len(vals) == 0 {
		return true
	}
	for _, x := range vals {
		if x == v {
			return true
		}
	}
	return false
}

func anyContains(needles []string, v string) bool {
	if len(needles) == 0 {
		return true
	}
	for _, n := range needles {
		if strings.Contains(strings.ToLower(v), strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func refMatch(q *Query, r *testRow) bool {
	if !anyOf(q.Groups, r.group) || !anyOf(q.Files, r.file) {
		return false
	}
	if (q.From != 0 && r.Timestamp < q.From) || (q.To != 0 && r.Timestamp >= q.To) {
		return false
	}
	if !anyOf(q.IPs, r.IP) || !anyOf(q.Methods, r.Method) || !anyOf(q.RegionCodes, r.RegionCode) ||
		!anyOf(q.Provinces, r.Province) || !anyOf(q.Cities, r.City) || !anyOf(q.Browsers, r.Browser) ||
		!anyOf(q.OSs, r.OS) || !anyOf(q.Devices, r.DeviceType) || !anyOf(q.Statuses, r.Status) {
		return false
	}
	if !anyContains(q.PathContains, r.Path) || !anyContains(q.UserAgentContains, r.UserAgent) || !anyContains(q.RefererContains, r.Referer) {
		return false
	}
	if q.MinBytes != nil && r.BytesSent < *q.MinBytes || q.MaxBytes != nil && r.BytesSent > *q.MaxBytes {
		return false
	}
	ms := secondsToMs(r.RequestTime)
	if q.MinRequestTime != nil && ms < secondsToMs(*q.MinRequestTime) || q.MaxRequestTime != nil && ms > secondsToMs(*q.MaxRequestTime) {
		return false
	}
	for _, term := range splitTerms(q.Text) {
		if !strings.Contains(strings.ToLower(r.Raw), strings.ToLower(term)) {
			return false
		}
	}
	return true
}

func refSelect(q *Query, rows []*testRow) []*testRow {
	var out []*testRow
	for _, r := range rows {
		if refMatch(q, r) {
			out = append(out, r)
		}
	}
	return out
}

func randomQuery(r *rand.Rand, rows []*testRow) *Query {
	q := &Query{}
	pick := func(p float64) bool { return r.Float64() < p }
	lo, hi := rows[0].Timestamp, rows[len(rows)-1].Timestamp
	if pick(0.5) {
		q.From = lo + r.Int63n(hi-lo+1)
	}
	if pick(0.5) {
		q.To = lo + r.Int63n(hi-lo+1)
	}
	if pick(0.3) {
		q.Groups = []string{testGroups[r.Intn(len(testGroups))]}
	}
	if pick(0.15) {
		q.Files = []string{testGroups[0] + ".1"}
	}
	if pick(0.3) {
		q.Statuses = []int{testStatuses[r.Intn(len(testStatuses))], testStatuses[r.Intn(len(testStatuses))]}
	}
	if pick(0.3) {
		q.Methods = []string{testMethods[r.Intn(len(testMethods))]}
	}
	if pick(0.2) {
		q.RegionCodes = []string{testRegions[r.Intn(3)]}
	}
	if pick(0.2) {
		q.Browsers = []string{testBrowsers[r.Intn(3)]}
	}
	if pick(0.1) {
		q.IPs = []string{rows[r.Intn(len(rows))].IP}
	}
	if pick(0.3) {
		q.PathContains = []string{[]string{"api", "/API/V1/products", "static", "nothing-matches"}[r.Intn(4)]}
	}
	if pick(0.2) {
		q.UserAgentContains = []string{[]string{"mozilla", "CURL", "iphone"}[r.Intn(3)]}
	}
	if pick(0.15) {
		q.RefererContains = []string{"example.COM"}
	}
	if pick(0.2) {
		v := int64(r.Intn(100_000))
		q.MinBytes = &v
	}
	if pick(0.2) {
		v := float64(r.Intn(3000)) / 1000
		q.MaxRequestTime = &v
	}
	if pick(0.3) {
		q.Text = []string{"post", `"HTTP/1.1" 404`, "google chrome", "10.0.1.", "zzz"}[r.Intn(5)]
	}
	return q
}

func TestQueriesMatchReference(t *testing.T) {
	rows := genRows(12_000, 1)
	s := openTestStore(t, t.TempDir(), 1000)
	ingest(t, s, rows, 2500)
	ctx := context.Background()

	r := rand.New(rand.NewSource(7))
	for i := 0; i < 150; i++ {
		q := randomQuery(r, rows)
		want := refSelect(q, rows)
		name := fmt.Sprintf("%d:%+v", i, *q)

		count, err := s.Count(ctx, q)
		require.NoError(t, err, name)
		require.EqualValues(t, len(want), count, name)

		// Facets: status (numeric) and two dictionary columns.
		facets, total, err := s.Facets(ctx, q, []string{"status", "browser", "path_exact"}, 3)
		require.NoError(t, err, name)
		require.EqualValues(t, len(want), total, name)
		checkFacet(t, facets["status"], want, func(r *testRow) string { return strconv.Itoa(r.Status) }, 3, name)
		checkFacet(t, facets["browser"], want, func(r *testRow) string { return r.Browser }, 3, name)
		checkFacet(t, facets["path_exact"], want, func(r *testRow) string { return r.Path }, 3, name)

		distinct, err := s.Distinct(ctx, q, "ip")
		require.NoError(t, err, name)
		ips := map[string]bool{}
		for _, r := range want {
			ips[r.IP] = true
		}
		require.EqualValues(t, len(ips), distinct, name)

		stats, err := s.Stats(ctx, q)
		require.NoError(t, err, name)
		var bytes int64
		for _, r := range want {
			bytes += r.BytesSent
		}
		require.EqualValues(t, len(want), stats.Count, name)
		require.Equal(t, bytes, stats.TotalBytes, name)
		require.Equal(t, len(ips), stats.UniqueIPs, name)

		checkSearch(t, s, q, want, SearchOptions{Desc: true, Offset: r.Intn(40), Limit: 25}, name)
		checkSearch(t, s, q, want, SearchOptions{SortBy: "bytes_sent", Offset: r.Intn(10), Limit: 10}, name)
		checkSearch(t, s, q, want, SearchOptions{SortBy: "browser", Desc: true, Limit: 10}, name)
	}
}

func checkFacet(t *testing.T, got *FacetResult, rows []*testRow, value func(*testRow) string, size int, name string) {
	t.Helper()
	counts := map[string]int{}
	missing := 0
	for _, r := range rows {
		if v := value(r); v == "" {
			missing++
		} else {
			counts[v]++
		}
	}
	require.Equal(t, len(counts), got.Distinct, name)
	require.Equal(t, missing, got.Missing, name)
	require.LessOrEqual(t, len(got.Terms), size, name)
	sum := 0
	for i, term := range got.Terms {
		require.Equal(t, counts[term.Term], term.Count, name)
		if i > 0 {
			require.GreaterOrEqual(t, got.Terms[i-1].Count, term.Count, name)
		}
		sum += term.Count
	}
	require.Equal(t, len(rows)-missing-sum, got.Other, name)
}

func refSort(rows []*testRow, opts SearchOptions) []*testRow {
	key := func(r *testRow) SortKey {
		k := SortKey{File: r.file, Ordinal: r.ordinal}
		switch opts.SortBy {
		case "", "timestamp":
			k.Num = r.Timestamp
		case "bytes_sent":
			k.Num = r.BytesSent
		case "status":
			k.Num = int64(r.Status)
		case "browser":
			k.Str = r.Browser
		}
		return k
	}
	rk := ranker{numeric: opts.SortBy != "browser", desc: opts.Desc}
	out := append([]*testRow(nil), rows...)
	sort.Slice(out, func(i, j int) bool {
		a, b := key(out[i]), key(out[j])
		return rk.before(&a, &b)
	})
	return out
}

func checkSearch(t *testing.T, s *Store, q *Query, want []*testRow, opts SearchOptions, name string) {
	t.Helper()
	res, err := s.Search(context.Background(), q, opts)
	require.NoError(t, err, name)
	require.EqualValues(t, len(want), res.Total, name)

	sorted := refSort(want, opts)
	lo, hi := min(opts.Offset, len(sorted)), min(opts.Offset+opts.Limit, len(sorted))
	page := sorted[lo:hi]
	require.Len(t, res.Hits, len(page), name)
	for i, h := range res.Hits {
		require.Equal(t, page[i].file, h.File, name)
		require.Equal(t, page[i].ordinal, h.Ordinal, name)
		require.Equal(t, page[i].Raw, h.Row.Raw, name)
	}
}

func TestRowsRoundTripAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	rows := genRows(3000, 2)
	s, err := Open(dir, Options{BlockRows: 700})
	require.NoError(t, err)
	offsets := ingest(t, s, rows, 0)
	require.NoError(t, s.Close())

	s = openTestStore(t, dir, 700)
	for file, off := range offsets {
		require.NotNil(t, s.Segment(file))
		assert.Equal(t, off, s.Segment(file).SourceOffset(), file)
	}

	res, err := s.Search(context.Background(), &Query{}, SearchOptions{Limit: len(rows)})
	require.NoError(t, err)
	require.Len(t, res.Hits, len(rows))
	byKey := map[string]*testRow{}
	for _, r := range rows {
		byKey[r.file+"#"+strconv.FormatInt(r.ordinal, 10)] = r
	}
	for _, h := range res.Hits {
		want := byKey[h.File+"#"+strconv.FormatInt(h.Ordinal, 10)]
		require.NotNil(t, want)
		require.Equal(t, want.group, h.Group)
		require.Equal(t, want.Row, *h.Row)
	}
}

func TestSearchAfterWalksEveryRowOnce(t *testing.T) {
	rows := genRows(5000, 3)
	s := openTestStore(t, t.TempDir(), 512)
	ingest(t, s, rows, 900)
	q := &Query{Methods: []string{"GET"}}
	want := refSelect(q, rows)

	for _, opts := range []SearchOptions{{Desc: true}, {SortBy: "browser"}, {SortBy: "status", Desc: true}} {
		var after *SortKey
		seen := 0
		for {
			opts.After, opts.Limit = after, 333
			res, err := s.Search(context.Background(), q, opts)
			require.NoError(t, err)
			require.EqualValues(t, len(want), res.Total)
			if len(res.Hits) == 0 {
				break
			}
			sorted := refSort(want, opts)
			for i, h := range res.Hits {
				require.Equal(t, sorted[seen+i].file, h.File)
				require.Equal(t, sorted[seen+i].ordinal, h.Ordinal)
			}
			seen += len(res.Hits)
			last := res.Hits[len(res.Hits)-1].Key
			after = &last
		}
		require.Equal(t, len(want), seen, opts.SortBy)
	}
}

func TestTornTailIsDroppedOnOpen(t *testing.T) {
	dir := t.TempDir()
	rows := genRows(2000, 4)
	for _, r := range rows {
		r.group, r.file = testGroups[0], testGroups[0]
	}
	s, err := Open(dir, Options{BlockRows: 500})
	require.NoError(t, err)
	ingest(t, s, rows[:1500], 0)
	seg := s.Segment(testGroups[0])
	before := seg.Info()
	path := seg.state.Load().h.path
	require.NoError(t, s.Close())

	// A crash in the middle of an append leaves part of a block behind.
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	partial := newBlockBuilder()
	for _, r := range rows[1500:] {
		partial.add(&r.Row, 1)
	}
	data := partial.encode(mustEncoder(t))
	_, err = f.WriteAt(data[:len(data)/2], before.Bytes)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	s = openTestStore(t, dir, 500)
	after := s.Segment(testGroups[0]).Info()
	assert.Equal(t, before.Rows, after.Rows)
	assert.Equal(t, before.SourceOffset, after.SourceOffset)
	fi, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.Bytes, fi.Size(), "the torn tail is truncated")

	// The store keeps appending after the recovered tail.
	ingest(t, s, rows[1500:], 0)
	n, err := s.Count(context.Background(), &Query{})
	require.NoError(t, err)
	assert.EqualValues(t, len(rows), n)
}

func TestCorruptLastBlockChecksumIsDropped(t *testing.T) {
	dir := t.TempDir()
	rows := genRows(1000, 5)
	for _, r := range rows {
		r.group, r.file = testGroups[0], testGroups[0]
	}
	s, err := Open(dir, Options{BlockRows: 400})
	require.NoError(t, err)
	ingest(t, s, rows, 0)
	st := s.Segment(testGroups[0]).state.Load()
	last := st.blocks[len(st.blocks)-1]
	path := st.h.path
	require.NoError(t, s.Close())

	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	_, err = f.WriteAt([]byte{0xff}, last.offset+last.length-6)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	s = openTestStore(t, dir, 400)
	assert.EqualValues(t, int64(len(rows)-last.rows), s.Segment(testGroups[0]).Info().Rows)
}

func mustEncoder(t testing.TB) *zstd.Encoder {
	t.Helper()
	enc, err := zstd.NewWriter(nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = enc.Close() })
	return enc
}

func TestCompactMergesSmallBlocksWhileQueriesRun(t *testing.T) {
	dir := t.TempDir()
	rows := genRows(6000, 6)
	for _, r := range rows {
		r.group, r.file = testGroups[0], testGroups[0]
	}
	for i, r := range rows {
		r.ordinal = int64(i)
	}
	s := openTestStore(t, dir, 2000)
	offsets := ingest(t, s, rows, 150) // 40 small blocks
	seg := s.Segment(testGroups[0])
	require.Equal(t, 40, seg.Info().Blocks)

	q := &Query{Statuses: []int{404}}
	want := refSelect(q, rows)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				n, err := s.Count(context.Background(), q)
				if !assert.NoError(t, err) || !assert.EqualValues(t, len(want), n) {
					return
				}
			}
		}()
	}
	changed, err := s.Compact(testGroups[0])
	cancel()
	wg.Wait()
	require.NoError(t, err)
	require.True(t, changed)

	info := seg.Info()
	assert.Equal(t, 4, info.Blocks, "40 blocks of 150 rows merge into runs of at most 2000")
	assert.Equal(t, offsets[testGroups[0]], info.SourceOffset)
	checkSearch(t, s, q, want, SearchOptions{Desc: true, Limit: 50}, "after compaction")

	entries, err := os.ReadDir(filepath.Dir(seg.state.Load().h.path))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "the old generation is removed")

	changed, err = s.Compact(testGroups[0])
	require.NoError(t, err)
	assert.False(t, changed, "nothing left to merge")

	require.NoError(t, s.Close())
	s = openTestStore(t, dir, 2000)
	n, err := s.Count(context.Background(), q)
	require.NoError(t, err)
	assert.EqualValues(t, len(want), n)
}

func TestDeleteFileDuringQuery(t *testing.T) {
	rows := genRows(4000, 8)
	s := openTestStore(t, t.TempDir(), 300)
	ingest(t, s, rows, 0)
	file := testGroups[0] + ".1"
	path := s.Segment(file).state.Load().h.path

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for ctx.Err() == nil {
			_, err := s.Search(context.Background(), &Query{Text: "get"}, SearchOptions{Desc: true, Limit: 100})
			if !assert.NoError(t, err) {
				return
			}
		}
	}()
	s.DeleteFile(file)
	cancel()
	wg.Wait()

	assert.Nil(t, s.Segment(file))
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "the file is removed once readers finish")
	n, err := s.Count(context.Background(), &Query{Files: []string{file}})
	require.NoError(t, err)
	assert.Zero(t, n)

	s.DeleteGroup(testGroups[1])
	assert.Empty(t, s.Segments(testGroups[1]))
}

func TestScanSeesEverySelectedRowOnce(t *testing.T) {
	rows := genRows(5000, 9)
	s := openTestStore(t, t.TempDir(), 600)
	ingest(t, s, rows, 0)
	q := &Query{From: rows[1000].Timestamp, To: rows[4000].Timestamp}
	want := refSelect(q, rows)

	perHour := map[int64]int{}
	var bytes int64
	uv := map[string]bool{}
	err := s.Scan(context.Background(), q, []string{"timestamp", "ip", "bytes_sent"}, func(v *BlockView) error {
		for _, i := range v.Rows {
			perHour[v.Timestamp(i)/3600]++
			bytes += v.BytesSent(i)
			uv[v.String(ColIP, i)] = true
		}
		return nil
	})
	require.NoError(t, err)

	wantHour := map[int64]int{}
	var wantBytes int64
	wantUV := map[string]bool{}
	for _, r := range want {
		wantHour[r.Timestamp/3600]++
		wantBytes += r.BytesSent
		wantUV[r.IP] = true
	}
	assert.Equal(t, wantHour, perHour)
	assert.Equal(t, wantBytes, bytes)
	assert.Equal(t, len(wantUV), len(uv))
}

func TestSplitTerms(t *testing.T) {
	assert.Equal(t, []string{"GET", "HTTP/1.1 404", "x"}, splitTerms(`GET "HTTP/1.1 404" x`))
	assert.Equal(t, []string{"a b", "c"}, splitTerms(` "a b"  c `))
	assert.Equal(t, []string{"open"}, splitTerms(`"open`))
	assert.Empty(t, splitTerms(`  ""  `))
}

func TestWriterAbortRemovesOnlyItsOwnBlocks(t *testing.T) {
	rows := genRows(3000, 10)
	for _, r := range rows {
		r.group, r.file = testGroups[0], testGroups[0]
	}
	dir := t.TempDir()
	s := openTestStore(t, dir, 500)
	ingest(t, s, rows[:1000], 0)
	seg := s.Segment(testGroups[0])
	before := seg.Info()

	// A second read flushes two full blocks, buffers more, then fails.
	w, err := s.Writer(testGroups[0], testGroups[0])
	require.NoError(t, err)
	for _, r := range rows[1000:2201] {
		require.NoError(t, w.Append(&r.Row, 0))
	}
	require.Equal(t, before.Rows+1000, seg.Info().Rows, "two sealed blocks are visible")
	require.NoError(t, w.Abort())

	after := seg.Info()
	assert.Equal(t, before.Rows, after.Rows)
	assert.Equal(t, before.Blocks, after.Blocks)
	assert.Equal(t, before.Bytes, after.Bytes)

	// The retry appends cleanly, and a reopen sees exactly what is published.
	w, err = s.Writer(testGroups[0], testGroups[0])
	require.NoError(t, err)
	for _, r := range rows[1000:] {
		require.NoError(t, w.Append(&r.Row, 0))
	}
	w.SetSourceEnd(999_999)
	require.NoError(t, w.Flush())
	require.NoError(t, s.Close())

	s = openTestStore(t, dir, 500)
	info := s.Segment(testGroups[0]).Info()
	assert.EqualValues(t, len(rows), info.Rows)
	assert.EqualValues(t, 999_999, info.SourceOffset)
}

func TestWriterAbortAfterCompactionRefuses(t *testing.T) {
	rows := genRows(600, 11)
	for _, r := range rows {
		r.group, r.file = testGroups[0], testGroups[0]
	}
	s := openTestStore(t, t.TempDir(), 1000)
	w, err := s.Writer(testGroups[0], testGroups[0])
	require.NoError(t, err)
	for i, r := range rows {
		require.NoError(t, w.Append(&r.Row, 0))
		if i%100 == 99 {
			require.NoError(t, w.Flush())
		}
	}
	changed, err := s.Compact(testGroups[0])
	require.NoError(t, err)
	require.True(t, changed)
	assert.Error(t, w.Abort(), "merged blocks can no longer be told apart")
	assert.EqualValues(t, len(rows), s.Segment(testGroups[0]).Info().Rows)
}

func TestDeleteAllKeepsStoreUsable(t *testing.T) {
	rows := genRows(2000, 12)
	dir := t.TempDir()
	s := openTestStore(t, dir, 400)
	ingest(t, s, rows, 0)
	s.DeleteAll()
	assert.Empty(t, s.Segments(""))
	n, err := s.Count(context.Background(), &Query{})
	require.NoError(t, err)
	assert.Zero(t, n)

	ingest(t, s, rows[:100], 0)
	n, err = s.Count(context.Background(), &Query{})
	require.NoError(t, err)
	assert.EqualValues(t, 100, n)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), len(testGroups), "emptied group directories are removed")
}
