package migrate

import (
	"encoding/json"
	"strconv"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// BackfillCertChallengeConfig copies the legacy DNS-01 columns into the
// challenge_config JSON column so the dns01 plugin sees the same options.
// The legacy columns are no longer part of the model, so they are read
// through a local row struct.
var BackfillCertChallengeConfig = &gormigrate.Migration{
	ID: "20260922000002",
	Migrate: func(tx *gorm.DB) error {
		if !tx.Migrator().HasColumn(&model.Cert{}, "ChallengeConfig") {
			if err := tx.Migrator().AddColumn(&model.Cert{}, "ChallengeConfig"); err != nil {
				return err
			}
		}

		// Databases created after the legacy columns were dropped from the
		// model have nothing to backfill.
		if !tx.Migrator().HasColumn("certs", "lego_disable_cname_support") ||
			!tx.Migrator().HasColumn("certs", "disable_authoritative_ns_propagation") {
			return nil
		}

		type certRow struct {
			ID                                uint64         `gorm:"column:id"`
			DnsCredentialID                   uint64         `gorm:"column:dns_credential_id"`
			ChallengeConfig                   datatypes.JSON `gorm:"column:challenge_config"`
			LegoDisableCNAMESupport           bool           `gorm:"column:lego_disable_cname_support"`
			DisableAuthoritativeNSPropagation bool           `gorm:"column:disable_authoritative_ns_propagation"`
		}

		var rows []certRow
		if err := tx.Table("certs").
			Select("id, dns_credential_id, challenge_config, "+
				"lego_disable_cname_support, disable_authoritative_ns_propagation").
			Where("challenge_method = ?", model.CertChallengeMethodDNS01).
			Find(&rows).Error; err != nil {
			return err
		}

		for _, row := range rows {
			var existing map[string]any
			if len(row.ChallengeConfig) > 0 {
				_ = json.Unmarshal(row.ChallengeConfig, &existing)
			}
			if len(existing) > 0 {
				continue
			}

			cfg := map[string]any{}
			if row.DnsCredentialID != 0 {
				cfg["credential_id"] = strconv.FormatUint(row.DnsCredentialID, 10)
			}
			if row.LegoDisableCNAMESupport {
				cfg["disable_cname"] = true
			}
			if row.DisableAuthoritativeNSPropagation {
				cfg["disable_authoritative_ns_propagation"] = true
			}
			if len(cfg) == 0 {
				continue
			}

			if err := tx.Model(&model.Cert{}).Where("id = ?", row.ID).
				Select("challenge_config").
				Updates(&model.Cert{ChallengeConfig: cfg}).Error; err != nil {
				return err
			}
		}
		return nil
	},
}
