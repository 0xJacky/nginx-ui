package testdb

import (
	"strings"
	"testing"
)

func TestDSNNamesANewDatabaseOnEveryCall(t *testing.T) {
	first, second := DSN(t), DSN(t)
	if first == second {
		t.Fatalf("two calls returned the same database %q", first)
	}
	if !strings.HasPrefix(first, "file:TestDSNNamesANewDatabaseOnEveryCall-") || !strings.HasSuffix(first, "?mode=memory&cache=shared") {
		t.Fatalf("unexpected name %q", first)
	}
	if named := DSN(t, "backup"); !strings.Contains(named, "-backup-") {
		t.Fatalf("the suffix is missing from %q", named)
	}
}
