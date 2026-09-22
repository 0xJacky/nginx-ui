package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// legacyCertColumns re-creates the DNS-01 boolean columns that the Cert model
// no longer declares, so the migration has something to read.
type legacyCertColumns struct {
	ID                                uint64 `gorm:"column:id;primaryKey"`
	LegoDisableCNAMESupport           bool   `gorm:"column:lego_disable_cname_support"`
	DisableAuthoritativeNSPropagation bool   `gorm:"column:disable_authoritative_ns_propagation"`
}

func (legacyCertColumns) TableName() string {
	return "certs"
}

func TestBackfillCertChallengeConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:migrate10?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	require.NoError(t, db.AutoMigrate(&legacyCertColumns{}))

	legacy := &model.Cert{
		Name:            "legacy",
		ChallengeMethod: model.CertChallengeMethodDNS01,
		DnsCredentialID: 7,
	}
	require.NoError(t, db.Create(legacy).Error)
	require.NoError(t, db.Exec(
		"UPDATE certs SET lego_disable_cname_support = ?, disable_authoritative_ns_propagation = ? WHERE id = ?",
		true, true, legacy.ID).Error)

	already := &model.Cert{
		Name:            "already",
		ChallengeMethod: model.CertChallengeMethodDNS01,
		ChallengeConfig: map[string]any{"credential_id": "1"},
	}
	require.NoError(t, db.Create(already).Error)
	http := &model.Cert{Name: "http", ChallengeMethod: model.CertChallengeMethodHTTP01, DnsCredentialID: 3}
	require.NoError(t, db.Create(http).Error)

	require.NoError(t, BackfillCertChallengeConfig.Migrate(db))

	var gotLegacy model.Cert
	require.NoError(t, db.First(&gotLegacy, legacy.ID).Error)
	require.Equal(t, "7", gotLegacy.ChallengeConfig["credential_id"])
	require.Equal(t, true, gotLegacy.ChallengeConfig["disable_cname"])
	require.Equal(t, true, gotLegacy.ChallengeConfig["disable_authoritative_ns_propagation"])

	var gotAlready model.Cert
	require.NoError(t, db.First(&gotAlready, already.ID).Error)
	require.Equal(t, map[string]any{"credential_id": "1"}, gotAlready.ChallengeConfig)

	var gotHTTP model.Cert
	require.NoError(t, db.First(&gotHTTP, http.ID).Error)
	require.Empty(t, gotHTTP.ChallengeConfig)
}
