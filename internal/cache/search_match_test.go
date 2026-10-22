package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indexTestDocs(t *testing.T, indexer *SearchIndexer, docs ...SearchDocument) {
	t.Helper()
	for _, doc := range docs {
		if doc.Path == "" {
			doc.Path = "/etc/nginx/sites-available/" + doc.Name
		}
		if doc.ID == "" {
			doc.ID = doc.Path
		}
		if doc.Type == "" {
			doc.Type = "site"
		}
		require.NoError(t, indexer.IndexDocument(doc))
	}
}

func searchIDs(t *testing.T, indexer *SearchIndexer, query string) []string {
	t.Helper()
	results, err := indexer.Search(context.Background(), query, 100)
	require.NoError(t, err)
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, result.Document.Name)
	}
	return ids
}

func TestSearchScoringTiers(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "shop", Content: "server { listen 80; }"},
		SearchDocument{Name: "shop-eu.example.com", Content: "server { listen 80; }"},
		SearchDocument{Name: "my-shop-admin", Content: "server { listen 80; }"},
		SearchDocument{Name: "shoq.example.com", Content: "server { listen 80; }"},
		SearchDocument{Name: "unrelated", Path: "/etc/nginx/sites-available/shopdir/unrelated", Content: "server { listen 80; }"},
		SearchDocument{Name: "backend", Content: "location / { proxy_pass http://shop.internal; }"},
		SearchDocument{Name: "other", Content: "server { listen 80; }"},
	)

	assert.Equal(t, []string{
		"shop",                // exact name
		"shop-eu.example.com", // name prefix
		"my-shop-admin",       // name contains
		"shoq.example.com",    // typo in a name token
		"unrelated",           // path contains
		"backend",             // content contains
	}, searchIDs(t, indexer, "shop"))
}

func TestSearchExactNameIgnoresExtensionAndCase(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "Example.COM.conf", Content: "server {}"},
		SearchDocument{Name: "example.community", Content: "server {}"},
	)

	results, err := indexer.Search(context.Background(), " EXAMPLE.com ", 10)
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, "Example.COM.conf", results[0].Document.Name)
	assert.Equal(t, scoreNameExact, results[0].Score)
	assert.Less(t, results[1].Score, results[0].Score)
}

func TestSearchScoreBonusesStayInsideTier(t *testing.T) {
	assert.Greater(t, scoreNameExact, scorePort)
	assert.Greater(t, scorePort, scoreNamePrefix+closeness("a", "a"))
	assert.Greater(t, scoreNamePrefix, scoreNameContains+closeness("a", "a"))
	assert.Greater(t, scoreNameContains, scoreNameFuzzy+closeness("a", "a"))
	assert.Greater(t, scoreNameFuzzy, scorePathContains)
	assert.Greater(t, scorePathContains, scoreContent+maxContentBonus)
	assert.Greater(t, scoreContent, scoreContentTerms)
}

func TestSearchPrefixPrefersCloserNames(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "api-gateway-production-eu", Content: "x"},
		SearchDocument{Name: "api-gateway", Content: "x"},
	)
	assert.Equal(t, []string{"api-gateway", "api-gateway-production-eu"}, searchIDs(t, indexer, "api-g"))
}

func TestSearchContentPrefersMoreOccurrences(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "once", Content: "proxy_pass a;"},
		SearchDocument{Name: "thrice", Content: "proxy_pass a; proxy_pass b; PROXY_PASS c;"},
	)
	assert.Equal(t, []string{"thrice", "once"}, searchIDs(t, indexer, "proxy_pass"))
}

func TestSearchContentMatchIsCaseInsensitive(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "add_header X-Frame-Options SAMEORIGIN;"})

	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "x-frame-options"))
	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "SameOrigin"))
}

func TestSearchMultiWordQuery(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "phrase", Content: "server_name example.com;"},
		SearchDocument{Name: "terms", Content: "server_name other.org;\nproxy_pass http://example.com;"},
		SearchDocument{Name: "partial", Content: "server_name other.org;"},
	)
	assert.Equal(t, []string{"phrase", "terms"}, searchIDs(t, indexer, "server_name example.com"))
}

