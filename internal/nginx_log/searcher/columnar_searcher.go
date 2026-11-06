package searcher

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/logstore"
)

// ColumnarSearcher serves SearchRequests from a logstore.Store instead of
// Bleve shards. It is a prototype of the replacement described in
// internal/nginx_log/logstore/DESIGN.md: same request and result types, so the
// API handlers do not change.
//
// Differences from the Bleve searcher, all deliberate:
//   - Query is the search box syntax of logstore.ParseSearchText: words match
//     the raw line as case-insensitive substrings, every term required, and
//     field:value filters (status:5xx, ip:10.0.0.0/8, path:/api, -bot, ...)
//     narrow further. Bleve ORed analyzed tokens.
//   - Paths, UserAgents and Referers are case-insensitive substrings instead
//     of phrase queries over analyzed tokens.
//   - Highlighting is not produced; no caller reads it.
//   - Stats are exact; they never sample and extrapolate.
//   - Results are not cached: a cache keyed on the request returns stale
//     pages after an append, and a column scan is cheap enough to repeat.
type ColumnarSearcher struct {
	store   *logstore.Store
	config  *Config
	stopped atomic.Bool

	total, failed atomic.Int64
}

// NewColumnarSearcher wraps a store.
func NewColumnarSearcher(config *Config, store *logstore.Store) *ColumnarSearcher {
	if config == nil {
		config = DefaultSearcherConfig()
	}
	return &ColumnarSearcher{store: store, config: config}
}

// CardinalityCounter counts distinct values among the rows a request's
// filters select.
type CardinalityCounter interface {
	Cardinality(ctx context.Context, field string, req *SearchRequest) (uint64, error)
}

// BlockScanner streams the decoded columns of every block a request selects,
// for aggregations a SearchRequest cannot express.
type BlockScanner interface {
	Scan(ctx context.Context, req *SearchRequest, columns []string, fn func(*logstore.BlockView) error) error
}

var (
	_ CardinalityCounter = (*ColumnarSearcher)(nil)
	_ BlockScanner       = (*ColumnarSearcher)(nil)
)

// defaultFacetFields matches the Bleve searcher's default facet set.
var defaultFacetFields = []string{"status", "method", "browser", "os", "device_type", "region_code"}

// Search implements SearcherInterface.
func (s *ColumnarSearcher) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	if s.stopped.Load() {
		return nil, fmt.Errorf("searcher stopped")
	}
	start := time.Now()
	s.total.Add(1)

	timeout := req.Timeout
	if timeout == 0 {
		timeout = s.config.TimeoutDuration
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, err := s.search(ctx, req)
	if err != nil {
		s.failed.Add(1)
		return nil, err
	}
	result.Duration = time.Since(start)
	return result, nil
}

func (s *ColumnarSearcher) search(ctx context.Context, req *SearchRequest) (*SearchResult, error) {
	q := QueryFromRequest(req)
	opts, err := searchOptionsFromRequest(req, s.config.DefaultLimit)
	if err != nil {
		return nil, err
	}

	res, err := s.store.Search(ctx, q, opts)
	if err != nil {
		return nil, err
	}

	_, warnings := logstore.ParseSearchText(req.Query)
	out := &SearchResult{
		Hits:          make([]*SearchHit, 0, len(res.Hits)),
		TotalHits:     res.Total,
		Facets:        map[string]*Facet{},
		QueryWarnings: warnings,
	}
	sortField := opts.SortBy
	for _, h := range res.Hits {
		id := h.File + "-" + strconv.FormatInt(h.Ordinal, 10)
		out.Hits = append(out.Hits, &SearchHit{
			ID:     id,
			Score:  1,
			Fields: hitFields(h, opts.Columns),
			Index:  h.Group,
			Sort:   []string{formatSortValue(sortField, h.Key), id},
		})
	}

	if req.IncludeFacets {
		fields := req.FacetFields
		if len(fields) == 0 {
			fields = defaultFacetFields
		}
		size := req.FacetSize
		if size <= 0 {
			size = DefaultFacetSize
		}
		// One extra term leaves room to rank the empty value among the rest.
		facets, _, err := s.store.Facets(ctx, q, fields, size+1)
		if err != nil {
			return nil, err
		}
		for name, f := range facets {
			out.Facets[name] = bleveCompatibleFacet(name, f, size)
		}
	}

	if req.IncludeStats {
		st, err := s.store.Stats(ctx, q)
		if err != nil {
			return nil, err
		}
		out.Stats = searchStatsFrom(st)
	}
	return out, nil
}

