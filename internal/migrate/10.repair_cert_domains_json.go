package migrate

import (
	"encoding/json"
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// RepairCertDomainsJSON rewrites certs.domains values that are not valid JSON.
// Re-importing an existing certificate used to update domains through a map,
// bypassing the JSON serializer and storing plain text such as
// "example.com", which then made every query loading certs fail.
var RepairCertDomainsJSON = &gormigrate.Migration{
	ID: "20260922000001",
	Migrate: func(tx *gorm.DB) error {
		if !tx.Migrator().HasTable("certs") {
			return nil
		}

		var rows []struct {
			ID      uint64
			Domains string
		}
		if err := tx.Raw("SELECT id, domains FROM certs WHERE domains IS NOT NULL AND domains <> ''").
			Scan(&rows).Error; err != nil {
			return err
		}

		for _, row := range rows {
			if json.Valid([]byte(row.Domains)) {
				continue
			}
			repaired, err := json.Marshal(splitPlainDomains(row.Domains))
			if err != nil {
				return err
			}
			if err := tx.Exec("UPDATE certs SET domains = ? WHERE id = ?", string(repaired), row.ID).Error; err != nil {
				return err
			}
		}
		return nil
	},
}

// splitPlainDomains parses a plain-text domains value. Domain names cannot
// contain commas or whitespace, so both are treated as separators.
func splitPlainDomains(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}
