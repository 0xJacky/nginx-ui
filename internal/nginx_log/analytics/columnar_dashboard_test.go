package analytics

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dashboardGroup = "/var/log/nginx/access.log"

// dashboardDocs spreads requests over three days and two files of one group.
func dashboardDocs(n int, start int64) []*indexer.Document {
	r := rand.New(rand.NewSource(5))
	files := []string{dashboardGroup, dashboardGroup + ".1"}
	ordinals := map[string]int{}
	docs := make([]*indexer.Document, n)
	for i := range docs {
		file := files[r.Intn(len(files))]
		doc := &indexer.LogDocument{
			Timestamp:   start + int64(r.Intn(3*86400)),
			IP:          fmt.Sprintf("203.0.113.%d", r.Intn(80)),
			Method:      "GET",
			Path:        []string{"/", "/a", "/b", "/c"}[r.Intn(4)],
			Status:      200,
			BytesSent:   int64(r.Intn(5000)),
			Browser:     []string{"Chrome", "Safari", ""}[r.Intn(3)],
			OS:          []string{"Linux", "iOS"}[r.Intn(2)],
			DeviceType:  []string{"desktop", "mobile"}[r.Intn(2)],
			FilePath:    file,
			MainLogPath: dashboardGroup,
		}
		doc.PathExact = doc.Path
		doc.Raw = doc.IP + " " + doc.Path
		docs[i] = &indexer.Document{ID: file + "-" + strconv.Itoa(ordinals[file]), Fields: doc}
		ordinals[file]++
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Fields.Timestamp < docs[j].Fields.Timestamp })
	return docs
}

// searchOnly hides every method but SearcherInterface.
type searchOnly struct{ searcher.SearcherInterface }

// The dashboard built from Scan and Cardinality matches the one the generic
// path builds by paging hits and counting UV from a facet.
func TestDashboardFromBlockScanMatchesGenericPath(t *testing.T) {
	start := int64(1_760_000_000) - int64(1_760_000_000)%86400
	docs := dashboardDocs(5000, start)

	store, err := logstore.Open(t.TempDir(), logstore.Options{BlockRows: 700})
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	writers := map[string]*logstore.Writer{}
	for _, d := range docs {
		w := writers[d.Fields.FilePath]
		if w == nil {
			w, err = store.Writer(dashboardGroup, d.Fields.FilePath)
			require.NoError(t, err)
			writers[d.Fields.FilePath] = w
		}
		require.NoError(t, w.Append(indexer.LogDocumentToRow(d.Fields), 0))
	}
	for _, w := range writers {
		require.NoError(t, w.Flush())
	}

	config := searcher.DefaultSearcherConfig()
	config.EnableCache = false
	columnar := searcher.NewColumnarSearcher(config, store)
	// The generic path sees only SearcherInterface, so it pages hits with
	// search_after and counts UV from a facet, as any other searcher would.
	genericService := NewService(searchOnly{columnar})
	columnarService := NewService(columnar)

	req := &DashboardQueryRequest{
		LogPath:   dashboardGroup,
		LogPaths:  []string{dashboardGroup},
		StartTime: start + 3600,
		EndTime:   start + 2*86400 + 7200,
	}
	want, err := genericService.GetDashboardAnalytics(context.Background(), req)
	require.NoError(t, err)
	got, err := columnarService.GetDashboardAnalytics(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, want.HourlyStats, got.HourlyStats)
	assert.Equal(t, want.DailyStats, got.DailyStats)
	// The generic path has no exact distinct count, so its UV comes from a
	// facet the dashboard does not request; compare UV with the documents.
	uv := map[string]bool{}
	for _, d := range docs {
		if ts := d.Fields.Timestamp; ts >= req.StartTime && ts < req.EndTime {
			uv[d.Fields.IP] = true
		}
	}
	assert.Equal(t, len(uv), got.Summary.TotalUV)
	wantSummary, gotSummary := want.Summary, got.Summary
	wantSummary.TotalUV, wantSummary.AvgDailyUV = 0, 0
	gotSummary.TotalUV, gotSummary.AvgDailyUV = 0, 0
	assert.Equal(t, wantSummary, gotSummary)
	assert.ElementsMatch(t, want.TopURLs, got.TopURLs)
	assert.ElementsMatch(t, want.Browsers, got.Browsers)
	assert.ElementsMatch(t, want.OperatingSystems, got.OperatingSystems)
	assert.ElementsMatch(t, want.Devices, got.Devices)

	require.Positive(t, got.Summary.TotalPV)
	require.Positive(t, got.Summary.TotalUV)
}
