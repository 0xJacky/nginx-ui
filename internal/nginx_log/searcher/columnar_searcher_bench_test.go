package searcher

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
	"github.com/blevesearch/bleve/v2"
)

// benchDocs is a combined-format corpus with realistic skew: a few hot paths,
// IPs and user agents, and a tail of unique paths.
func benchDocs(n int) []*indexer.Document {
	r := rand.New(rand.NewSource(42))
	ips := make([]string, 5000)
	for i := range ips {
		ips[i] = fmt.Sprintf("%d.%d.%d.%d", 1+r.Intn(223), r.Intn(256), r.Intn(256), 1+r.Intn(254))
	}
	uas := []struct{ ua, browser, os, device string }{
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36", "Chrome", "Windows", "desktop"},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1", "Mobile Safari", "iOS", "mobile"},
		{"curl/8.4.0", "curl", "", "bot"},
		{"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "Googlebot", "", "bot"},
	}
	paths := []string{"/", "/api/v1/products", "/api/v1/users", "/static/js/app.js", "/favicon.ico", "/login"}
	zip := rand.NewZipf(r, 1.1, 2, uint64(len(ips)-1))
	docs := make([]*indexer.Document, n)
	for i := range docs {
		ua := uas[r.Intn(len(uas))]
		path := paths[r.Intn(len(paths))]
		if r.Intn(4) == 0 {
			path += "/" + strconv.Itoa(r.Intn(100_000))
		}
		ip := ips[zip.Uint64()]
		ts := int64(1_760_000_000 + i/2)
		doc := &indexer.LogDocument{
			Timestamp: ts, IP: ip, RegionCode: []string{"CN", "US", "JP", "DE"}[r.Intn(4)],
			Method: []string{"GET", "GET", "GET", "POST"}[r.Intn(4)], Path: path, PathExact: path, Protocol: "HTTP/1.1",
			Status: []int{200, 200, 200, 304, 404, 500}[r.Intn(6)], BytesSent: int64(r.Intn(50_000)),
			Referer: "-", UserAgent: ua.ua, Browser: ua.browser, OS: ua.os, DeviceType: ua.device,
			RequestTime: float64(r.Intn(2000)) / 1000,
			FilePath:    "/var/log/nginx/access.log", MainLogPath: "/var/log/nginx/access.log",
		}
		doc.Raw = fmt.Sprintf(`%s - - [%s] "%s %s HTTP/1.1" %d %d "-" "%s"`, ip,
			time.Unix(ts, 0).UTC().Format("02/Jan/2006:15:04:05 -0700"), doc.Method, path, doc.Status, doc.BytesSent, ua.ua)
		docs[i] = &indexer.Document{ID: doc.FilePath + "-" + strconv.Itoa(i), Fields: doc}
	}
	return docs
}

func dirBytes(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}

func rawBytes(docs []*indexer.Document) int64 {
	var n int64
	for _, d := range docs {
		n += int64(len(d.Fields.Raw) + 1)
	}
	return n
}

const benchCorpusSize = 100_000

// BenchmarkIngest compares writing the same parsed documents to Bleve with the
// production mapping and to the column store. disk-B/doc is the on-disk size
// once each is closed; raw-x is that size over the raw log size.
func BenchmarkIngest(b *testing.B) {
	docs := benchDocs(benchCorpusSize)
	raw := rawBytes(docs)

	b.Run("bleve", func(b *testing.B) {
		var size int64
		for b.Loop() {
			dir := filepath.Join(b.TempDir(), "idx")
			index, err := bleve.New(dir, indexer.CreateLogIndexMapping())
			if err != nil {
				b.Fatal(err)
			}
			batch := index.NewBatch()
			for _, d := range docs {
				if err := batch.Index(d.ID, d.Fields); err != nil {
					b.Fatal(err)
				}
				if batch.Size() >= 2000 {
					if err := index.Batch(batch); err != nil {
						b.Fatal(err)
					}
					batch.Reset()
				}
			}
			if err := index.Batch(batch); err != nil {
				b.Fatal(err)
			}
			if err := index.Close(); err != nil {
				b.Fatal(err)
			}
			size = dirBytes(dir)
		}
		b.ReportMetric(float64(size)/float64(len(docs)), "disk-B/doc")
		b.ReportMetric(float64(size)/float64(raw), "raw-x")
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(docs)), "ns/doc")
	})

	b.Run("logstore", func(b *testing.B) {
		var size int64
		for b.Loop() {
			dir := b.TempDir()
			store, err := logstore.Open(dir, logstore.Options{})
			if err != nil {
				b.Fatal(err)
			}
			w, err := store.Writer(docs[0].Fields.MainLogPath, docs[0].Fields.FilePath)
			if err != nil {
				b.Fatal(err)
			}
			for _, d := range docs {
				if err := w.Append(indexer.LogDocumentToRow(d.Fields), 0); err != nil {
					b.Fatal(err)
				}
			}
			if err := w.Flush(); err != nil {
				b.Fatal(err)
			}
			_ = store.Close()
			size = dirBytes(dir)
		}
		b.ReportMetric(float64(size)/float64(len(docs)), "disk-B/doc")
		b.ReportMetric(float64(size)/float64(raw), "raw-x")
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*len(docs)), "ns/doc")
	})
}

// BenchmarkSearchRequests runs the requests the log viewer and dashboard send
// against both engines over the same corpus.
func BenchmarkSearchRequests(b *testing.B) {
	docs := benchDocs(benchCorpusSize)

	index, err := bleve.New(filepath.Join(b.TempDir(), "idx"), indexer.CreateLogIndexMapping())
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = index.Close() }()
	batch := index.NewBatch()
	for _, d := range docs {
		_ = batch.Index(d.ID, d.Fields)
		if batch.Size() >= 2000 {
			_ = index.Batch(batch)
			batch.Reset()
		}
	}
	_ = index.Batch(batch)

	store, err := logstore.Open(b.TempDir(), logstore.Options{})
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	w, _ := store.Writer(docs[0].Fields.MainLogPath, docs[0].Fields.FilePath)
	for _, d := range docs {
		_ = w.Append(indexer.LogDocumentToRow(d.Fields), 0)
	}
	_ = w.Flush()

	config := DefaultSearcherConfig()
	config.EnableCache = false
	engines := map[string]SearcherInterface{
		"bleve":    NewSearcher(config, []bleve.Index{index}),
		"logstore": NewColumnarSearcher(config, store),
	}

	requests := map[string]SearchRequest{
		"viewer-page":       {Limit: 50, IncludeFacets: true, FacetFields: []string{"ip", "path_exact"}, FacetSize: 10000, IncludeStats: true},
		"viewer-filtered":   {StatusCodes: []int{404}, Paths: []string{"/api/v1/users"}, Limit: 50},
		"dashboard-facets":  {Limit: -1, IncludeFacets: true, FacetFields: []string{"browser", "os", "device_type"}, FacetSize: 50},
		"dashboard-top-url": {Limit: -1, IncludeFacets: true, FacetFields: []string{"path_exact"}, FacetSize: 100},
		"geo-world":         {Limit: -1, IncludeFacets: true, FacetFields: []string{"region_code"}, FacetSize: 300},
		"deep-page":         {Limit: 50, Offset: 20_000},
	}
	for reqName, req := range requests {
		for engineName, engine := range engines {
			b.Run(reqName+"/"+engineName, func(b *testing.B) {
				for b.Loop() {
					r := req
					if _, err := engine.Search(context.Background(), &r); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
