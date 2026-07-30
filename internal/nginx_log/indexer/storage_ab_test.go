package indexer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/stretchr/testify/require"
)

var errABDocumentLimit = errors.New("A/B document limit reached")

// TestLogIndexMappingStorageAB is an opt-in real-data comparison. It is kept
// out of normal CI because its result is a benchmark artifact rather than a
// platform-independent correctness assertion.
func TestLogIndexMappingStorageAB(t *testing.T) {
	if os.Getenv("NGINX_LOG_INDEX_AB") != "1" {
		t.Skip("set NGINX_LOG_INDEX_AB=1 to run the storage A/B test")
	}

	sourceDir := os.Getenv("NGINX_LOG_INDEX_AB_SOURCE")
	require.NotEmpty(t, sourceDir, "NGINX_LOG_INDEX_AB_SOURCE is required")
	documentLimit := 50_000
	if value := os.Getenv("NGINX_LOG_INDEX_AB_DOCUMENTS"); value != "" {
		parsed, err := strconv.Atoi(value)
		require.NoError(t, err)
		documentLimit = parsed
	}

	documents := loadABDocuments(t, sourceDir, documentLimit)
	require.Greater(t, len(documents), 1000)

	legacySize, legacyDuration := buildABIndex(t, "legacy", legacyLogIndexMapping(), documents)
	optimizedSize, optimizedDuration := buildABIndex(t, "optimized", CreateLogIndexMapping(), documents)
	require.Less(t, optimizedSize, legacySize)

	savedPercent := float64(legacySize-optimizedSize) / float64(legacySize) * 100
	t.Logf(
		"AB_RESULT documents=%d legacy_bytes=%d optimized_bytes=%d saved_percent=%.2f legacy_duration=%s optimized_duration=%s",
		len(documents),
		legacySize,
		optimizedSize,
		savedPercent,
		legacyDuration,
		optimizedDuration,
	)
}

func loadABDocuments(t *testing.T, sourceDir string, limit int) []*LogDocument {
	t.Helper()
	InitLogParser()

	entries, err := os.ReadDir(sourceDir)
	require.NoError(t, err)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	documents := make([]*LogDocument, 0, limit)
	for _, entry := range entries {
		if entry.IsDir() || !isABAccessLog(entry.Name()) || len(documents) >= limit {
			continue
		}
		path := filepath.Join(sourceDir, entry.Name())
		file, err := os.Open(path)
		require.NoError(t, err)
		_, _, parseErr := ParseLogStreamBatches(context.Background(), file, path, func(batch []*LogDocument) error {
			remaining := limit - len(documents)
			if remaining <= 0 {
				return errABDocumentLimit
			}
			if len(batch) > remaining {
				batch = batch[:remaining]
			}
			documents = append(documents, batch...)
			if len(documents) >= limit {
				return errABDocumentLimit
			}
			return nil
		})
		require.NoError(t, file.Close())
		if parseErr != nil && !errors.Is(parseErr, errABDocumentLimit) {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
	}
	return documents
}

func isABAccessLog(name string) bool {
	return strings.HasPrefix(name, "access.log") || strings.HasPrefix(name, "t.jackyu.cn.log")
}

func buildABIndex(
	t *testing.T,
	name string,
	indexMapping mapping.IndexMapping,
	documents []*LogDocument,
) (int64, time.Duration) {
	t.Helper()

	indexPath := filepath.Join(t.TempDir(), name)
	config := DefaultIndexerConfig()
	index, err := bleve.NewUsing(
		indexPath,
		indexMapping,
		bleve.Config.DefaultIndexType,
		bleve.Config.DefaultMemKVStore,
		scorchRuntimeConfig(config),
	)
	require.NoError(t, err)
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = index.Close()
		}
	})

	startedAt := time.Now()
	for start := 0; start < len(documents); start += 1000 {
		end := min(start+1000, len(documents))
		batch := index.NewBatch()
		for documentIndex := start; documentIndex < end; documentIndex++ {
			require.NoError(t, batch.Index(fmt.Sprintf("doc-%08d", documentIndex), documents[documentIndex]))
		}
		require.NoError(t, index.Batch(batch))
	}
	advanced, err := index.Advanced()
	require.NoError(t, err)
	scorchIndex, ok := advanced.(*scorch.Scorch)
	require.True(t, ok)
	require.NoError(t, scorchIndex.ForceMerge(context.Background(), nil))
	duration := time.Since(startedAt)

	size, err := directorySize(indexPath)
	require.NoError(t, err)
	require.NoError(t, index.Close())
	closed = true
	return size, duration
}

func legacyLogIndexMapping() mapping.IndexMapping {
	indexMapping := bleve.NewIndexMapping()
	indexMapping.DefaultAnalyzer = "standard"
	documentMapping := bleve.NewDocumentMapping()

	addNumeric := func(name string, store bool) {
		field := bleve.NewNumericFieldMapping()
		field.Store = store
		field.Index = true
		documentMapping.AddFieldMappingsAt(name, field)
	}
	addText := func(name, analyzer string, store, index, docValues bool) {
		field := bleve.NewTextFieldMapping()
		field.Store = store
		field.Index = index
		field.Analyzer = analyzer
		field.DocValues = docValues
		documentMapping.AddFieldMappingsAt(name, field)
	}

	addNumeric("timestamp", true)
	addText("ip", "keyword", true, true, true)
	for _, name := range []string{"region_code", "province", "city"} {
		addText(name, "keyword", true, true, true)
	}
	addText("method", "keyword", true, true, true)
	addText("path", "standard", true, true, true)
	addText("path_exact", "keyword", false, true, true)
	addNumeric("status", true)
	addNumeric("bytes_sent", true)
	addText("referer", "standard", true, true, true)
	addText("user_agent", "standard", true, true, true)
	for _, name := range []string{"browser", "browser_version", "os", "os_version", "device_type"} {
		addText(name, "keyword", true, true, true)
	}
	addNumeric("request_time", true)
	addNumeric("upstream_time", true)
	addText("raw", "standard", true, false, true)
	addText("file_path", "keyword", true, true, true)
	addText("main_log_path", "keyword", true, true, true)

	indexMapping.AddDocumentMapping("_default", documentMapping)
	return indexMapping
}