func TestSearchTypoTolerance(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "example.com", Content: "x"},
		SearchDocument{Name: "stark-industries.net", Content: "x"},
	)

	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "substitution", query: "exampls", want: []string{"example.com"}},
		{name: "missing letter", query: "exmple", want: []string{"example.com"}},
		{name: "extra letter", query: "exxample", want: []string{"example.com"}},
		{name: "transposition", query: "exmaple", want: []string{"example.com"}},
		{name: "two typos in a short token", query: "exmpls", want: nil},
		{name: "typo in a multi token query with two edits", query: "stark-indutsrie", want: []string{"stark-industries.net"}},
		{name: "three typos in a multi token query", query: "stark-indutsrei", want: nil},
		{name: "two typos in a token of 8 or more", query: "industrei", want: []string{"stark-industries.net"}},
		{name: "three typos in a long token", query: "indstrxs", want: nil},
		{name: "too short for fuzzy", query: "ex", want: []string{"example.com"}}, // prefix, not fuzzy
		{name: "two letter typo is ignored", query: "qx", want: nil},
		{name: "typo in a multi token query", query: "stark-industires", want: []string{"stark-industries.net"}},
		{name: "typo in the first of several tokens", query: "strak-industries", want: []string{"stark-industries.net"}},
		{name: "several tokens with too many typos", query: "strak-industrei", want: nil},
		{name: "unrelated", query: "zzzzzz", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nilIfEmpty(searchIDs(t, indexer, tt.query)))
		})
	}
}

func nilIfEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return values
}

func TestSearchTypoRanksBelowSubstringMatches(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "globx.example.com", Content: "x"},
		SearchDocument{Name: "globex.example.com", Content: "x"},
		SearchDocument{Name: "my-globx-site", Content: "x"},
	)
	assert.Equal(t, []string{"globx.example.com", "my-globx-site", "globex.example.com"}, searchIDs(t, indexer, "globx"))
}

func TestSearchNumericQueriesPreferPorts(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "listens-8080", Content: "server {\n    listen 127.0.0.1:8080;\n    listen [::]:8080;\n}"},
		SearchDocument{Name: "8080", Content: "server { listen 80; }"},
		SearchDocument{Name: "8080-proxy", Content: "server { listen 80; }"},
		SearchDocument{Name: "mentions", Content: "proxy_pass http://backend:8080;\n# listen 8080;"},
		SearchDocument{Name: "unrelated", Content: "server { listen 9090; }"},
	)

	// Exact name, then port match, then name prefix, then content
	assert.Equal(t, []string{"8080", "listens-8080", "8080-proxy", "mentions"}, searchIDs(t, indexer, "8080"))
	// A port prefix only matches ports and content
	assert.Equal(t, []string{"listens-8080", "mentions"}, searchIDs(t, indexer, ":8080"))
	assert.Equal(t, []string{"8080", "listens-8080", "8080-proxy", "mentions"}, searchIDs(t, indexer, "port:8080"))
}

func TestSearchNumericQueriesDoNotUseTypoTolerance(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "site-9005", Content: "x"},
		SearchDocument{Name: "site-9006", Content: "x"},
	)
	assert.Equal(t, []string{"site-9005"}, searchIDs(t, indexer, "9005"))
}

func TestSearchNumericQueryIgnoresDirectoryDigits(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Path: "/srv/1080/sites-available/site", Content: "server { listen 90; }"})
	assert.Empty(t, searchIDs(t, indexer, "80"))
}

func TestSearchDocTypeFilter(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{Name: "proxy-site", Type: "site", Content: "x"},
		SearchDocument{Name: "proxy-stream", Type: "stream", Content: "x"},
		SearchDocument{Name: "proxy-config", Type: "config", Content: "x"},
	)
	ctx := context.Background()

	all, err := indexer.Search(ctx, "proxy", 10)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	for docType, want := range map[string]string{"site": "proxy-site", "stream": "proxy-stream", "config": "proxy-config"} {
		results, err := indexer.SearchByType(ctx, "proxy", docType, 10)
		require.NoError(t, err)
		require.Len(t, results, 1, docType)
		assert.Equal(t, want, results[0].Document.Name)
		assert.Equal(t, docType, results[0].Document.Type)
	}

	results, err := indexer.SearchByType(ctx, "proxy", "unknown", 10)
	require.NoError(t, err)
	assert.Empty(t, results)

	sites, err := indexer.SearchByType(ctx, "proxy", "site", 10)
	require.NoError(t, err)
	assert.Len(t, sites, 1)
}

