package nginx_log_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nginxlogapi "github.com/0xJacky/Nginx-UI/api/nginx_log"
	nginxlog "github.com/0xJacky/Nginx-UI/internal/nginx_log"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type searchHTTPResponse struct {
	Entries []map[string]interface{} `json:"entries"`
	Total   uint64                   `json:"total"`
	Summary map[string]interface{}   `json:"summary"`
}

func TestNginxLogHTTPHandlersWithRealRouterAndIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	nginxlog.StopServices()
	originalNginxSettings := *settings.NginxSettings
	originalNginxLogSettings := *settings.NginxLogSettings
	tempDir := t.TempDir()
	originalModelDB := model.UseDB()
	originalQueryDB := query.Q.UnderlyingDB()
	metadataDB, err := gorm.Open(sqlite.Open(filepath.Join(tempDir, "metadata.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, metadataDB.AutoMigrate(&model.NginxLogIndex{}))
	model.Use(metadataDB)
	query.SetDefault(metadataDB)
	t.Cleanup(func() {
		nginxlog.StopServices()
		*settings.NginxSettings = originalNginxSettings
		*settings.NginxLogSettings = originalNginxLogSettings
		if originalModelDB != nil {
			model.Use(originalModelDB)
		}
		if originalQueryDB != nil {
			query.SetDefault(originalQueryDB)
		}
	})

	logsDir := filepath.Join(tempDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	logPath := filepath.Join(logsDir, "access.log")
	baseTime := time.Date(2026, time.January, 16, 8, 0, 0, 0, time.UTC)
	bytesSent := []int64{111, 222, 333, 444, 555, 666}

	var contents strings.Builder
	for i, size := range bytesSent {
		timestamp := baseTime.Add(time.Duration(i*15+5) * time.Second)
		_, err = fmt.Fprintf(&contents,
			`198.51.100.%d - - [%s] "GET /handler/%d HTTP/1.1" 200 %d "-" "handler-integration-test"`+"\n",
			i+1,
			timestamp.Format("02/Jan/2006:15:04:05 -0700"),
			i,
			size,
		)
		require.NoError(t, err)
	}
	require.NoError(t, os.WriteFile(logPath, []byte(contents.String()), 0o600))

	settings.NginxSettings.AccessLogPath = logPath
	settings.NginxSettings.LogDirWhiteList = []string{logsDir}
	settings.NginxLogSettings.IndexingEnabled = true
	settings.NginxLogSettings.IndexPath = filepath.Join(tempDir, "index")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	nginxlog.InitializeServices(ctx)

	indexer := nginxlog.GetIndexer()
	require.NotNil(t, indexer)
	indexed, _, _, err := indexer.IndexSingleFile(logPath)
	require.NoError(t, err)
	require.Equal(t, uint64(len(bytesSent)), indexed)
	require.NoError(t, indexer.FlushAll())

	searchService := nginxlog.GetSearcher()
	require.NotNil(t, searchService)
	require.NoError(t, searchService.SwapShards(indexer.GetAllShards()))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	nginxlogapi.InitRouter(router.Group("/"))

	defaultDirRecorder := httptest.NewRecorder()
	defaultDirRequest := httptest.NewRequest(http.MethodGet, "/nginx_log/default_log_dir", nil)
	router.ServeHTTP(defaultDirRecorder, defaultDirRequest)
	require.Equal(t, http.StatusOK, defaultDirRecorder.Code)

	var defaultDirBody map[string]interface{}
	require.NoError(t, json.Unmarshal(defaultDirRecorder.Body.Bytes(), &defaultDirBody))
	accessLogDir, ok := defaultDirBody["access_log_dir"].(string)
	require.True(t, ok)
	require.NotEmpty(t, accessLogDir)
	assert.Equal(t, logsDir, accessLogDir)
	require.Len(t, defaultDirBody, 1)

	expectedTraffic := int64(2331)
	startTime := baseTime.Unix()
	endTime := baseTime.Add(2 * time.Minute).Unix()
	search := func(limit, offset int) searchHTTPResponse {
		t.Helper()
		payload, err := json.Marshal(map[string]interface{}{
			"log_path":   logPath,
			"start_time": startTime,
			"end_time":   endTime,
			"limit":      limit,
			"offset":     offset,
		})
		require.NoError(t, err)

		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/nginx_log/search", bytes.NewReader(payload))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

		var response searchHTTPResponse
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
		require.Equal(t, uint64(len(bytesSent)), response.Total)
		totalTraffic, ok := response.Summary["total_traffic"]
		require.True(t, ok)
		assert.Equal(t, float64(expectedTraffic), totalTraffic)
		trafficApproximate, ok := response.Summary["traffic_approximate"]
		require.True(t, ok)
		assert.Equal(t, false, trafficApproximate)
		return response
	}

	smallPage := search(2, 0)
	largeLaterPage := search(4, 2)
	require.Len(t, smallPage.Entries, 2)
	require.Len(t, largeLaterPage.Entries, 4)
	assert.Equal(t, smallPage.Summary["total_traffic"], largeLaterPage.Summary["total_traffic"])
}
