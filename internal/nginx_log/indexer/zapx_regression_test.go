package indexer

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/index/scorch"
)

func TestZapxSparsePostingMergeReadback(t *testing.T) {
	const (
		documentCount = 20_000
		batchSize     = 500
		sparseCount   = 1_100
	)

	indexPath := filepath.Join(t.TempDir(), "index")
	index, err := bleve.NewUsing(
		indexPath,
		CreateLogIndexMapping(),
		bleve.Config.DefaultIndexType,
		bleve.Config.DefaultMemKVStore,
		nil,
	)
	if err != nil {
		t.Fatalf("create index: %v", err)
	}
	defer index.Close()

	for batchStart := 0; batchStart < documentCount; batchStart += batchSize {
		batch := index.NewBatch()
		for i := batchStart; i < batchStart+batchSize; i++ {
			document := map[string]any{
				"timestamp": int64(i + 1),
				"raw":       "GET /health HTTP/1.1",
			}
			if i < sparseCount || i >= documentCount-sparseCount {
				document["referer"] = "sparse-marker"
			}
			batch.Index(fmt.Sprintf("doc-%05d", i), document)
		}
		if err := index.Batch(batch); err != nil {
			t.Fatalf("index batch starting at %d: %v", batchStart, err)
		}
	}

	advancedIndex, err := index.Advanced()
	if err != nil {
		t.Fatalf("get advanced index: %v", err)
	}
	scorchIndex, ok := advancedIndex.(*scorch.Scorch)
	if !ok {
		t.Fatalf("advanced index type = %T, want *scorch.Scorch", advancedIndex)
	}
	if err := scorchIndex.ForceMerge(context.Background(), nil); err != nil {
		t.Fatalf("force merge: %v", err)
	}

	query := bleve.NewTermQuery("sparse")
	query.SetField("referer")
	request := bleve.NewSearchRequest(query)
	request.Size = 0

	result, err := index.Search(request)
	if err != nil {
		t.Fatalf("search merged index: %v", err)
	}
	if result.Total != 2*sparseCount {
		t.Fatalf("search total = %d, want %d", result.Total, 2*sparseCount)
	}
}
