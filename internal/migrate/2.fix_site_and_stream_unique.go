package migrate

import (
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var FixSiteAndStreamPathUnique = &gormigrate.Migration{
	ID: "202505070000001",
	Migrate: func(tx *gorm.DB) error {
		// Check if sites table exists
		if tx.Migrator().HasTable(&model.Site{}) {
			// Find duplicated paths in sites table
			var siteDuplicates []struct {
				Path  string
				Count int
			}

			if err := tx.Model(&model.Site{}).
				Select("path, count(*) as count").
				Group("path").
				Having("count(*) > 1").
				Unscoped().
				Find(&siteDuplicates).Error; err != nil {
				return err
			}

			// For each duplicated path, delete all but the one with max id
			for _, dup := range siteDuplicates {
				if err := tx.Exec(`DELETE FROM sites WHERE path = ? AND id NOT IN 
				(SELECT max(id) FROM sites WHERE path = ?)`, dup.Path, dup.Path).Error; err != nil {
					return err
				}
			}
		}

		// Check if streams table exists
		if tx.Migrator().HasTable(&model.Stream{}) {
			// Find duplicated paths in streams table
			var streamDuplicates []struct {
				Path  string
				Count int
			}

			if err := tx.Model(&model.Stream{}).
				Select("path, count(*) as count").
				Group("path").
				Having("count(*) > 1").
				Unscoped().
				Find(&streamDuplicates).Error; err != nil {
				return err
			}

			// For each duplicated path, delete all but the one with max id
			for _, dup := range streamDuplicates {
				if err := tx.Exec(`DELETE FROM streams WHERE path = ? AND id NOT IN 
					(SELECT max(id) FROM streams WHERE path = ?)`, dup.Path, dup.Path).Error; err != nil {
					return err
				}
			}
		}

		return nil
	},
}
