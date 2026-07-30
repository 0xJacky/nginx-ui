package searcher

import (
	"context"
	"strings"
	"testing"

	"github.com/blevesearch/bleve/v2"
)

type panickingSearchIndex struct {
	indexDelegate
}

func (index *panickingSearchIndex) Search(*bleve.SearchRequest) (*bleve.SearchResult, error) {
	panic("corrupt shard")
}

func (index *panickingSearchIndex) SearchInContext(context.Context, *bleve.SearchRequest) (*bleve.SearchResult, error) {
	panic("corrupt shard")
}

func TestSearcherRecoversShardSearchPanic(t *testing.T) {
	index, err := bleve.NewMemOnly(bleve.NewIndexMapping())
	if err != nil {
		t.Fatalf("create index: %v", err)
	}
	defer index.Close()
	healthyIndex, err := bleve.NewMemOnly(bleve.NewIndexMapping())
	if err != nil {
		t.Fatalf("create healthy index: %v", err)
	}
	defer healthyIndex.Close()

	searcher := NewSearcher(DefaultSearcherConfig(), []bleve.Index{
		&panickingSearchIndex{indexDelegate: index},
		healthyIndex,
	})
	defer searcher.Stop()

	_, err = searcher.Search(context.Background(), &SearchRequest{})
	if err == nil {
		t.Fatal("Search() error = nil, want recovered shard panic")
	}
	if !strings.Contains(err.Error(), "corrupt shard") {
		t.Fatalf("Search() error = %q, want corrupt shard details", err)
	}
}
