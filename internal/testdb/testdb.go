// Package testdb names the in-memory SQLite databases of the tests.
package testdb

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

// opened numbers the databases of the process.
var opened atomic.Int64

// DSN returns a shared cache in-memory SQLite database of its own for t. A
// shared cache database lives as long as a connection to it does, so a name
// used again in the same process, by -count or a rerun, would hand over the
// rows of the earlier run. Every call returns a new database; open the
// returned name more than once to share one database within a test. suffix
// tells apart several databases of one test in error messages.
func DSN(t testing.TB, suffix ...string) string {
	t.Helper()
	name := t.Name()
	if len(suffix) > 0 {
		name += "-" + strings.Join(suffix, "-")
	}
	return fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", name, opened.Add(1))
}
