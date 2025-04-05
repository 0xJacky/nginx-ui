package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var SiteCategoryToEnvGroup = &gormigrate.Migration{
	ID: "20250405000001",
	Migrate: func(tx *gorm.DB) error {
		// Step 1: Create new env_groups table
		if err := tx.Migrator().AutoMigrate(&model.EnvGroup{}); err != nil {
			return err
		}

		// Step 2: Copy data from site_categories to env_groups
		if tx.Migrator().HasTable("site_categories") {
			var siteCategories []map[string]interface{}
			if err := tx.Table("site_categories").Find(&siteCategories).Error; err != nil {
				return err
			}

			for _, sc := range siteCategories {
				if err := tx.Table("env_groups").Create(sc).Error; err != nil {
					return err
				}
			}

			// Step 3: Update sites table to use env_group_id instead of site_category_id
			if tx.Migrator().HasColumn("sites", "site_category_id") {
				// First add the new column if it doesn't exist
				if !tx.Migrator().HasColumn("sites", "env_group_id") {
					if err := tx.Exec("ALTER TABLE sites ADD COLUMN env_group_id bigint").Error; err != nil {
						return err
					}
				}

				// Copy the values from site_category_id to env_group_id
				if err := tx.Exec("UPDATE sites SET env_group_id = site_category_id").Error; err != nil {
					return err
				}
			}
		}
		return nil
	},
}
