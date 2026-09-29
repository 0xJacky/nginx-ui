package plugin

import (
	"context"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/model"
	"github.com/0xJacky/Nginx-UI/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerUsageListsRenewingDNS01Certificates(t *testing.T) {
	m := newCertAwareManager(t)
	ctx := context.Background()

	_, err := m.Install(ctx, buildTestPackage(t, pluginManifest("official.alpha"), nil), InstallOptions{})
	require.NoError(t, err)
	_, err = m.Install(ctx, buildTestPackage(t, marketplaceManifest("official.webapp", "1.0.0"), nil), InstallOptions{})
	require.NoError(t, err)

	credentials := []*model.DnsCredential{
		{Name: "ours", ProviderCode: "test"},
		{Name: "other", ProviderCode: "elsewhere"},
	}
	for _, credential := range credentials {
		require.NoError(t, query.DnsCredential.WithContext(ctx).Create(credential))
	}

	certs := []*model.Cert{
		{Name: "renews", ChallengeMethod: model.CertChallengeMethodDNS01, AutoCert: model.AutoCertEnabled, DnsCredentialID: credentials[0].ID},
		{Domains: []string{"a.example", "b.example"}, ChallengeMethod: model.CertChallengeMethodDNS01, AutoCert: model.AutoCertEnabled},
		{Name: "manual", ChallengeMethod: model.CertChallengeMethodDNS01, AutoCert: model.AutoCertDisabled},
		{Name: "http", ChallengeMethod: model.CertChallengeMethodHTTP01, AutoCert: model.AutoCertEnabled},
		{Name: "other provider", ChallengeMethod: model.CertChallengeMethodDNS01, AutoCert: model.AutoCertEnabled, DnsCredentialID: credentials[1].ID},
	}
	for _, cert := range certs {
		require.NoError(t, query.Cert.WithContext(ctx).Create(cert))
	}

	usage, err := m.Usage(ctx, "official.alpha")
	require.NoError(t, err)
	assert.Equal(t, 2, usage.Total)
	require.Len(t, usage.Items, 2)
	assert.Equal(t, UsageItem{Kind: UsageKindCertificate, ID: "1", Name: "renews"}, usage.Items[0])
	assert.Equal(t, "a.example, b.example", usage.Items[1].Name)

	// A plugin without the capability has nothing depending on it.
	usage, err = m.Usage(ctx, "official.webapp")
	require.NoError(t, err)
	assert.Zero(t, usage.Total)
	assert.Empty(t, usage.Items)

	_, err = m.Usage(ctx, "official.missing")
	assertPluginError(t, err, ErrPluginNotFound)
}

func TestUsageAddCapsItemsButCountsAll(t *testing.T) {
	usage := &Usage{Items: []UsageItem{}}
	items := make([]UsageItem, maxUsageItems+5)
	usage.add(items...)
	usage.add(UsageItem{Kind: UsageKindCertificate})

	assert.Equal(t, maxUsageItems+6, usage.Total)
	assert.Len(t, usage.Items, maxUsageItems)
}

func TestSettingsListFieldIsValidatedAndStored(t *testing.T) {
	schema := &protocol.SettingsSchema{
		Settings: []protocol.SettingsField{
			{Key: "servers", Type: settingsTypeList, DisplayName: "Servers", Required: true},
			{Key: "extra", Type: settingsTypeList, DisplayName: "Extra"},
		},
	}

	next, err := normalizeSettings(schema, map[string]any{
		"servers": []any{" 1.1.1.1:53 ", "", "8.8.8.8:53"},
		"extra":   nil,
	}, nil)
	require.NoError(t, err)
	assert.Equal(t, []any{"1.1.1.1:53", "8.8.8.8:53"}, next["servers"])
	assert.Equal(t, []any{}, next["extra"])

	// Anything but an array of strings is refused.
	for _, bad := range []any{"1.1.1.1", []any{"a", 1.0}, map[string]any{}} {
		_, err = normalizeSettings(schema, map[string]any{"servers": bad}, nil)
		assertPluginError(t, err, ErrSettingsInvalid)
	}

	// A required list needs at least one item.
	_, err = normalizeSettings(schema, map[string]any{"servers": []any{" "}}, nil)
	assertPluginError(t, err, ErrSettingsInvalid)
}

func TestValidateSettingsSchemaListDefault(t *testing.T) {
	valid := &protocol.SettingsSchema{Settings: []protocol.SettingsField{
		{Key: "servers", Type: settingsTypeList, DisplayName: "Servers", Default: []any{"1.1.1.1:53"}},
	}}
	require.NoError(t, validateSettingsSchema(valid))

	invalid := &protocol.SettingsSchema{Settings: []protocol.SettingsField{
		{Key: "servers", Type: settingsTypeList, DisplayName: "Servers", Default: "1.1.1.1:53"},
	}}
	assertPluginError(t, validateSettingsSchema(invalid), ErrManifestInvalid)

	mixed := &protocol.SettingsSchema{Settings: []protocol.SettingsField{
		{Key: "servers", Type: settingsTypeList, DisplayName: "Servers", Default: []any{"a", true}},
	}}
	assertPluginError(t, validateSettingsSchema(mixed), ErrManifestInvalid)
}

func TestLintSettingsSchemaListDefault(t *testing.T) {
	m := goodManifest()
	m.SettingsSchema = &protocol.SettingsSchema{
		Settings: []protocol.SettingsField{{Key: "servers", Type: settingsTypeList, DisplayName: "Servers", Default: "x"}},
	}
	report := &LintReport{}
	lintSettingsSchema(m.SettingsSchema, report)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "MAN-28")

	ok := &LintReport{}
	lintSettingsSchema(&protocol.SettingsSchema{
		Settings: []protocol.SettingsField{{Key: "servers", Type: settingsTypeList, DisplayName: "Servers", Default: []any{"x"}}},
	}, ok)
	assert.Empty(t, ok.Findings)
}
