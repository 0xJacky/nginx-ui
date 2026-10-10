package cache

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"testing"
)

const benchConfigCount = 500

type benchConfig struct {
	path    string
	content []byte
}

var (
	benchSubdomains = []string{"www", "api", "shop", "blog", "mail", "cdn", "static", "admin", "app", "auth", "docs", "status", "media", "portal", "git", "wiki"}
	benchBrands     = []string{"acme-corp", "globex", "initech", "umbrella", "hooli", "stark-industries", "wayne_tech", "wonka", "tyrell", "cyberdyne", "soylent", "vandelay"}
	benchTLDs       = []string{"com", "net", "org", "io", "dev", "co.uk"}
)

// generateBenchConfigs builds a deterministic set of realistic nginx configs.
func generateBenchConfigs(count int) []benchConfig {
	rng := rand.New(rand.NewSource(42))
	configs := make([]benchConfig, 0, count)

	for i := 0; i < count; i++ {
		switch {
		case i%10 < 6:
			configs = append(configs, generateBenchSite(rng, i))
		case i%10 < 8:
			configs = append(configs, generateBenchStream(rng, i))
		default:
			configs = append(configs, generateBenchPlainConfig(rng, i))
		}
	}
	return configs
}

func benchDomain(rng *rand.Rand, i int) string {
	return fmt.Sprintf("%s%d.%s.%s",
		benchSubdomains[rng.Intn(len(benchSubdomains))], i,
		benchBrands[rng.Intn(len(benchBrands))],
		benchTLDs[rng.Intn(len(benchTLDs))])
}

func generateBenchSite(rng *rand.Rand, i int) benchConfig {
	domain := benchDomain(rng, i)
	name := domain
	if i%2 == 0 {
		name += ".conf"
	}
	port := []int{80, 8080, 8443, 3000, 9000}[rng.Intn(5)]
	backendPort := 3000 + rng.Intn(50)

	var b strings.Builder
	fmt.Fprintf(&b, "upstream backend_%d {\n    server 127.0.0.1:%d;\n    server 10.0.%d.%d:%d backup;\n    keepalive 32;\n}\n\n", i, backendPort, i%250, rng.Intn(250), backendPort)
	fmt.Fprintf(&b, "server {\n    listen %d;\n    listen [::]:%d;\n    server_name %s www.%s;\n\n", port, port, domain, domain)
	b.WriteString("    # Managed by Nginx UI\n")
	fmt.Fprintf(&b, "    access_log /var/log/nginx/%s.access.log main;\n    error_log /var/log/nginx/%s.error.log warn;\n\n", domain, domain)
	if i%3 == 0 {
		fmt.Fprintf(&b, "    listen 443 ssl http2;\n    ssl_certificate /etc/nginx/ssl/%s/fullchain.cer;\n    ssl_certificate_key /etc/nginx/ssl/%s/private.key;\n    ssl_protocols TLSv1.2 TLSv1.3;\n\n", domain, domain)
	}
	b.WriteString("    location / {\n")
	fmt.Fprintf(&b, "        proxy_pass http://backend_%d;\n", i)
	b.WriteString("        proxy_set_header Host $host;\n        proxy_set_header X-Real-IP $remote_addr;\n        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n        proxy_http_version 1.1;\n    }\n\n")
	b.WriteString("    location /static/ {\n        expires 30d;\n        add_header Cache-Control \"public\";\n    }\n\n")
	if i%4 == 0 {
		b.WriteString("    location ~ \\.php$ {\n        fastcgi_pass unix:/var/run/php/php8.2-fpm.sock;\n        include fastcgi_params;\n    }\n\n")
	}
	if i%5 == 0 {
		b.WriteString("    location /api/ {\n        limit_req zone=api burst=20 nodelay;\n        proxy_pass http://127.0.0.1:8000;\n    }\n")
	}
	b.WriteString("}\n")

	return benchConfig{path: "/etc/nginx/sites-available/" + name, content: []byte(b.String())}
}

