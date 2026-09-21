package migrate

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillCertChallengeConfig(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Cert{}))

	legacy := &model.Cert{
		Name:                              "legacy",
		ChallengeMethod:                   model.CertChallengeMethodDNS01,
		DnsCredentialID:                   7,
		LegoDisableCNAMESupport:           true,
		DisableAuthoritativeNSPropagation: true,
	}
	require.NoError(t, db.Create(legacy).Error)
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
