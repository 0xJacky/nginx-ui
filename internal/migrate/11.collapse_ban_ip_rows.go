package migrate

import (
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// CollapseBanIPRows leaves at most one ban_ips row per client. Before the
// login ban re-armed correctly, every failure after a client's first window
// had ended added another row, so a busy client could own hundreds of rows.
// Ended windows are dropped; the active rows of one client become a single
// row with the highest attempt count and the latest expiry, which never
// shortens a ban that was in force.
var CollapseBanIPRows = &gormigrate.Migration{
	ID: "20261029000001",
	Migrate: func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("ban_ips") {
			return nil
		}

		if err := tx.Exec("DELETE FROM ban_ips WHERE expired_at < ?", time.Now().Unix()).Error; err != nil {
			return err
		}

		var duplicates []struct {
			IP        string
			Attempts  int
			ExpiredAt int64
		}
		if err := tx.Raw(`SELECT ip, MAX(attempts) AS attempts, MAX(expired_at) AS expired_at
			FROM ban_ips GROUP BY ip HAVING COUNT(*) > 1`).Scan(&duplicates).Error; err != nil {
			return err
		}

		for _, dup := range duplicates {
			if err := tx.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("DELETE FROM ban_ips WHERE ip = ?", dup.IP).Error; err != nil {
					return err
				}
				return tx.Exec("INSERT INTO ban_ips (ip, attempts, expired_at) VALUES (?, ?, ?)",
					dup.IP, dup.Attempts, dup.ExpiredAt).Error
			}); err != nil {
				return err
			}
		}
		return nil
	},
}
