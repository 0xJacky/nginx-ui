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
