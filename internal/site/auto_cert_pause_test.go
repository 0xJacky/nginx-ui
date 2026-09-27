package site

import (
	"os"
	"testing"

	"github.com/0xJacky/Nginx-UI/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func autoCertStates(t *testing.T) map[string]int {
	t.Helper()
	var certs []model.Cert
	require.NoError(t, model.UseDB().Order("id").Find(&certs).Error)
	states := make(map[string]int, len(certs))
	for _, c := range certs {
		states[c.Name] = c.AutoCert
	}
	return states
}

func TestDisablePausesAndEnableResumesAutoCert(t *testing.T) {
	f := setupOnboardingTest(t, "server {\n    listen 80;\n    server_name example.com;\n}\n")
	db := model.UseDB()
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	require.NoError(t, db.Create([]*model.Cert{
		{Name: "renewing", Filename: onboardingSiteName, AutoCert: model.AutoCertEnabled},
		{Name: "switched-off", Filename: onboardingSiteName, AutoCert: model.AutoCertDisabled},
		{Name: "other-site", Filename: "other.example.com", AutoCert: model.AutoCertEnabled},
	}).Error)

	require.NoError(t, os.Symlink(f.availablePath, f.enabledPath))
	require.NoError(t, Disable(onboardingSiteName))

	// The records stay, so the certificates remain reusable; only renewal pauses.
	assert.Equal(t, map[string]int{
		"renewing":     model.AutoCertPaused,
		"switched-off": model.AutoCertDisabled,
		"other-site":   model.AutoCertEnabled,
	}, autoCertStates(t))

	require.NoError(t, Enable(onboardingSiteName))

	// A renewal the user switched off stays off.
	assert.Equal(t, map[string]int{
		"renewing":     model.AutoCertEnabled,
		"switched-off": model.AutoCertDisabled,
		"other-site":   model.AutoCertEnabled,
	}, autoCertStates(t))
}

func TestPausedAutoCertIsNotRenewed(t *testing.T) {
	f := setupOnboardingTest(t, "server {\n    listen 80;\n    server_name example.com;\n}\n")
	db := model.UseDB()
	require.NoError(t, db.AutoMigrate(&model.Cert{}))
	require.NoError(t, db.Create(&model.Cert{
		Name: "dns", Filename: onboardingSiteName, AutoCert: model.AutoCertEnabled,
		ChallengeMethod: model.CertChallengeMethodDNS01,
	}).Error)
	require.NoError(t, os.Symlink(f.availablePath, f.enabledPath))
	require.Len(t, model.GetAutoCertList(), 1)

	require.NoError(t, Disable(onboardingSiteName))

	// DNS-01 certificates renew regardless of the site's symlink, so the paused
	// state is what keeps a disabled site's certificate from renewing.
	assert.Empty(t, model.GetAutoCertList())
}