// bleveCompatibleFacet reports an empty value as a term, as Bleve does: the
// keyword analyzer indexes "" like any other value, so Bleve has no missing
// rows for these fields and ranks "" among the top terms.
func bleveCompatibleFacet(name string, f *logstore.FacetResult, size int) *Facet {
	terms := make([]*FacetTerm, 0, len(f.Terms)+1)
	withValue := f.Other
	for _, t := range f.Terms {
		terms = append(terms, &FacetTerm{Term: t.Term, Count: t.Count})
		withValue += t.Count
	}
	if f.Missing > 0 {
		terms = append(terms, &FacetTerm{Term: "", Count: f.Missing})
		withValue += f.Missing
	}
	sort.SliceStable(terms, func(i, j int) bool { return terms[i].Count > terms[j].Count })
	if len(terms) > size {
		terms = terms[:size]
	}
	other := withValue
	for _, t := range terms {
		other -= t.Count
	}
	// Total is the number of returned terms, as convertBleveResult reports it.
	return &Facet{Field: name, Total: len(terms), Other: other, Terms: terms}
}

// QueryFromRequest translates the filter part of a SearchRequest.
func QueryFromRequest(req *SearchRequest) *logstore.Query {
	clauses, _ := logstore.ParseSearchText(req.Query)
	q := &logstore.Query{
		Clauses:           clauses,
		IPs:               req.IPAddresses,
		Methods:           req.Methods,
		RegionCodes:       req.Countries,
		Provinces:         req.Provinces,
		Browsers:          req.Browsers,
		OSs:               req.OSs,
		Devices:           req.Devices,
		Statuses:          req.StatusCodes,
		PathContains:      req.Paths,
		UserAgentContains: req.UserAgents,
		RefererContains:   req.Referers,
		MinBytes:          req.MinBytes,
		MaxBytes:          req.MaxBytes,
		MinRequestTime:    req.MinReqTime,
		MaxRequestTime:    req.MaxReqTime,
	}
	if len(req.LogPaths) > 0 {
		if req.UseMainLogPath {
			q.Groups = req.LogPaths
		} else {
			q.Files = req.LogPaths
		}
	}
	// Bleve's numeric range is start-inclusive and end-exclusive; so is the store's.
	if req.StartTime != nil {
		q.From = *req.StartTime
	}
	if req.EndTime != nil {
		q.To = *req.EndTime
	}
	return q
}

func searchOptionsFromRequest(req *SearchRequest, defaultLimit int) (logstore.SearchOptions, error) {
	opts := logstore.SearchOptions{
		SortBy: req.SortBy,
		Desc:   req.SortOrder != SortOrderAsc,
		Offset: req.Offset,
		Limit:  req.Limit,
	}
	if opts.SortBy == "" || opts.SortBy == DefaultSortField {
		opts.SortBy = "timestamp"
	}
	switch {
	case req.Limit == 0:
		opts.Limit = defaultLimit
	case req.Limit < 0:
		opts.Limit = 0
	}
	for _, f := range req.Fields {
		if f == "*" {
			opts.Columns = nil
			break
		}
		opts.Columns = append(opts.Columns, f)
	}
	if len(req.SearchAfter) > 0 {
		key, err := parseSortCursor(opts.SortBy, req.SearchAfter)
		if err != nil {
			return opts, err
		}
		opts.After = key
	}
	return opts, nil
}

func isNumericSortField(field string) bool {
	switch field {
	case "timestamp", "status", "bytes_sent", "request_time", "upstream_time":
		return true
	}
	return false
}

func formatSortValue(field string, key logstore.SortKey) string {
	if isNumericSortField(field) {
		return strconv.FormatInt(key.Num, 10)
	}
	return key.Str
}

// parseSortCursor reads the [value, id] pair that Search puts in Hit.Sort.
func parseSortCursor(field string, after []string) (*logstore.SortKey, error) {
	if len(after) != 2 {
		return nil, fmt.Errorf("search_after needs [sort value, id], got %d values", len(after))
	}
	cut := strings.LastIndexByte(after[1], '-')
	if cut < 0 {
		return nil, fmt.Errorf("search_after id %q is not file-ordinal", after[1])
	}
	ordinal, err := strconv.ParseInt(after[1][cut+1:], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("search_after id %q: %w", after[1], err)
	}
	key := &logstore.SortKey{File: after[1][:cut], Ordinal: ordinal}
	if isNumericSortField(field) {
		if key.Num, err = strconv.ParseInt(after[0], 10, 64); err != nil {
			return nil, fmt.Errorf("search_after value %q: %w", after[0], err)
		}
	} else {
		key.Str = after[0]
	}
	return key, nil
}

