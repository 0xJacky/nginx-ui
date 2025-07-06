package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var UpdateCertDomains = &gormigrate.Migration{
	ID: "20250706000001",
	Migrate: func(tx *gorm.DB) error {
		// Update domains field in certs table: replace { with [ and } with ]
		if err := tx.Exec("UPDATE certs SET domains = REPLACE(REPLACE(domains, '{', '['), '}', ']')").Error; err != nil {
			return err
		}
		return nil
	},
}
