package searcher

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// QueryBuilderService provides high-level query building functionality
type QueryBuilderService struct {
	defaultAnalyzer string
}

// NewQueryBuilderService creates a new query builder service
func NewQueryBuilderService() *QueryBuilderService {
	return &QueryBuilderService{
		defaultAnalyzer: "standard",
	}
}

// BuildQuery builds a Bleve query from a SearchRequest
func (qb *QueryBuilderService) BuildQuery(req *SearchRequest) (query.Query, error) {
	if req == nil {
		return nil, fmt.Errorf("search request cannot be nil")
	}

	// Build main query
	var mainQuery query.Query

	if req.Query == "" {
		mainQuery = bleve.NewMatchAllQuery()
	} else {
		mainQuery = bleve.NewMatchQuery(req.Query)
	}

	// Create boolean query to combine filters
	boolQuery := bleve.NewBooleanQuery()
	boolQuery.AddMust(mainQuery)

	// Add time range filters
	if req.StartTime != nil || req.EndTime != nil {
		if timeQuery := qb.buildTimeRangeQuery(req.StartTime, req.EndTime); timeQuery != nil {
			boolQuery.AddMust(timeQuery)
		}
	}

	// Add log path filters
	if len(req.LogPaths) > 0 {
		if logPathQuery := qb.buildTermsQuery("file_path", req.LogPaths); logPathQuery != nil {
			boolQuery.AddMust(logPathQuery)
		}
	}

	// Add IP address filters
	if len(req.IPAddresses) > 0 {
		if ipQuery := qb.buildTermsQuery("remote_addr", req.IPAddresses); ipQuery != nil {
			boolQuery.AddMust(ipQuery)
		}
	}

	// Add status code filters
	if len(req.StatusCodes) > 0 {
		if statusQuery := qb.buildStatusCodeQuery(req.StatusCodes); statusQuery != nil {
			boolQuery.AddMust(statusQuery)
		}
	}

	// Add method filters
	if len(req.Methods) > 0 {
		if methodQuery := qb.buildTermsQuery("request_method", req.Methods); methodQuery != nil {
			boolQuery.AddMust(methodQuery)
		}
	}

	// Add country filters
	if len(req.Countries) > 0 {
		if countryQuery := qb.buildTermsQuery("region_code", req.Countries); countryQuery != nil {
			boolQuery.AddMust(countryQuery)
		}
	}

	// Log the final query structure for debugging.
	queryBytes, err := json.Marshal(boolQuery)
	if err == nil {
		logger.Debugf("Constructed Bleve Query: %s", string(queryBytes))
	}

	return boolQuery, nil
}

// buildTimeRangeQuery builds a time range query
func (qb *QueryBuilderService) buildTimeRangeQuery(start, end *int64) query.Query {
	if start == nil && end == nil {
		return nil
	}

	var min, max *float64
	if start != nil {
		startFloat := float64(*start)
		min = &startFloat
	}
	if end != nil {
		endFloat := float64(*end)
		max = &endFloat
	}

	rangeQuery := bleve.NewNumericRangeQuery(min, max)
	rangeQuery.SetField("timestamp")

	return rangeQuery
}

// buildTermsQuery builds a terms query for multiple values
func (qb *QueryBuilderService) buildTermsQuery(field string, terms []string) query.Query {
	if len(terms) == 0 {
		return nil
	}

	if len(terms) == 1 {
		termQuery := bleve.NewTermQuery(terms[0])
		termQuery.SetField(field)
		return termQuery
	}

	// For multiple terms, use boolean OR
	boolQuery := bleve.NewBooleanQuery()
	for _, term := range terms {
		termQuery := bleve.NewTermQuery(term)
		termQuery.SetField(field)
		boolQuery.AddShould(termQuery)
	}
	boolQuery.SetMinShould(1) // Crucial for OR behavior

	return boolQuery
}

// buildStatusCodeQuery builds a status code query
func (qb *QueryBuilderService) buildStatusCodeQuery(codes []int) query.Query {
	if len(codes) == 0 {
		return nil
	}

	// Convert status codes to strings for term query
	terms := make([]string, len(codes))
	for i, code := range codes {
		terms[i] = strconv.Itoa(code)
	}

	return qb.buildTermsQuery("status", terms)
}

// ValidateSearchRequest validates a search request
func (qb *QueryBuilderService) ValidateSearchRequest(req *SearchRequest) error {
	if req == nil {
		return fmt.Errorf("search request cannot be nil")
	}

	if req.Limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}

	if req.Offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}


	// Validate sort order
	if req.SortOrder != "" && req.SortOrder != SortOrderAsc && req.SortOrder != SortOrderDesc {
		return fmt.Errorf("invalid sort order: %s", req.SortOrder)
	}

	return nil
}

// BuildSuggestionQuery builds a query for suggestions (simplified version)
func (qb *QueryBuilderService) BuildSuggestionQuery(text, field string) (query.Query, error) {
	if text == "" {
		return nil, fmt.Errorf("suggestion text cannot be empty")
	}

	// Use a prefix query for suggestions
	if field == "" {
		field = "_all"
	}

	prefixQuery := bleve.NewPrefixQuery(strings.ToLower(text))
	prefixQuery.SetField(field)

	return prefixQuery, nil
}
