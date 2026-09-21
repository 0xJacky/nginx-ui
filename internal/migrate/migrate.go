package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
)

var Migrations = []*gormigrate.Migration{
	SiteCategoryToNamespace,
	UpdateCertDomains,
	RenameEnvGroupsToNamespaces,
	RenameEnvironmentsToNodes,
	AddProviderCodeToDnsCredentials,
	EncryptSensitiveJSONFields,
	DropLegacyRenamedTableIndexes,
	RepairCertDomainsJSON,
	CollapseBanIPRows,
	BackfillCertChallengeConfig,
	BackfillNodeAcceptPluginSync,
}

var BeforeAutoMigrate = []*gormigrate.Migration{
	FixSiteAndStreamPathUnique,
	RenameAuthsToUsers,
}
