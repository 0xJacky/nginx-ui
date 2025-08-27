package searcher

import (
	"context"
	"sort"
	"strings"

	"github.com/blevesearch/bleve/v2/search"
)

// convertFacets converts Bleve facets to our facet format
func (ds *DistributedSearcher) convertFacets(bleveFacets search.FacetResults) map[string]*Facet {
	facets := make(map[string]*Facet)

	for name, result := range bleveFacets {
		facet := &Facet{
			Field:   name,
			Total:   result.Total,
			Missing: result.Missing,
			Other:   result.Other,
			Terms:   make([]*FacetTerm, 0),
		}

		// Handle Terms facet
		if result.Terms != nil {
			for _, term := range result.Terms.Terms() {
				facet.Terms = append(facet.Terms, &FacetTerm{
					Term:  term.Term,
					Count: term.Count,
				})
			}
		}

		facets[name] = facet
	}

	return facets
}

// mergeFacets merges facet results from multiple shards
func (ds *DistributedSearcher) mergeFacets(combined, incoming map[string]*Facet) {
	for field, incomingFacet := range incoming {
		if existingFacet, exists := combined[field]; exists {
			ds.mergeSingleFacet(existingFacet, incomingFacet)
		} else {
			combined[field] = ds.copyFacet(incomingFacet)
		}
	}
}

// mergeSingleFacet merges two facets for the same field
func (ds *DistributedSearcher) mergeSingleFacet(existing, incoming *Facet) {
	// Note: Do NOT sum Total values - it represents unique terms count, not document count
	// The Total should be recalculated based on the actual number of unique terms after merging
	existing.Missing += incoming.Missing
	existing.Other += incoming.Other

	// Merge terms
	termCounts := make(map[string]int)

	// Add existing terms
	for _, term := range existing.Terms {
		termCounts[term.Term] = term.Count
	}

	// Add incoming terms
	for _, term := range incoming.Terms {
		termCounts[term.Term] += term.Count
	}

	// Convert back to slice and sort by count
	terms := make([]*FacetTerm, 0, len(termCounts))
	for term, count := range termCounts {
		terms = append(terms, &FacetTerm{
			Term:  term,
			Count: count,
		})
	}

	// Sort by count (descending) then by term (ascending)
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].Count == terms[j].Count {
			return terms[i].Term < terms[j].Term
		}
		return terms[i].Count > terms[j].Count
	})

	// Limit to top terms
	if len(terms) > DefaultFacetSize {
		// Calculate "other" count
		otherCount := 0
		for _, term := range terms[DefaultFacetSize:] {
			otherCount += term.Count
		}
		existing.Other += otherCount
		terms = terms[:DefaultFacetSize]
	}

	existing.Terms = terms
	// Set Total to the actual number of unique terms (not sum of totals)
	existing.Total = len(termCounts)
}

// copyFacet creates a deep copy of a facet
func (ds *DistributedSearcher) copyFacet(original *Facet) *Facet {
	facet := &Facet{
		Field:   original.Field,
		Total:   original.Total,
		Missing: original.Missing,
		Other:   original.Other,
		Terms:   make([]*FacetTerm, len(original.Terms)),
	}

	for i, term := range original.Terms {
		facet.Terms[i] = &FacetTerm{
			Term:  term.Term,
			Count: term.Count,
		}
	}

	return facet
}

// Aggregate performs aggregations on search results
func (ds *DistributedSearcher) Aggregate(ctx context.Context, req *AggregationRequest) (*AggregationResult, error) {
	// This is a simplified implementation
	// In a full implementation, you would execute the aggregation across all shards
	// and merge the results similar to how facets are handled

	result := &AggregationResult{
		Field: req.Field,
		Type:  req.Type,
	}

	// For now, return a placeholder result
	// This would need to be implemented based on specific requirements
	switch req.Type {
	case AggregationTerms:
		result.Data = map[string]interface{}{
			"buckets": []map[string]interface{}{},
		}
	case AggregationStats:
		result.Data = map[string]interface{}{
			"count": 0,
			"min":   0,
			"max":   0,
			"avg":   0,
			"sum":   0,
		}
	case AggregationHistogram:
		result.Data = map[string]interface{}{
			"buckets": []map[string]interface{}{},
		}
	case AggregationDateHistogram:
		result.Data = map[string]interface{}{
			"buckets": []map[string]interface{}{},
		}
	case AggregationCardinality:
		result.Data = map[string]interface{}{
			"value": 0,
		}
	}

	return result, nil
}

