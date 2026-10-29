package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddProbeKindToSiteConfigs adds the optional probe kind columns of the site
// health check. Existing rows keep NULL, which selects the built-in check.
var AddProbeKindToSiteConfigs = &gormigrate.Migration{
	ID: "20260923000001",
	Migrate: func(tx *gorm.DB) error {
		for _, column := range []string{"ProbeKind", "ProbeConfig"} {
			if tx.Migrator().HasColumn(&model.SiteConfig{}, column) {
				continue
			}
			if err := tx.Migrator().AddColumn(&model.SiteConfig{}, column); err != nil {
				return err
			}
		}
		return nil
	},
}
