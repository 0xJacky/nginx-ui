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
	"github.com/blevesearch/bleve/v2"
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

// The dashboard built from the column store's Scan and Cardinality matches
// the one built by paging Bleve hits and counting with the Bleve counter.
func TestDashboardFromColumnStoreMatchesBleve(t *testing.T) {
	start := int64(1_760_000_000) - int64(1_760_000_000)%86400
	docs := dashboardDocs(5000, start)

	index, err := bleve.NewMemOnly(indexer.CreateLogIndexMapping())
	require.NoError(t, err)
	t.Cleanup(func() { _ = index.Close() })
	batch := index.NewBatch()
	for _, d := range docs {
		require.NoError(t, batch.Index(d.ID, d.Fields))
	}
	require.NoError(t, index.Batch(batch))

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
	bleveSearcher := searcher.NewSearcher(config, []bleve.Index{index})
	t.Cleanup(func() { _ = bleveSearcher.Stop() })
	bleveService := NewService(bleveSearcher)
	t.Cleanup(func() { _ = bleveService.Stop() })
	columnarService := NewService(searcher.NewColumnarSearcher(config, store))

	req := &DashboardQueryRequest{
		LogPath:   dashboardGroup,
		LogPaths:  []string{dashboardGroup},
		StartTime: start + 3600,
		EndTime:   start + 2*86400 + 7200,
	}
	want, err := bleveService.GetDashboardAnalytics(context.Background(), req)
	require.NoError(t, err)
	got, err := columnarService.GetDashboardAnalytics(context.Background(), req)
	require.NoError(t, err)

	assert.Equal(t, want.HourlyStats, got.HourlyStats)
	assert.Equal(t, want.DailyStats, got.DailyStats)
	assert.Equal(t, want.Summary, got.Summary)
	assert.ElementsMatch(t, want.TopURLs, got.TopURLs)
	assert.ElementsMatch(t, want.Browsers, got.Browsers)
	assert.ElementsMatch(t, want.OperatingSystems, got.OperatingSystems)
	assert.ElementsMatch(t, want.Devices, got.Devices)

	require.Positive(t, got.Summary.TotalPV)
	require.Positive(t, got.Summary.TotalUV)
}
