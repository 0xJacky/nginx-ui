package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
)

var Migrations = []*gormigrate.Migration{
	SiteCategoryToEnvGroup,
	RenameAuthsToUsers,
}

var BeforeAutoMigrate = []*gormigrate.Migration{
	FixSiteAndStreamPathUnique,
}
