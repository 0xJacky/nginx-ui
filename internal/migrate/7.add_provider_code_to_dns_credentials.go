package migrate

import (
	"encoding/json"
	"strings"

	"github.com/0xJacky/Nginx-UI/internal/cert/dns"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var AddProviderCodeToDnsCredentials = &gormigrate.Migration{
	ID: "20251209000002",
	Migrate: func(tx *gorm.DB) error {
		if !tx.Migrator().HasColumn(&model.DnsCredential{}, "ProviderCode") {
			if err := tx.Migrator().AddColumn(&model.DnsCredential{}, "ProviderCode"); err != nil {
				return err
			}
		}
		if !tx.Migrator().HasIndex(&model.DnsCredential{}, "ProviderCode") {
			if err := tx.Migrator().CreateIndex(&model.DnsCredential{}, "ProviderCode"); err != nil {
				return err
			}
		}

		// Backfill provider_code from config.code (preferred) or provider/name fallback.
		type credentialRow struct {
			ID       uint64         `gorm:"column:id"`
			Config   datatypes.JSON `gorm:"column:config"`
			Provider string         `gorm:"column:provider"`
			Name     string         `gorm:"column:name"`
		}

		var rows []credentialRow
		if err := tx.Table("dns_credentials").Select("id, config, provider, name").Find(&rows).Error; err != nil {
			return err
		}

		for _, row := range rows {
			providerCode := normalizeProviderCode(row.Provider)

			if len(row.Config) > 0 {
				var cfg dns.Config
				if err := json.Unmarshal(row.Config, &cfg); err == nil && strings.TrimSpace(cfg.Code) != "" {
					providerCode = normalizeProviderCode(cfg.Code)
				}
			}

			if providerCode == "" {
				providerCode = normalizeProviderCode(row.Name)
			}

			if err := tx.Table("dns_credentials").
				Where("id = ?", row.ID).
				Update("provider_code", providerCode).Error; err != nil {
				return err
			}
		}

		return nil
	},
}

func normalizeProviderCode(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