// hitFields renders a row the way Bleve returns stored fields: numbers as
// float64, every stored string even when empty (Bleve indexes the struct, so
// json omitempty does not apply), plus the file and group paths.
func hitFields(h *logstore.Hit, columns []string) map[string]interface{} {
	r := h.Row
	all := map[string]interface{}{
		"timestamp":       float64(r.Timestamp),
		"status":          float64(r.Status),
		"bytes_sent":      float64(r.BytesSent),
		"file_path":       h.File,
		"main_log_path":   h.Group,
		"request_time":    r.RequestTime,
		"ip":              r.IP,
		"region_code":     r.RegionCode,
		"province":        r.Province,
		"city":            r.City,
		"c1":              r.C1,
		"c2":              r.C2,
		"c3":              r.C3,
		"c4":              r.C4,
		"method":          r.Method,
		"path":            r.Path,
		"protocol":        r.Protocol,
		"referer":         r.Referer,
		"user_agent":      r.UserAgent,
		"browser":         r.Browser,
		"browser_version": r.BrowserVersion,
		"os":              r.OS,
		"os_version":      r.OSVersion,
		"device_type":     r.DeviceType,
		"raw":             r.Raw,
	}
	if r.UpstreamTime != nil {
		all["upstream_time"] = *r.UpstreamTime
	}
	if len(columns) == 0 {
		return all
	}
	out := make(map[string]interface{}, len(columns))
	for _, c := range columns {
		if v, ok := all[c]; ok {
			out[c] = v
		}
	}
	return out
}

func searchStatsFrom(st *logstore.Stats) *SearchStats {
	out := &SearchStats{
		TotalBytes:     st.TotalBytes,
		MinBytes:       st.MinBytes,
		MaxBytes:       st.MaxBytes,
		TotalReqTime:   st.TotalReqTime,
		MinReqTime:     st.MinReqTime,
		MaxReqTime:     st.MaxReqTime,
		UniqueIPs:      st.UniqueIPs,
		UniquePaths:    st.UniquePaths,
		StatusCodeDist: st.StatusCodeDist,
		MethodDist:     st.MethodDist,
		ScannedDocs:    st.Count,
	}
	if st.Count > 0 {
		out.AvgBytes = float64(st.TotalBytes) / float64(st.Count)
		out.AvgReqTime = st.TotalReqTime / float64(st.Count)
	}
	return out
}

// Cardinality counts distinct values of field among the rows the request's
// filters select. Unlike Counter.Count it applies every filter, not just the
// time range and log paths.
func (s *ColumnarSearcher) Cardinality(ctx context.Context, field string, req *SearchRequest) (uint64, error) {
	return s.store.Distinct(ctx, QueryFromRequest(req), field)
}

// Scan exposes the store's block scan for aggregations the request type
// cannot express, such as the dashboard's hour/day/minute buckets.
func (s *ColumnarSearcher) Scan(ctx context.Context, req *SearchRequest, columns []string, fn func(*logstore.BlockView) error) error {
	return s.store.Scan(ctx, QueryFromRequest(req), columns, fn)
}

// ClearCache implements SearcherInterface; there is no cache.
func (s *ColumnarSearcher) ClearCache() error { return nil }

// GetCacheStats implements SearcherInterface; there is no cache.
func (s *ColumnarSearcher) GetCacheStats() *CacheStats { return &CacheStats{} }

// IsHealthy implements SearcherInterface.
func (s *ColumnarSearcher) IsHealthy() bool { return !s.stopped.Load() }

// IsRunning implements SearcherInterface.
func (s *ColumnarSearcher) IsRunning() bool { return !s.stopped.Load() }

// GetStats implements SearcherInterface.
func (s *ColumnarSearcher) GetStats() *Stats {
	total, failed := s.total.Load(), s.failed.Load()
	return &Stats{TotalSearches: total, SuccessfulSearches: total - failed, FailedSearches: failed, CacheStats: &CacheStats{}}
}

// GetConfig implements SearcherInterface.
func (s *ColumnarSearcher) GetConfig() *Config { return s.config }

// Stop implements SearcherInterface. The store is owned by the caller.
func (s *ColumnarSearcher) Stop() error {
	s.stopped.Store(true)
	return nil
}

var _ SearcherInterface = (*ColumnarSearcher)(nil)
