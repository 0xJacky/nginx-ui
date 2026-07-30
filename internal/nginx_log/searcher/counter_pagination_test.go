package searcher

import (
	"context"
	"fmt"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type paginationRequestSnapshot struct {
	from        int
	searchAfter []string
	sortCount   int
}

type paginationTestAlias struct {
	bleve.IndexAlias
	requests []paginationRequestSnapshot
}

func (alias *paginationTestAlias) SearchInContext(
	ctx context.Context,
	request *bleve.SearchRequest,
) (*bleve.SearchResult, error) {
	alias.requests = append(alias.requests, paginationRequestSnapshot{
		from:        request.From,
		searchAfter: append([]string(nil), request.SearchAfter...),
		sortCount:   len(request.Sort),
	})

	estimatedMemory := uint64(request.From + 1)
	if callback, ok := ctx.Value(bleve.SearchQueryStartCallbackKey).(bleve.SearchQueryStartCallbackFn); ok {
		if err := callback(estimatedMemory); err != nil {
			return nil, err
		}
	}
	if callback, ok := ctx.Value(bleve.SearchQueryEndCallbackKey).(bleve.SearchQueryEndCallbackFn); ok {
		defer func() { _ = callback(estimatedMemory) }()
	}

	var hits search.DocumentMatchCollection
	switch len(alias.requests) {
	case 1:
		hits = make(search.DocumentMatchCollection, 10_000)
		for i := range hits {
			id := fmt.Sprintf("doc-%05d", i)
			hits[i] = &search.DocumentMatch{
				ID:     id,
				Fields: map[string]interface{}{"path": "/same"},
				Sort:   []string{id},
			}
		}
	case 2:
		hits = search.DocumentMatchCollection{&search.DocumentMatch{
			ID:     "doc-10000",
			Fields: map[string]interface{}{"path": "/same"},
			Sort:   []string{"doc-10000"},
		}}
	}

	return &bleve.SearchResult{
		Status: &bleve.SearchStatus{Total: 1, Successful: 1},
		Hits:   hits,
		Total:  10_001,
	}, nil
}

func TestCounterPaginationUsesBoundedSearchAfterMemory(t *testing.T) {
	alias := &paginationTestAlias{}
	limiter := newSearchMemoryLimiter(1)
	counter := &Counter{memoryLimit: limiter}
	ctx := withSearchMemoryLimit(context.Background(), limiter)

	terms, total, err := counter.collectTermsUsingPagination(ctx, &CardinalityRequest{Field: "path"}, alias)
	require.NoError(t, err)
	assert.Equal(t, uint64(10_001), total)
	assert.Equal(t, map[string]struct{}{`/same`: {}}, terms)
	require.Len(t, alias.requests, 2)
	assert.Zero(t, alias.requests[0].from)
	assert.Empty(t, alias.requests[0].searchAfter)
	assert.NotZero(t, alias.requests[0].sortCount)
	assert.Zero(t, alias.requests[1].from)
	assert.Equal(t, []string{"doc-09999"}, alias.requests[1].searchAfter)
	assert.NotZero(t, alias.requests[1].sortCount)
	assert.Zero(t, limiter.used.Load())
}
