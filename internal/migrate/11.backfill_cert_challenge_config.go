package migrate

import (
	"strconv"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// BackfillCertChallengeConfig copies the legacy DNS-01 columns into the
// challenge_config JSON column so the dns01 plugin sees the same options.
var BackfillCertChallengeConfig = &gormigrate.Migration{
	ID: "20260922000002",
	Migrate: func(tx *gorm.DB) error {
		if !tx.Migrator().HasColumn(&model.Cert{}, "ChallengeConfig") {
			if err := tx.Migrator().AddColumn(&model.Cert{}, "ChallengeConfig"); err != nil {
				return err
			}
		}

		var certs []*model.Cert
		if err := tx.Where("challenge_method = ?", model.CertChallengeMethodDNS01).Find(&certs).Error; err != nil {
			return err
		}

		for _, c := range certs {
			if len(c.ChallengeConfig) > 0 {
				continue
			}
			cfg := map[string]any{}
			if c.DnsCredentialID != 0 {
				cfg["credential_id"] = strconv.FormatUint(c.DnsCredentialID, 10)
			}
			if c.LegoDisableCNAMESupport {
				cfg["disable_cname"] = true
			}
			if c.DisableAuthoritativeNSPropagation {
				cfg["disable_authoritative_ns_propagation"] = true
			}
			if len(cfg) == 0 {
				continue
			}
			if err := tx.Model(&model.Cert{}).Where("id = ?", c.ID).
				Select("challenge_config").
				Updates(&model.Cert{ChallengeConfig: cfg}).Error; err != nil {
				return err
			}
		}
		return nil
	},
}
