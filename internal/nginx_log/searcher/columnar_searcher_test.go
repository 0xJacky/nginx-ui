package searcher

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	parityGroup = "/var/log/nginx/access.log"
	parityBase  = int64(1_760_000_000)
)

// parityDocs builds documents with unique timestamps, so both engines agree
// on the order of every hit regardless of how they break ties.
func parityDocs(n int) []*indexer.Document {
	r := rand.New(rand.NewSource(11))
	files := []string{parityGroup, parityGroup + ".1"}
	paths := []string{"/", "/api/v1/users", "/api/v1/orders", "/static/app.js", "/login"}
	browsers := []string{"Chrome", "Safari", "Firefox", ""}
	regions := []string{"CN", "US", "JP", ""}
	methods := []string{"GET", "POST", "PUT"}
	statuses := []int{200, 200, 301, 404, 500}
	ordinals := map[string]int{}
	docs := make([]*indexer.Document, n)
	for i := range docs {
		file := files[r.Intn(len(files))]
		doc := &indexer.LogDocument{
			Timestamp:   parityBase + int64(i)*7,
			IP:          fmt.Sprintf("192.0.2.%d", r.Intn(60)),
			RegionCode:  regions[r.Intn(len(regions))],
			Method:      methods[r.Intn(len(methods))],
			Path:        paths[r.Intn(len(paths))],
			Protocol:    "HTTP/1.1",
			Status:      statuses[r.Intn(len(statuses))],
			BytesSent:   int64(r.Intn(10_000))*10_000 + int64(i), // unique, so sorts have no ties
			Browser:     browsers[r.Intn(len(browsers))],
			OS:          []string{"Linux", "macOS"}[r.Intn(2)],
			DeviceType:  []string{"desktop", "mobile"}[r.Intn(2)],
			RequestTime: float64(r.Intn(900)) / 1000,
			FilePath:    file,
			MainLogPath: parityGroup,
		}
		doc.PathExact = doc.Path
		doc.Raw = fmt.Sprintf(`%s - - "%s %s HTTP/1.1" %d %d`, doc.IP, doc.Method, doc.Path, doc.Status, doc.BytesSent)
		docs[i] = &indexer.Document{ID: file + "-" + strconv.Itoa(ordinals[file]), Fields: doc}
		ordinals[file]++
	}
	return docs
}

func newColumnarForDocs(t *testing.T, docs []*indexer.Document) *ColumnarSearcher {
	t.Helper()
	store, err := logstore.Open(t.TempDir(), logstore.Options{BlockRows: 256, Parallelism: 4})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	writers := map[string]*logstore.Writer{}
	for _, d := range docs {
		w := writers[d.Fields.FilePath]
		if w == nil {
			w, err = store.Writer(d.Fields.MainLogPath, d.Fields.FilePath)
			require.NoError(t, err)
			writers[d.Fields.FilePath] = w
		}
		require.NoError(t, w.Append(indexer.LogDocumentToRow(d.Fields), 0))
	}
	for _, w := range writers {
		require.NoError(t, w.Flush())
	}
	config := DefaultSearcherConfig()
	config.EnableCache = false
	return NewColumnarSearcher(config, store)
}

func facetMap(f *Facet) map[string]int {
	m := map[string]int{}
	if f == nil {
		return m
	}
	for _, t := range f.Terms {
		m[t.Term] = t.Count
	}
	return m
}

func anyEqual[T comparable](values []T, v T) bool {
	if len(values) == 0 {
		return true
	}
	for _, x := range values {
		if x == v {
			return true
		}
	}
	return false
}