func generateBenchStream(rng *rand.Rand, i int) benchConfig {
	port := 10000 + rng.Intn(5000)
	target := 5000 + rng.Intn(100)
	name := fmt.Sprintf("tcp-%s-%d.conf", benchBrands[rng.Intn(len(benchBrands))], i)

	content := fmt.Sprintf("upstream stream_backend_%d {\n    server 10.1.%d.%d:%d;\n    server 10.1.%d.%d:%d;\n}\n\nserver {\n    listen %d;\n    listen %d udp;\n    proxy_pass stream_backend_%d;\n    proxy_timeout 10m;\n    proxy_connect_timeout 5s;\n}\n",
		i, i%250, rng.Intn(250), target, i%250, rng.Intn(250), target+1, port, port+1, i)

	return benchConfig{path: "/etc/nginx/streams-available/" + name, content: []byte(content)}
}

func generateBenchPlainConfig(rng *rand.Rand, i int) benchConfig {
	kinds := []struct{ name, body string }{
		{"gzip", "gzip on;\ngzip_vary on;\ngzip_min_length 1024;\ngzip_types text/plain text/css application/json application/javascript;\n"},
		{"ratelimit", "limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;\nlimit_conn_zone $binary_remote_addr zone=addr:10m;\n"},
		{"logformat", "log_format main '$remote_addr - $remote_user [$time_local] \"$request\" $status $body_bytes_sent \"$http_referer\" \"$http_user_agent\"';\n"},
		{"proxy-cache", "proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=cache:10m max_size=1g inactive=60m;\nproxy_cache_key \"$scheme$request_method$host$request_uri\";\n"},
		{"security-headers", "add_header X-Frame-Options SAMEORIGIN;\nadd_header X-Content-Type-Options nosniff;\nadd_header Strict-Transport-Security \"max-age=31536000\";\n"},
	}
	kind := kinds[rng.Intn(len(kinds))]
	name := fmt.Sprintf("%s-%d.conf", kind.name, i)
	return benchConfig{path: "/etc/nginx/conf.d/" + name, content: []byte("# " + kind.name + " settings\n" + kind.body)}
}

func newBenchIndexer(b testing.TB) *SearchIndexer {
	b.Helper()
	indexer := &SearchIndexer{indexPath: "memory", maxMemoryUsage: 100 * 1024 * 1024}
	if err := indexer.Initialize(context.Background()); err != nil {
		b.Fatalf("Initialize() error = %v", err)
	}
	return indexer
}

func indexBenchConfigs(b testing.TB, indexer *SearchIndexer, configs []benchConfig) {
	b.Helper()
	for _, cfg := range configs {
		if err := indexer.handleConfigScan(cfg.path, cfg.content); err != nil {
			b.Fatalf("handleConfigScan(%s) error = %v", cfg.path, err)
		}
	}
}

func heapInUse() uint64 {
	runtime.GC()
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms.HeapAlloc
}

// BenchmarkSearchIndexAll measures indexing the whole corpus through the config scan entry.
func BenchmarkSearchIndexAll(b *testing.B) {
	configs := generateBenchConfigs(benchConfigCount)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		indexer := newBenchIndexer(b)
		b.StartTimer()
		indexBenchConfigs(b, indexer, configs)
		b.StopTimer()
		_ = indexer.Close()
		b.StartTimer()
	}
}

// BenchmarkSearchRetainedHeap reports the heap retained by an indexed corpus.
func BenchmarkSearchRetainedHeap(b *testing.B) {
	configs := generateBenchConfigs(benchConfigCount)
	var retained float64
	for i := 0; i < b.N; i++ {
		before := heapInUse()
		indexer := newBenchIndexer(b)
		indexBenchConfigs(b, indexer, configs)
		after := heapInUse()
		if after > before {
			retained += float64(after - before)
		}
		runtime.KeepAlive(indexer)
		_ = indexer.Close()
	}
	b.ReportMetric(retained/float64(b.N), "retained-heap-B")
}

