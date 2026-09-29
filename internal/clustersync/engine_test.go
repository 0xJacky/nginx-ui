package clustersync

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRunSkipsRemainingItemsAfterBlockingFailure(t *testing.T) {
	secondCalled := false
	items := []item{
		{
			kind:     KindConfig,
			name:     "managed configurations (2)",
			blocking: true,
			push: func(context.Context, nodeRef) error {
				return errors.New("batch rejected")
			},
		},
		{
			kind: KindSite,
			name: "dependent.example",
			push: func(context.Context, nodeRef) error {
				secondCalled = true
				return nil
			},
		},
	}

	summary := run(context.Background(), []nodeRef{{id: 1, name: "remote"}}, items)

	if secondCalled {
		t.Fatal("a site must not be applied after its prerequisite batch failed")
	}
	if summary.Total != 2 || summary.Failed != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if !strings.Contains(summary.Results[1].Error, "skipped after prerequisite failed") {
		t.Fatalf("expected the dependent item to report why it was skipped, got %+v", summary.Results[1])
	}
}

func TestBatchOutcomeReportsExistingFilesTheNodeKept(t *testing.T) {
	got := batchOutcome([]byte(`{"written":1,"skipped":2,"skipped_paths":["ssl/a.pem","ssl/a.key"]}`))
	if got.skippedExisting != 2 || len(got.skippedPaths) != 2 {
		t.Fatalf("unexpected outcome %+v", got)
	}

	// A node predating skipped_paths only reports the count.
	got = batchOutcome([]byte(`{"written":0,"skipped":3}`))
	if got.skippedExisting != 3 || len(got.skippedPaths) != 0 {
		t.Fatalf("unexpected legacy outcome %+v", got)
	}
}

func TestRunCarriesTheOutcomeIntoTheResult(t *testing.T) {
	items := []item{{
		kind: KindConfig,
		name: "ssl (2)",
		pushOutcome: func(context.Context, nodeRef) (outcome, error) {
			return outcome{skippedExisting: 1, skippedPaths: []string{"ssl/a.pem"}}, nil
		},
	}}

	summary := run(context.Background(), []nodeRef{{id: 1, name: "n1"}}, items)
	if summary.Succeeded != 1 || summary.Results[0].SkippedExisting != 1 || summary.Results[0].SkippedPaths[0] != "ssl/a.pem" {
		t.Fatalf("unexpected summary %+v", summary)
	}
}
