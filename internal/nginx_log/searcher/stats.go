package searcher

import (
	"context"
	"math"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

const (
	// statsScanBatchSize is the number of documents pulled per scan page.
	statsScanBatchSize = 10000

	// maxStatsScanDocs caps how many documents a stats scan reads. Beyond this
	// the totals are extrapolated from the scanned prefix and the result is
	// flagged approximate, so an unfiltered search over a large index cannot
	// turn into an unbounded scan.
	maxStatsScanDocs = 200000
)

// searchStats returns the statistics for the request's match set, reusing a
// cached aggregation when one is available. The cache entry deliberately
// ignores pagination, so paging through a result set scans only once.
func (s *Searcher) searchStats(
	ctx context.Context,
	q query.Query,
	req *SearchRequest,
	totalHits uint64,
	indexAlias bleve.IndexAlias,
) *SearchStats {
	useCache := s.config.EnableCache && req.UseCache && s.cache != nil
	if useCache {
		if cached := s.cache.GetSearchStats(req); cached != nil {
			return cached
		}
	}

	stats := s.computeStats(ctx, q, totalHits, indexAlias)
	if useCache && stats != nil {
		s.cache.PutSearchStats(req, stats, DefaultCacheTTL)
	}

	return stats
}

// computeStats aggregates byte and response-time statistics over the documents
// matching the query. Bleve has no native sum aggregation, so the matches are
// scanned with a SearchAfter cursor that loads only the two numeric fields
// involved; each page costs O(page) instead of the O(offset+page) of offset
// pagination, keeping a full scan linear in the number of documents.
//
// It must be called from within Search, which already holds the concurrency
// semaphore, hence the direct use of the index alias rather than Search itself.
func (s *Searcher) computeStats(
	ctx context.Context,
	q query.Query,
	totalHits uint64,
	indexAlias bleve.IndexAlias,
) *SearchStats {
	stats := &SearchStats{}
	if totalHits == 0 {
		return stats
	}

	var (
		scanned     uint64
		minBytes    int64 = math.MaxInt64
		maxBytes    int64
		minReqTime  = math.MaxFloat64
		maxReqTime  float64
		searchAfter []string
	)

	for scanned < maxStatsScanDocs {
		size := statsScanBatchSize
		if remaining := maxStatsScanDocs - scanned; remaining < uint64(size) {
			size = int(remaining)
		}

		searchReq := bleve.NewSearchRequest(q)
		searchReq.Size = size
		searchReq.Fields = []string{"bytes_sent", "request_time"}
		// Sort on the same (timestamp, _id) key the rest of the scanning code
		// uses: the timestamp alone is not unique, so the document ID is needed
		// as a tiebreaker for a stable cursor.
		searchReq.SortBy([]string{"timestamp", "_id"})
		if len(searchAfter) > 0 {
			searchReq.SearchAfter = searchAfter
		}

		result, err := indexAlias.SearchInContext(ctx, searchReq)
		if err != nil {
			// A cancelled context or shard error leaves a partial scan, which
			// is still a better estimate than nothing; extrapolation below
			// marks the outcome approximate.
			logger.Warnf("Stats scan stopped after %d documents: %v", scanned, err)
			break
		}
		if err := searchResultError(result); err != nil {
			logger.Warnf("Stats scan stopped after %d documents: %v", scanned, err)
			break
		}
		if len(result.Hits) == 0 {
			break
		}

		for _, hit := range result.Hits {
			if v, ok := numericField(hit.Fields, "bytes_sent"); ok {
				b := int64(v)
				stats.TotalBytes += b
				if b < minBytes {
					minBytes = b
				}
				if b > maxBytes {
					maxBytes = b
				}
			}
			if v, ok := numericField(hit.Fields, "request_time"); ok {
				stats.TotalReqTime += v
				if v < minReqTime {
					minReqTime = v
				}
				if v > maxReqTime {
					maxReqTime = v
				}
			}
		}

		scanned += uint64(len(result.Hits))

		if len(result.Hits) < size {
			break
		}

		lastHit := result.Hits[len(result.Hits)-1]
		if len(lastHit.Sort) == 0 {
			logger.Warnf("Stats scan: last hit carries no sort values, cannot continue pagination (scanned %d)", scanned)
			break
		}
		searchAfter = lastHit.Sort
	}

	if scanned == 0 {
		return stats
	}

	stats.ScannedDocs = scanned
	stats.AvgBytes = float64(stats.TotalBytes) / float64(scanned)
	stats.AvgReqTime = stats.TotalReqTime / float64(scanned)

	if minBytes != math.MaxInt64 {
		stats.MinBytes = minBytes
	}
	stats.MaxBytes = maxBytes
	if minReqTime != math.MaxFloat64 {
		stats.MinReqTime = minReqTime
	}
	stats.MaxReqTime = maxReqTime

	// The scan stopped short of the full match set: scale the sums by the ratio
	// of matches to scanned documents so the totals stay comparable, and say so.
	if scanned < totalHits {
		stats.Approximate = true
		stats.TotalBytes = int64(stats.AvgBytes * float64(totalHits))
		stats.TotalReqTime = stats.AvgReqTime * float64(totalHits)
	}

	return stats
}

// numericField reads a stored numeric field from a hit. Bleve returns every
// stored numeric value as float64 regardless of the mapped Go type.
func numericField(fields map[string]interface{}, name string) (float64, bool) {
	value, ok := fields[name]
	if !ok {
		return 0, false
	}
	f, ok := value.(float64)
	return f, ok
}