// refMatches evaluates a request's filters on one document the slow way.
func refMatches(req *SearchRequest, d *indexer.LogDocument) bool {
	if len(req.LogPaths) > 0 {
		path := d.FilePath
		if req.UseMainLogPath {
			path = d.MainLogPath
		}
		if !anyEqual(req.LogPaths, path) {
			return false
		}
	}
	if req.StartTime != nil && d.Timestamp < *req.StartTime || req.EndTime != nil && d.Timestamp >= *req.EndTime {
		return false
	}
	if !anyEqual(req.IPAddresses, d.IP) || !anyEqual(req.Methods, d.Method) || !anyEqual(req.StatusCodes, d.Status) ||
		!anyEqual(req.Countries, d.RegionCode) || !anyEqual(req.Browsers, d.Browser) || !anyEqual(req.OSs, d.OS) ||
		!anyEqual(req.Devices, d.DeviceType) {
		return false
	}
	if len(req.Paths) > 0 {
		found := false
		for _, p := range req.Paths {
			found = found || strings.Contains(strings.ToLower(d.Path), strings.ToLower(p))
		}
		if !found {
			return false
		}
	}
	return true
}

// refSearch returns the matching documents in the request's order.
func refSearch(req *SearchRequest, docs []*indexer.Document) []*indexer.Document {
	var out []*indexer.Document
	for _, d := range docs {
		if refMatches(req, d.Fields) {
			out = append(out, d)
		}
	}
	key := func(d *indexer.Document) int64 {
		if req.SortBy == "bytes_sent" {
			return d.Fields.BytesSent
		}
		return d.Fields.Timestamp
	}
	sort.Slice(out, func(i, j int) bool {
		if req.SortOrder == SortOrderAsc {
			return key(out[i]) < key(out[j])
		}
		return key(out[i]) > key(out[j])
	})
	return out
}

func refFacet(field string, docs []*indexer.Document) map[string]int {
	m := map[string]int{}
	for _, d := range docs {
		var v string
		switch field {
		case "browser":
			v = d.Fields.Browser
		case "region_code":
			v = d.Fields.RegionCode
		case "method":
			v = d.Fields.Method
		case "status":
			v = strconv.Itoa(d.Fields.Status)
		}
		m[v]++
	}
	return m
}

// The columnar searcher answers the API's structured requests exactly as a
// brute-force evaluation of the same filters over the documents does.
func TestColumnarSearcherAnswersStructuredRequests(t *testing.T) {
	docs := parityDocs(3000)
	columnar := newColumnarForDocs(t, docs)
	ctx := context.Background()

	start, end := parityBase+2000, parityBase+15000
	requests := map[string]*SearchRequest{
		"all newest first":  {Limit: 40},
		"time range page 3": {StartTime: &start, EndTime: &end, Limit: 25, Offset: 50},
		"status and method": {StatusCodes: []int{404, 500}, Methods: []string{"POST"}, Limit: 30, SortOrder: "asc"},
		"group by main path": {
			LogPaths: []string{parityGroup}, UseMainLogPath: true, Browsers: []string{"Chrome", "Firefox"}, Limit: 20,
		},
		"rotated file only": {LogPaths: []string{parityGroup + ".1"}, Countries: []string{"CN"}, Limit: 20},
		"ip filter":         {IPAddresses: []string{"192.0.2.7", "192.0.2.8"}, Limit: 100},
		"path substring":    {Paths: []string{"/API/v1/users"}, Limit: 15},
		"sort by bytes":     {SortBy: "bytes_sent", SortOrder: "desc", Limit: 30},
		"facets and stats":  {StartTime: &start, IncludeFacets: true, FacetFields: []string{"browser", "region_code", "status", "method"}, FacetSize: 50, IncludeStats: true, Limit: -1},
		"empty result":      {StatusCodes: []int{418}, IncludeFacets: true, FacetFields: []string{"browser"}, Limit: 10},
		"device and os":     {Devices: []string{"mobile"}, OSs: []string{"macOS"}, Limit: 10, Offset: 5},
	}

	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			r := *req
			got, err := columnar.Search(ctx, &r)
			require.NoError(t, err)

			want := refSearch(req, docs)
			require.EqualValues(t, len(want), got.TotalHits)
			limit := req.Limit
			if limit == 0 {
				limit = DefaultSearcherConfig().DefaultLimit
			}
			page := want[min(req.Offset, len(want)):]
			page = page[:max(0, min(limit, len(page)))]
			require.Len(t, got.Hits, len(page))
			for i, d := range page {
				require.Equal(t, d.ID, got.Hits[i].ID, "hit %d", i)
				assert.Equal(t, d.Fields.IP, got.Hits[i].Fields["ip"])
				assert.Equal(t, float64(d.Fields.Timestamp), got.Hits[i].Fields["timestamp"])
				assert.Equal(t, d.Fields.Raw, got.Hits[i].Fields["raw"])
				assert.Equal(t, d.Fields.FilePath, got.Hits[i].Fields["file_path"])
			}

			for _, field := range req.FacetFields {
				assert.Equal(t, refFacet(field, want), facetMap(got.Facets[field]), "facet %s", field)
			}
			if req.IncludeStats {
				require.NotNil(t, got.Stats)
				var bytes, maxBytes int64
				var reqTime float64
				for _, d := range want {
					bytes += d.Fields.BytesSent
					maxBytes = max(maxBytes, d.Fields.BytesSent)
					reqTime += d.Fields.RequestTime
				}
				assert.Equal(t, bytes, got.Stats.TotalBytes)
				assert.Equal(t, maxBytes, got.Stats.MaxBytes)
				assert.InDelta(t, reqTime, got.Stats.TotalReqTime, 1e-6)
			}
		})
	}
}

