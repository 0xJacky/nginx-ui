package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
)

var Migrations = []*gormigrate.Migration{
	SiteCategoryToNamespace,
	RenameAuthsToUsers,
	UpdateCertDomains,
	RenameEnvGroupsToNamespaces,
	RenameEnvironmentsToNodes,
	AddProviderCodeToDnsCredentials,
}

var BeforeAutoMigrate = []*gormigrate.Migration{
	FixSiteAndStreamPathUnique,
}