func TestSearchResultDocumentFields(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	updatedAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	indexTestDocs(t, indexer, SearchDocument{
		ID: "id-1", Type: "stream", Name: "Mixed-Case.conf", Path: "/etc/nginx/streams-available/Mixed-Case.conf",
		Content: "server { listen 3306; }", UpdatedAt: updatedAt,
	})

	results, err := indexer.Search(context.Background(), "mixed-case", 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	doc := results[0].Document
	// Callers map results back to files, so case must be preserved
	assert.Equal(t, "id-1", doc.ID)
	assert.Equal(t, "stream", doc.Type)
	assert.Equal(t, "Mixed-Case.conf", doc.Name)
	assert.Equal(t, "/etc/nginx/streams-available/Mixed-Case.conf", doc.Path)
	assert.True(t, doc.UpdatedAt.Equal(updatedAt))
	assert.Positive(t, results[0].Score)
}

func TestSearchLimit(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	for i := 0; i < 20; i++ {
		indexTestDocs(t, indexer, SearchDocument{Name: fmt.Sprintf("site-%02d", i), Content: "x"})
	}
	ctx := context.Background()

	results, err := indexer.Search(ctx, "site", 5)
	require.NoError(t, err)
	require.Len(t, results, 5)
	// The limit keeps the best results in a stable order
	for i, result := range results {
		assert.Equal(t, fmt.Sprintf("site-%02d", i), result.Document.Name)
	}

	results, err = indexer.Search(ctx, "site", 0)
	require.NoError(t, err)
	assert.Len(t, results, 20)
}

func TestSearchEmptyQuery(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "x"})

	results, err := indexer.Search(context.Background(), "   ", 10)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestSearchHonorsContextCancellation(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "x"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := indexer.Search(ctx, "site", 10)
	assert.ErrorIs(t, err, context.Canceled)

	// A cancelled scan of a larger index stops as well
	for i := 0; i < 300; i++ {
		indexTestDocs(t, indexer, SearchDocument{Name: fmt.Sprintf("bulk-%d", i), Content: "x"})
	}
	_, err = indexer.SearchByType(ctx, "bulk", "site", 10)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSearchDeleteAndReindex(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "server { listen 80; }"})
	require.Equal(t, []string{"site"}, searchIDs(t, indexer, "site"))

	require.NoError(t, indexer.DeleteDocument("/etc/nginx/sites-available/site"))
	assert.Empty(t, searchIDs(t, indexer, "site"))
	// Deleting a missing document is not an error
	require.NoError(t, indexer.DeleteDocument("/etc/nginx/sites-available/site"))

	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "server { listen 80; }"})
	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "site"))
}

func TestSearchUpdateReplacesDocument(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "server { listen 80; server_name old.example.com; }"})
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "server { listen 9090; server_name new.example.com; }"})

	assert.Empty(t, searchIDs(t, indexer, "old.example.com"))
	assert.Empty(t, searchIDs(t, indexer, "80;"))
	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "new.example.com"))
	assert.Equal(t, 1, documentCount(t, indexer))
}

func TestSearchRebuildEmptiesIndex(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "x"})

	require.NoError(t, indexer.RebuildIndex(context.Background()))
	assert.Empty(t, searchIDs(t, indexer, "site"))

	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "x"})
	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "site"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.ErrorIs(t, indexer.RebuildIndex(ctx), context.Canceled)
	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "site"))
}

func TestSearchConcurrentAccess(t *testing.T) {
	indexer := newTestSearchIndexer(t, 1<<24)
	ctx := context.Background()

	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				path := fmt.Sprintf("/etc/nginx/sites-available/site-%d-%d", worker, i%20)
				_ = indexer.IndexDocument(SearchDocument{
					ID: path, Type: "site", Name: fmt.Sprintf("site-%d-%d", worker, i%20), Path: path,
					Content: fmt.Sprintf("server { listen %d; }", 8000+i),
				})
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_, _ = indexer.Search(ctx, "site", 50)
				_, _ = indexer.SearchByType(ctx, "8010", "site", 50)
				_, _ = indexer.GetIndexStats()
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = indexer.DeleteDocument(fmt.Sprintf("/etc/nginx/sites-available/site-%d-%d", worker, i%20))
				if i%50 == 0 {
					_ = indexer.RebuildIndex(ctx)
				}
			}
		}()
	}
	wg.Wait()

	totalBytes, count, _ := indexer.getMemoryUsage()
	assert.GreaterOrEqual(t, totalBytes, int64(0))
	assert.Equal(t, int64(documentCount(t, indexer)), count)
}

func TestSearchReinitializeKeepsNewIndex(t *testing.T) {
	indexer := &SearchIndexer{maxMemoryUsage: 1 << 20}
	first, cancelFirst := context.WithCancel(context.Background())
	require.NoError(t, indexer.Initialize(first))
	require.NoError(t, indexer.Initialize(context.Background()))
	t.Cleanup(func() { _ = indexer.Close() })
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "x"})

	// The first context expiring must not clear the index created later
	cancelFirst()
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, []string{"site"}, searchIDs(t, indexer, "site"))
}

