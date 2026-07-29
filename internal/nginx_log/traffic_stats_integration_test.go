package nginx_log

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/analytics"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/searcher"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type knownTrafficEntry struct {
	offset    time.Duration
	bytesSent int64
}

func writeKnownTrafficLog(t *testing.T, logPath string, baseTime time.Time, entries []knownTrafficEntry) {
	t.Helper()

	var contents strings.Builder
	for i, entry := range entries {
		timestamp := baseTime.Add(entry.offset)
		_, err := fmt.Fprintf(&contents,
			`192.0.2.%d - - [%s] "GET /traffic/%d HTTP/1.1" 200 %d "-" "integration-test"`+"\n",
			i+1,
			timestamp.Format("02/Jan/2006:15:04:05 -0700"),
			i,
			entry.bytesSent,
		)
		require.NoError(t, err)
	}

	require.NoError(t, os.WriteFile(logPath, []byte(contents.String()), 0o600))
}

func useTrafficTestMetadataDB(t *testing.T, dbPath string) func() {
	t.Helper()

	originalModelDB := model.UseDB()
	originalQueryDB := query.Q.UnderlyingDB()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.NginxLogIndex{}))
	model.Use(db)
	query.SetDefault(db)

	return func() {
		if originalModelDB != nil {
			model.Use(originalModelDB)
		}
		if originalQueryDB != nil {
			query.SetDefault(originalQueryDB)
		}
	}
}

func TestTrafficStatsEndToEndWithRealIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	suite := NewIntegrationTestSuite(t)
	defer suite.cleanup()
	restoreMetadataDB := useTrafficTestMetadataDB(t, filepath.Join(suite.tempDir, "metadata.db"))
	defer restoreMetadataDB()

	baseTime := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)
	insideRange := []knownTrafficEntry{
		{offset: 5 * time.Second, bytesSent: 100},
		{offset: 15 * time.Second, bytesSent: 200},
		{offset: 30 * time.Second, bytesSent: 300},
		{offset: 55 * time.Second, bytesSent: 400},
		{offset: 65 * time.Second, bytesSent: 500},
		{offset: 80 * time.Second, bytesSent: 600},
		{offset: 110 * time.Second, bytesSent: 700},
		{offset: 125 * time.Second, bytesSent: 800},
		{offset: 150 * time.Second, bytesSent: 900},
	}
	entries := append([]knownTrafficEntry(nil), insideRange...)
	entries = append(entries, knownTrafficEntry{
		offset:    3 * time.Minute,
		bytesSent: 9999,
	})

	logPath := filepath.Join(suite.tempDir, "logs", "access_traffic.log")
	writeKnownTrafficLog(t, logPath, baseTime, entries)
	suite.logFiles = []string{filepath.Base(logPath)}
	suite.logFilePaths = []string{logPath}

	suite.InitializeServices(t)
	suite.PerformGlobalIndexRebuild(t)

	startTime := baseTime.Unix()
	endTime := baseTime.Add(3 * time.Minute).Unix()
	expectedTraffic := int64(4500)

	searchPage := func(limit, offset int) *searcher.SearchResult {
		t.Helper()
		result, err := suite.searcher.Search(suite.ctx, &searcher.SearchRequest{
			LogPaths:       []string{logPath},
			UseMainLogPath: true,
			StartTime:      &startTime,
			EndTime:        &endTime,
			Limit:          limit,
			Offset:         offset,
			SortBy:         "timestamp",
			SortOrder:      "asc",
			IncludeStats:   true,
			UseCache:       true,
		})
		require.NoError(t, err)
		require.Equal(t, uint64(len(insideRange)), result.TotalHits)
		require.NotNil(t, result.Stats)
		assert.Equal(t, expectedTraffic, result.Stats.TotalBytes)
		assert.False(t, result.Stats.Approximate)
		return result
	}

	pageOne := searchPage(3, 0)
	pageThree := searchPage(3, 6)
	differentPageSize := searchPage(5, 0)
	require.Len(t, pageOne.Hits, 3)
	require.Len(t, pageThree.Hits, 3)
	require.Len(t, differentPageSize.Hits, 5)
	assert.Equal(t, pageOne.Stats.TotalBytes, pageThree.Stats.TotalBytes)
	assert.Equal(t, pageOne.Stats.TotalBytes, differentPageSize.Stats.TotalBytes)

	dashboard, err := suite.analytics.GetDashboardAnalytics(suite.ctx, &analytics.DashboardQueryRequest{
		LogPath:   logPath,
		LogPaths:  []string{logPath},
		StartTime: startTime,
		EndTime:   endTime,
	})
	require.NoError(t, err)
	require.NotNil(t, dashboard)
	assert.Equal(t, len(insideRange), dashboard.Summary.TotalPV)
	assert.Equal(t, expectedTraffic, dashboard.Summary.TotalTraffic)
	assert.InDelta(t, float64(len(insideRange))/float64(endTime-startTime), dashboard.Summary.AvgQPS, 1e-12)
	assert.InDelta(t, 4.0/60.0, dashboard.Summary.PeakQPS, 1e-12)
}
