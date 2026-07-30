package clustersync

import (
	"errors"
	"testing"
)

func TestScopeIsEmpty(t *testing.T) {
	if !(Scope{}).IsEmpty() {
		t.Fatal("an unset scope must be empty")
	}
	if (Scope{Sites: true}).IsEmpty() {
		t.Fatal("a scope selecting sites is not empty")
	}
	if (Scope{Overwrite: true}).IsEmpty() != true {
		t.Fatal("overwrite alone selects no content")
	}
}

func TestFullScopeCoversEverything(t *testing.T) {
	scope := FullScope()
	if !scope.Configs || !scope.Sites || !scope.Streams || !scope.Overwrite {
		t.Fatalf("full scope must cover everything, got %+v", scope)
	}
}

func TestSummaryCountsAndSorts(t *testing.T) {
	results := &collector{}
	nodeB := nodeRef{id: 2, name: "beta"}
	nodeA := nodeRef{id: 1, name: "alpha"}

	results.ok(nodeB, KindSite, "b.conf")
	results.fail(nodeA, KindSite, "a.conf", errors.New("boom"))
	results.ok(nodeA, KindConfig, "conf.d (2)")

	summary := results.summary()

	if summary.Total != 3 || summary.Succeeded != 2 || summary.Failed != 1 {
		t.Fatalf("unexpected counts: %+v", summary)
	}

	if summary.Results[0].Node != "alpha" || summary.Results[0].Kind != KindConfig {
		t.Fatalf("results must be sorted by node then kind, got %+v", summary.Results)
	}
	if summary.Results[1].Name != "a.conf" || summary.Results[1].Error != "boom" {
		t.Fatalf("failure detail lost: %+v", summary.Results[1])
	}
	if summary.Results[2].Node != "beta" {
		t.Fatalf("expected beta last, got %+v", summary.Results[2])
	}
}

func TestSummaryOfEmptyRunIsNotNil(t *testing.T) {
	summary := (&collector{}).summary()
	if summary.Results == nil {
		t.Fatal("results must serialize as an empty array")
	}
	if summary.Total != 0 {
		t.Fatalf("expected an empty summary, got %+v", summary)
	}
}