// Suggest provides search suggestions
func (ds *DistributedSearcher) Suggest(ctx context.Context, text string, field string, size int) ([]*Suggestion, error) {
	if size <= 0 || size > 100 {
		size = 10
	}

	// Create search request
	req := &SearchRequest{
		Query:     text,
		Fields:    []string{field},
		Limit:     size * 2, // Get more results to have better suggestions
		SortBy:    "_score",
		SortOrder: SortOrderDesc,
	}

	// Execute search
	result, err := ds.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert results to suggestions
	suggestions := make([]*Suggestion, 0, size)
	seen := make(map[string]bool)

	for _, hit := range result.Hits {
		if len(suggestions) >= size {
			break
		}

		// Extract text from the specified field
		if fieldValue, exists := hit.Fields[field]; exists {
			if textValue, ok := fieldValue.(string); ok {
				// Simple suggestion extraction - this could be made more sophisticated
				terms := ds.extractSuggestionTerms(textValue, text)

				for _, term := range terms {
					if len(suggestions) >= size {
						break
					}

					if !seen[term] && strings.Contains(strings.ToLower(term), strings.ToLower(text)) {
						suggestions = append(suggestions, &Suggestion{
							Text:  term,
							Score: hit.Score,
							Freq:  1, // Would need to be calculated from corpus
						})
						seen[term] = true
					}
				}
			}
		}
	}

	// Sort suggestions by score
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Score > suggestions[j].Score
	})

	return suggestions, nil
}

// extractSuggestionTerms extracts potential suggestion terms from text
func (ds *DistributedSearcher) extractSuggestionTerms(text string, query string) []string {
	// Simple term extraction - this could be enhanced with NLP
	terms := strings.Fields(text)

	// Filter and clean terms
	var suggestions []string
	for _, term := range terms {
		term = strings.TrimSpace(term)
		if len(term) > 2 && !isCommonWord(term) {
			suggestions = append(suggestions, term)
		}
	}

	return suggestions
}

// isCommonWord checks if a word is too common to be a good suggestion
func isCommonWord(word string) bool {
	commonWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true,
		"a": true, "an": true, "as": true, "is": true,
		"are": true, "was": true, "were": true, "be": true,
		"been": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true,
		"would": true, "could": true, "should": true, "may": true,
		"might": true, "must": true, "can": true, "shall": true,
	}

	return commonWords[strings.ToLower(word)]
}

// Analyze analyzes text using a specified analyzer
func (ds *DistributedSearcher) Analyze(ctx context.Context, text string, analyzer string) ([]string, error) {
	// This would typically use Bleve's analysis capabilities
	// For now, provide a simple implementation

	if analyzer == "" {
		analyzer = "standard"
	}

	// Simple tokenization - this should use proper analyzers
	terms := strings.Fields(strings.ToLower(text))

	// Remove punctuation and short terms
	var analyzed []string
	for _, term := range terms {
		term = strings.Trim(term, ".,!?;:\"'()[]{}/-_")
		if len(term) > 2 {
			analyzed = append(analyzed, term)
		}
	}

	return analyzed, nil
}

// Cache operations
func (ds *DistributedSearcher) getFromCache(req *SearchRequest) *SearchResult {
	if ds.cache == nil {
		return nil
	}

	return ds.cache.Get(req)
}

func (ds *DistributedSearcher) cacheResult(req *SearchRequest, result *SearchResult) {
	if ds.cache == nil {
		return
	}

	ds.cache.Put(req, result, DefaultCacheTTL)
}

// ClearCache clears the search cache
func (ds *DistributedSearcher) ClearCache() error {
	if ds.cache != nil {
		ds.cache.Clear()
	}
	return nil
}

// GetCacheStats returns cache statistics
func (ds *DistributedSearcher) GetCacheStats() *CacheStats {
	if ds.cache != nil {
		return ds.cache.GetStats()
	}
	return nil
}
