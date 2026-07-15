package searcher

import (
	"testing"

	"github.com/blevesearch/bleve/v2/search/query"
)

// collectFieldQueries walks a boolean query tree and returns all
// field-scoped queries keyed by field name.
func collectFieldQueries(t *testing.T, q query.Query, fields map[string]query.Query) {
	t.Helper()

	switch typed := q.(type) {
	case *query.BooleanQuery:
		if typed.Must != nil {
			if conj, ok := typed.Must.(*query.ConjunctionQuery); ok {
				for _, sub := range conj.Conjuncts {
					collectFieldQueries(t, sub, fields)
				}
			}
		}
	case *query.NumericRangeQuery:
		fields[typed.FieldVal] = typed
	case *query.TermQuery:
		fields[typed.FieldVal] = typed
	}
}

func TestBuildQueryNumericRangeFilters(t *testing.T) {
	qb := NewQueryBuilder()

	minBytes := int64(100)
	maxBytes := int64(5000)
	minReqTime := 0.5

	req := &SearchRequest{
		MinBytes:   &minBytes,
		MaxBytes:   &maxBytes,
		MinReqTime: &minReqTime,
	}

	q, err := qb.BuildQuery(req)
	if err != nil {
		t.Fatalf("BuildQuery() error = %v", err)
	}

	fields := make(map[string]query.Query)
	collectFieldQueries(t, q, fields)

	bytesQuery, ok := fields["bytes_sent"].(*query.NumericRangeQuery)
	if !ok {
		t.Fatal("expected a numeric range query on bytes_sent")
	}
	if bytesQuery.Min == nil || *bytesQuery.Min != float64(minBytes) {
		t.Errorf("bytes_sent min = %v, want %v", bytesQuery.Min, float64(minBytes))
	}
	if bytesQuery.Max == nil || *bytesQuery.Max != float64(maxBytes) {
		t.Errorf("bytes_sent max = %v, want %v", bytesQuery.Max, float64(maxBytes))
	}

	reqTimeQuery, ok := fields["request_time"].(*query.NumericRangeQuery)
	if !ok {
		t.Fatal("expected a numeric range query on request_time")
	}
	if reqTimeQuery.Min == nil || *reqTimeQuery.Min != minReqTime {
		t.Errorf("request_time min = %v, want %v", reqTimeQuery.Min, minReqTime)
	}
	if reqTimeQuery.Max != nil {
		t.Errorf("request_time max = %v, want nil", reqTimeQuery.Max)
	}
}

func TestBuildQueryWithoutRangeFilters(t *testing.T) {
	qb := NewQueryBuilder()

	q, err := qb.BuildQuery(&SearchRequest{})
	if err != nil {
		t.Fatalf("BuildQuery() error = %v", err)
	}

	fields := make(map[string]query.Query)
	collectFieldQueries(t, q, fields)

	if _, exists := fields["bytes_sent"]; exists {
		t.Error("bytes_sent filter should not be present when no range is requested")
	}
	if _, exists := fields["request_time"]; exists {
		t.Error("request_time filter should not be present when no range is requested")
	}
}
