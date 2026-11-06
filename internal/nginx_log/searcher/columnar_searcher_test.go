package searcher

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
	"github.com/blevesearch/bleve/v2"
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

func newParitySearchers(t *testing.T, docs []*indexer.Document) (*Searcher, *ColumnarSearcher) {
	t.Helper()
	index, err := bleve.NewMemOnly(indexer.CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() { _ = index.Close() })
	batch := index.NewBatch()
	for _, d := range docs {
		require.NoError(t, batch.Index(d.ID, d.Fields))
	}
	require.NoError(t, index.Batch(batch))

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
	bleveSearcher := NewSearcher(config, []bleve.Index{index})
	t.Cleanup(func() { _ = bleveSearcher.Stop() })
	return bleveSearcher, NewColumnarSearcher(config, store)
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

// The columnar searcher answers the API's structured requests exactly as the
// Bleve searcher does. Free text is excluded: its semantics change on purpose.
func TestColumnarSearcherMatchesBleve(t *testing.T) {
	docs := parityDocs(3000)
	bleveSearcher, columnar := newParitySearchers(t, docs)
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
		"path phrase":       {Paths: []string{"/api/v1/users"}, Limit: 15},
		"sort by bytes":     {SortBy: "bytes_sent", SortOrder: "desc", Limit: 30},
		"facets and stats":  {StartTime: &start, IncludeFacets: true, FacetFields: []string{"browser", "region_code", "status", "method"}, FacetSize: 50, IncludeStats: true, Limit: -1},
		"selected fields":   {Fields: []string{"timestamp", "ip", "bytes_sent"}, Limit: 10},
		"empty result":      {StatusCodes: []int{418}, IncludeFacets: true, FacetFields: []string{"browser"}, Limit: 10},
		"device and os":     {Devices: []string{"mobile"}, OSs: []string{"macOS"}, Limit: 10, Offset: 5},
	}

	for name, req := range requests {
		t.Run(name, func(t *testing.T) {
			bReq, cReq := *req, *req
			want, err := bleveSearcher.Search(ctx, &bReq)
			require.NoError(t, err)
			got, err := columnar.Search(ctx, &cReq)
			require.NoError(t, err)

			require.Equal(t, want.TotalHits, got.TotalHits)
			require.Len(t, got.Hits, len(want.Hits))
			for i := range want.Hits {
				require.Equal(t, want.Hits[i].ID, got.Hits[i].ID, "hit %d", i)
				for _, field := range []string{"timestamp", "ip", "method", "path", "status", "bytes_sent", "browser", "region_code", "raw", "file_path", "main_log_path"} {
					assert.Equal(t, want.Hits[i].Fields[field], got.Hits[i].Fields[field], "hit %d field %s", i, field)
				}
			}

			for field, f := range want.Facets {
				assert.Equal(t, facetMap(f), facetMap(got.Facets[field]), "facet %s", field)
			}
			if want.Stats != nil {
				require.NotNil(t, got.Stats)
				assert.Equal(t, want.Stats.TotalBytes, got.Stats.TotalBytes)
				assert.Equal(t, want.Stats.MaxBytes, got.Stats.MaxBytes)
				assert.InDelta(t, want.Stats.TotalReqTime, got.Stats.TotalReqTime, 1e-6)
			}
		})
	}
}

// A cursor from the columnar searcher walks the same order Bleve does.
func TestColumnarSearcherSearchAfterMatchesBleve(t *testing.T) {
	docs := parityDocs(1200)
	bleveSearcher, columnar := newParitySearchers(t, docs)
	ctx := context.Background()

	walk := func(s SearcherInterface) []string {
		var ids []string
		var after []string
		for {
			res, err := s.Search(ctx, &SearchRequest{
				Methods: []string{"GET"}, SortBy: "timestamp", SortOrder: "asc", Limit: 97, SearchAfter: after,
			})
			require.NoError(t, err)
			if len(res.Hits) == 0 {
				return ids
			}
			for _, h := range res.Hits {
				ids = append(ids, h.ID)
			}
			after = res.Hits[len(res.Hits)-1].Sort
		}
	}
	want := walk(bleveSearcher)
	require.NotEmpty(t, want)
	assert.Equal(t, want, walk(columnar))
}

// Cardinality applies every filter of the request, which Counter does not.
func TestColumnarCardinalityHonorsFilters(t *testing.T) {
	docs := parityDocs(2000)
	_, columnar := newParitySearchers(t, docs)

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
	_, columnar := newParitySearchers(t, docs)

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