func TestSearchContextCancellationReleasesIndex(t *testing.T) {
	indexer := &SearchIndexer{maxMemoryUsage: 1 << 20}
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, indexer.Initialize(ctx))
	indexTestDocs(t, indexer, SearchDocument{Name: "site", Content: "x"})

	cancel()
	require.Eventually(t, func() bool {
		_, err := indexer.GetIndexStats()
		return err != nil
	}, 2*time.Second, 10*time.Millisecond)
}

func TestExtractListenPorts(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []uint16
	}{
		{name: "plain", content: "server { listen 80; }", want: []uint16{80}},
		{name: "options", content: "server { listen 443 ssl http2; listen 8443 ssl; }", want: []uint16{443, 8443}},
		{name: "address and port", content: "listen 127.0.0.1:8080;\nlisten 0.0.0.0:9000 default_server;", want: []uint16{8080, 9000}},
		{name: "ipv6", content: "listen [::]:443 ssl;\nlisten [::]:80;", want: []uint16{443, 80}},
		{name: "ipv6 without port", content: "listen [::1];", want: nil},
		{name: "unix socket", content: "listen unix:/var/run/nginx.sock;", want: nil},
		{name: "duplicates", content: "listen 80;\nlisten [::]:80;\nlisten 80 default_server;", want: []uint16{80}},
		{name: "comment", content: "# listen 81;\nlisten 82; # listen 83;", want: []uint16{82}},
		{name: "not a directive", content: "proxy_pass http://x; set $listen 90; server_name listen;", want: nil},
		{name: "same line directives", content: "server { listen 1; } server { listen 2; }", want: []uint16{1, 2}},
		{name: "out of range", content: "listen 70000; listen 0;", want: nil},
		{name: "uppercase directive", content: "LISTEN 8000;", want: []uint16{8000}},
		{name: "empty", content: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractListenPorts(tt.content))
		})
	}
}

func TestSplitNameTokens(t *testing.T) {
	assert.Equal(t, []string{"api", "acme", "corp", "com", "conf"}, splitNameTokens("api.acme-corp.com.conf"))
	assert.Equal(t, []string{"a", "b", "c", "d"}, splitNameTokens("a_b c/d"))
	assert.Empty(t, splitNameTokens("..."))
}

func TestBoundedEditDistance(t *testing.T) {
	tests := []struct {
		a, b  string
		limit int
		want  int
	}{
		{"example", "example", 1, 0},
		{"example", "exampl", 1, 1},
		{"example", "examples", 1, 1},
		{"example", "exampke", 1, 1},
		{"example", "exmaple", 1, 1}, // transposition counts once
		{"example", "exmpls", 1, -1},
		{"example", "exmpls", 2, 2},
		{"example", "ex", 2, -1},
		{"", "abc", 3, -1},
		{"abc", "", 3, -1},
		{"café", "cafe", 1, 1},
		{"abcdefgh", "abcdefhg", 2, 1},
		{"abcdefgh", "bacdefhg", 2, 2},
		{"abcdefgh", "bcdefghi", 2, 2},
		{"abcdefgh", "bcdefghij", 2, -1},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s_%d", tt.a, tt.b, tt.limit), func(t *testing.T) {
			assert.Equal(t, tt.want, boundedEditDistance(tt.a, tt.b, tt.limit))
		})
	}
}

func TestSearchPathMatchesBelowConfigRootOnly(t *testing.T) {
	previous := settings.NginxSettings.ConfigDir
	settings.NginxSettings.ConfigDir = "/etc/nginx"
	t.Cleanup(func() { settings.NginxSettings.ConfigDir = previous })

	indexer := newTestSearchIndexer(t, 1<<20)
	indexTestDocs(t, indexer,
		SearchDocument{ID: "a", Type: "site", Name: "shop.conf", Path: "/etc/nginx/sites-available/shop.conf", Content: "server {}"},
		SearchDocument{ID: "b", Type: "site", Name: "blog.conf", Path: "/etc/nginx/sites-available/blog.conf", Content: "server {}"},
	)

	results, err := indexer.Search(context.Background(), "nginx", 10)
	require.NoError(t, err)
	assert.Empty(t, results)

	results, err = indexer.Search(context.Background(), "sites-available", 10)
	require.NoError(t, err)
	assert.Len(t, results, 2)
}