// Walking a query with search_after cursors visits every match once, in the
// order offset pagination returns them.
func TestColumnarSearcherSearchAfterWalksInOrder(t *testing.T) {
	docs := parityDocs(1200)
	columnar := newColumnarForDocs(t, docs)
	ctx := context.Background()

	var got []string
	var after []string
	for {
		res, err := columnar.Search(ctx, &SearchRequest{
			Methods: []string{"GET"}, SortBy: "timestamp", SortOrder: "asc", Limit: 97, SearchAfter: after,
		})
		require.NoError(t, err)
		if len(res.Hits) == 0 {
			break
		}
		for _, h := range res.Hits {
			got = append(got, h.ID)
		}
		after = res.Hits[len(res.Hits)-1].Sort
	}

	var want []string
	for _, d := range refSearch(&SearchRequest{Methods: []string{"GET"}, SortOrder: SortOrderAsc}, docs) {
		want = append(want, d.ID)
	}
	require.NotEmpty(t, want)
	assert.Equal(t, want, got)
}

// Cardinality applies every filter of the request, which Counter does not.
func TestColumnarCardinalityHonorsFilters(t *testing.T) {
	docs := parityDocs(2000)
	columnar := newColumnarForDocs(t, docs)

	want := map[string]bool{}
	for _, d := range docs {
		if d.Fields.Status == 404 {
			want[d.Fields.IP] = true
		}
	}
	n, err := columnar.Cardinality(context.Background(), "ip", &SearchRequest{StatusCodes: []int{404}})
	require.NoError(t, err)
	assert.EqualValues(t, len(want), n)
}

// Scan replaces the dashboard's search_after loop over stored fields.
func TestColumnarScanBuildsHourlyBuckets(t *testing.T) {
	docs := parityDocs(2000)
	columnar := newColumnarForDocs(t, docs)

	want := map[int64]int{}
	for _, d := range docs {
		want[d.Fields.Timestamp/3600*3600]++
	}
	got := map[int64]int{}
	err := columnar.Scan(context.Background(), &SearchRequest{}, []string{"timestamp"}, func(v *logstore.BlockView) error {
		for _, i := range v.Rows {
			got[v.Timestamp(i)/3600*3600]++
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, want, got)

	hours := make([]int64, 0, len(got))
	for h := range got {
		hours = append(hours, h)
	}
	sort.Slice(hours, func(i, j int) bool { return hours[i] < hours[j] })
	assert.Greater(t, len(hours), 1)
}
