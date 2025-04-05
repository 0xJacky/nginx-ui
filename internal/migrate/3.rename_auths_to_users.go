package migrate

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var RenameAuthsToUsers = &gormigrate.Migration{
	ID: "20250405000002",
	Migrate: func(tx *gorm.DB) error {
		// Check if both tables exist
		hasAuthsTable := tx.Migrator().HasTable("auths")
		hasUsersTable := tx.Migrator().HasTable("users")

		if hasAuthsTable {
			if hasUsersTable {
				// Both tables exist - we need to check if users table is empty
				var count int64
				if err := tx.Table("users").Count(&count).Error; err != nil {
					return err
				}

				if count > 0 {
					// Users table has data - drop auths table as users table is now the source of truth
					return tx.Migrator().DropTable("auths")
				} else {
					// Users table is empty - drop it and rename auths to users
					return tx.Transaction(func(ttx *gorm.DB) error {
						if err := ttx.Migrator().DropTable("users"); err != nil {
							return err
						}
						return ttx.Migrator().RenameTable("auths", "users")
					})
				}
			} else {
				// Only auths table exists - simply rename it
				return tx.Migrator().RenameTable("auths", "users")
			}
		}

		// If auths table doesn't exist, nothing to do
		return nil
	},
}