type benchQuery struct {
	name    string
	query   string
	docType string
}

// benchQueries needs names that exist in the generated corpus.
func benchQueries(configs []benchConfig) []benchQuery {
	firstSite := ""
	for _, cfg := range configs {
		if strings.Contains(cfg.path, "sites-available") {
			firstSite = cfg.path[strings.LastIndex(cfg.path, "/")+1:]
			break
		}
	}
	typoSite := strings.Replace(firstSite, ".", ".x", 1)
	return []benchQuery{
		{"exact_name", firstSite, ""},
		{"prefix", firstSite[:len(firstSite)/2], ""},
		{"substring", "acme-corp", ""},
		{"port", "8443", ""},
		{"content_keyword", "proxy_pass", ""},
		{"one_typo_name", typoSite, ""},
		{"typo_word", "globx", ""},
		{"type_site", "acme", "site"},
		{"type_stream", "10.1", "stream"},
		{"type_config", "gzip", "config"},
	}
}

// BenchmarkSearchQueries measures a mix of queries against a 500 config corpus.
func BenchmarkSearchQueries(b *testing.B) {
	configs := generateBenchConfigs(benchConfigCount)
	indexer := newBenchIndexer(b)
	indexBenchConfigs(b, indexer, configs)
	b.Cleanup(func() { _ = indexer.Close() })

	ctx := context.Background()
	for _, q := range benchQueries(configs) {
		b.Run(q.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				var err error
				if q.docType == "" {
					_, err = indexer.Search(ctx, q.query, 500)
				} else {
					_, err = indexer.SearchByType(ctx, q.query, q.docType, 500)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	b.Run("mix", func(b *testing.B) {
		queries := benchQueries(configs)
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			for _, q := range queries {
				var err error
				if q.docType == "" {
					_, err = indexer.Search(ctx, q.query, 500)
				} else {
					_, err = indexer.SearchByType(ctx, q.query, q.docType, 500)
				}
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})
}

// goldenQueries is a fixed list used to compare result sets across implementations.
var goldenQueries = []benchQuery{
	{"exact site name", "api83.acme-corp.com", ""},
	{"name prefix", "shop", "site"},
	{"name substring", "corp", "site"},
	{"brand word", "globex", ""},
	{"brand with underscore", "wayne_tech", ""},
	{"port 80", "80", "site"},
	{"port 8443", "8443", ""},
	{"port stream", "10001", "stream"},
	{"content keyword", "fastcgi_pass", "site"},
	{"content directive", "limit_req", ""},
	{"config keyword", "gzip", "config"},
	{"stream keyword", "proxy_timeout", "stream"},
	{"stream name", "tcp-umbrella", "stream"},
	{"typo brand", "globx", "site"},
	{"typo brand transposed", "hooil", ""},
	{"typo long", "stark-industires", ""},
	{"cert path", "fullchain", "site"},
	{"uppercase query", "SHOP", "site"},
	{"tld", "co.uk", "site"},
	{"no match", "zzzzqqqq", ""},
}

// TestDumpGoldenSearchResults writes the ordered result IDs of goldenQueries when SEARCH_GOLDEN_OUT is set.
func TestDumpGoldenSearchResults(t *testing.T) {
	out := os.Getenv("SEARCH_GOLDEN_OUT")
	if out == "" {
		t.Skip("SEARCH_GOLDEN_OUT not set")
	}

	configs := generateBenchConfigs(benchConfigCount)
	indexer := newBenchIndexer(t)
	defer indexer.Close()
	indexBenchConfigs(t, indexer, configs)

	var sb strings.Builder
	ctx := context.Background()
	for _, q := range goldenQueries {
		results, err := indexer.SearchByType(ctx, q.query, q.docType, 5000)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&sb, "## %s | query=%q type=%q | %d results\n", q.name, q.query, q.docType, len(results))
		for _, r := range results {
			fmt.Fprintf(&sb, "%s\t%.3f\n", r.Document.ID, r.Score)
		}
	}
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
