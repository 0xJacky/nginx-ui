package migrate

import (
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
)

// Every migration id must be unique across both lists, or gormigrate refuses
// to start the application.
func TestMigrationIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, list := range [][]*gormigrate.Migration{BeforeAutoMigrate, Migrations} {
		for _, m := range list {
			if seen[m.ID] {
				t.Fatalf("migration id %q is used more than once", m.ID)
			}
			seen[m.ID] = true
		}
	}
}
