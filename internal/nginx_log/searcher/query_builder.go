package searcher

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/uozi-tech/cosy/logger"
)

// QueryBuilder provides high-level query building functionality
type QueryBuilder struct {
	defaultAnalyzer string
}

// NewQueryBuilder creates a new query builder service
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		defaultAnalyzer: "standard",
	}
}

// BuildQuery builds a Bleve query from a SearchRequest
func (qb *QueryBuilder) BuildQuery(req *SearchRequest) (query.Query, error) {
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

	// Add log path filters - use main_log_path for efficient log group queries or file_path for specific files
	if len(req.LogPaths) > 0 {
		var fieldName string
		if req.UseMainLogPath {
			fieldName = "main_log_path"
		} else {
			fieldName = "file_path"
		}
		if logPathQuery := qb.buildTermsQuery(fieldName, req.LogPaths); logPathQuery != nil {
			boolQuery.AddMust(logPathQuery)
		}
	}

	// Add IP address filters
	if len(req.IPAddresses) > 0 {
		if ipQuery := qb.buildTermsQuery("ip", req.IPAddresses); ipQuery != nil {
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
		if methodQuery := qb.buildTermsQuery("method", req.Methods); methodQuery != nil {
			boolQuery.AddMust(methodQuery)
		}
	}

	// Add country filters
	if len(req.Countries) > 0 {
		if countryQuery := qb.buildTermsQuery("region_code", req.Countries); countryQuery != nil {
			boolQuery.AddMust(countryQuery)
		}
	}

	// Add request path filters
	if len(req.Paths) > 0 {
		if pathQuery := qb.buildMatchPhraseQuery("path", req.Paths); pathQuery != nil {
			boolQuery.AddMust(pathQuery)
		}
	}

	// Add user agent filters
	if len(req.UserAgents) > 0 {
		if uaQuery := qb.buildMatchPhraseQuery("user_agent", req.UserAgents); uaQuery != nil {
			boolQuery.AddMust(uaQuery)
		}
	}

	// Add referer filters
	if len(req.Referers) > 0 {
		if refererQuery := qb.buildMatchPhraseQuery("referer", req.Referers); refererQuery != nil {
			boolQuery.AddMust(refererQuery)
		}
	}

	// Add browser filters
	if len(req.Browsers) > 0 {
		if browserQuery := qb.buildTermsQuery("browser", req.Browsers); browserQuery != nil {
			boolQuery.AddMust(browserQuery)
		}
	}

	// Add operating system filters
	if len(req.OSs) > 0 {
		if osQuery := qb.buildTermsQuery("os", req.OSs); osQuery != nil {
			boolQuery.AddMust(osQuery)
		}
	}

	// Add device type filters
	if len(req.Devices) > 0 {
		if deviceQuery := qb.buildTermsQuery("device_type", req.Devices); deviceQuery != nil {
			boolQuery.AddMust(deviceQuery)
		}
	}

	// Log the final query structure for debugging.
	queryBytes, err := json.Marshal(boolQuery)
	if err == nil {
		logger.Debugf("Constructed Bleve Query: %s", string(queryBytes))
	}

	// Additional debug: if using main_log_path, also log a diagnostic query without filters
	if len(req.LogPaths) > 0 && req.UseMainLogPath {
		logger.Debugf("DEBUG: Dashboard query using main_log_path field with path: %v", req.LogPaths)
	}

	return boolQuery, nil
}

// buildTimeRangeQuery builds a time range query
func (qb *QueryBuilder) buildTimeRangeQuery(start, end *int64) query.Query {
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
func (qb *QueryBuilder) buildTermsQuery(field string, terms []string) query.Query {
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

// buildMatchPhraseQuery builds a match phrase query for analyzed text fields.
// A phrase query matches the analyzed terms in order, which keeps filters such
// as request path precise instead of loosely matching any single shared token.
// Multiple values are combined with boolean OR.
func (qb *QueryBuilder) buildMatchPhraseQuery(field string, values []string) query.Query {
	if len(values) == 0 {
		return nil
	}

	if len(values) == 1 {
		phraseQuery := bleve.NewMatchPhraseQuery(values[0])
		phraseQuery.SetField(field)
		return phraseQuery
	}

	// For multiple values, use boolean OR
	boolQuery := bleve.NewBooleanQuery()
	for _, value := range values {
		phraseQuery := bleve.NewMatchPhraseQuery(value)
		phraseQuery.SetField(field)
		boolQuery.AddShould(phraseQuery)
	}
	boolQuery.SetMinShould(1) // Crucial for OR behavior

	return boolQuery
}

// buildStatusCodeQuery builds a status code query.
// The status field is indexed as a numeric field, so each code is matched
// with an inclusive numeric range query instead of a text term query.
func (qb *QueryBuilder) buildStatusCodeQuery(codes []int) query.Query {
	if len(codes) == 0 {
		return nil
	}

	if len(codes) == 1 {
		return qb.buildStatusCodeRangeQuery(codes[0])
	}

	// For multiple status codes, use boolean OR
	boolQuery := bleve.NewBooleanQuery()
	for _, code := range codes {
		boolQuery.AddShould(qb.buildStatusCodeRangeQuery(code))
	}
	boolQuery.SetMinShould(1) // Crucial for OR behavior

	return boolQuery
}

// buildStatusCodeRangeQuery builds an inclusive numeric range query that
// matches exactly one status code on the numeric "status" field.
func (qb *QueryBuilder) buildStatusCodeRangeQuery(code int) query.Query {
	value := float64(code)
	inclusive := true
	rangeQuery := bleve.NewNumericRangeInclusiveQuery(&value, &value, &inclusive, &inclusive)
	rangeQuery.SetField("status")
	return rangeQuery
}

// ValidateSearchRequest validates a search request
func (qb *QueryBuilder) ValidateSearchRequest(req *SearchRequest) error {
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
func (qb *QueryBuilder) BuildSuggestionQuery(text, field string) (query.Query, error) {
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
