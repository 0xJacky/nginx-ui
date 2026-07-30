package searcher

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"

	"github.com/blevesearch/bleve/v2"
	"github.com/uozi-tech/cosy/logger"
)

type indexDelegate interface {
	bleve.Index
}

type recoveringIndex struct {
	indexDelegate
}

func wrapRecoveringIndexes(indexes []bleve.Index) []bleve.Index {
	wrapped := make([]bleve.Index, 0, len(indexes))
	for _, index := range indexes {
		if index == nil {
			continue
		}
		wrapped = append(wrapped, &recoveringIndex{indexDelegate: index})
	}
	return wrapped
}

func (index *recoveringIndex) Search(request *bleve.SearchRequest) (result *bleve.SearchResult, err error) {
	return index.SearchInContext(context.Background(), request)
}

func (index *recoveringIndex) SearchInContext(ctx context.Context, request *bleve.SearchRequest) (
	result *bleve.SearchResult,
	err error,
) {
	defer func() {
		if recovered := recover(); recovered != nil {
			name := index.Name()
			logger.Errorf("Recovered panic while searching Bleve index %q: %v\n%s", name, recovered, debug.Stack())
			result = nil
			err = fmt.Errorf("search panic in index %q: %v", name, recovered)
		}
	}()

	return index.indexDelegate.SearchInContext(ctx, request)
}

func searchResultError(result *bleve.SearchResult) error {
	if result == nil || result.Status == nil || result.Status.Failed == 0 {
		return nil
	}

	names := make([]string, 0, len(result.Status.Errors))
	for name := range result.Status.Errors {
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return fmt.Errorf("distributed search failed on %d of %d shards", result.Status.Failed, result.Status.Total)
	}

	name := names[0]
	return fmt.Errorf(
		"distributed search failed on %d of %d shards, including %q: %w",
		result.Status.Failed,
		result.Status.Total,
		name,
		result.Status.Errors[name],
	)
}
