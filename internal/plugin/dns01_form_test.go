package plugin

import (
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// formManifest is validManifest with a provider that carries a form.
func formManifest() *protocol.Manifest {
	m := validManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{
		Fields: []protocol.DNS01ProviderField{
			{Key: "CF_DNS_API_TOKEN", Label: "API token", Group: "credential", Secret: true},
			{Key: "CF_ZONE_API_TOKEN", Label: "Zone token", Group: "credential", Optional: true},
			{Key: "CF_API_EMAIL", Label: "Account email", Group: "credential"},
			{Key: "CF_API_KEY", Label: "API key", Group: "credential", Secret: true},
			{Key: "CF_TTL", Label: "TXT record TTL", Group: "setting", Default: "120", Unit: "seconds"},
		},
		Methods: []protocol.DNS01ProviderMethod{
			{Name: "API token", Recommended: true, Fields: []string{"CF_DNS_API_TOKEN", "CF_ZONE_API_TOKEN"}},
			{Name: "Global API key", Fields: []string{"CF_API_EMAIL", "CF_API_KEY"}},
		},
	}
	return m
}

func TestValidateManifestAcceptsDNS01Form(t *testing.T) {
	require.NoError(t, ValidateManifest(formManifest()))

	m := formManifest()
	m.DNS01.Providers[0].Form.Methods = nil
	require.NoError(t, ValidateManifest(m), "methods are optional")
}

func TestValidateManifestAcceptsMethodsWithoutFields(t *testing.T) {
	m := formManifest()
	form := m.DNS01.Providers[0].Form
	form.Methods = append(form.Methods, protocol.DNS01ProviderMethod{Name: "Instance role"})
	require.NoError(t, ValidateManifest(m), "a method may list no fields")

	// Methods that differ only in their fixed values are distinct.
	m = formManifest()
	form = m.DNS01.Providers[0].Form
	form.Methods = append(form.Methods,
		protocol.DNS01ProviderMethod{Name: "Instance role", Values: map[string]string{"CF_AUTH_MODE": "role"}},
		protocol.DNS01ProviderMethod{Name: "Environment", Values: map[string]string{"CF_AUTH_MODE": "env"}},
	)
	require.NoError(t, ValidateManifest(m))
}

func TestValidateManifestAcceptsValueForFieldOfAnotherMethod(t *testing.T) {
	m := formManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{
		Fields: []protocol.DNS01ProviderField{
			{Key: "DNSUPDATE_TSIG_KEY", Label: "TSIG key name", Group: "credential"},
			{Key: "DNSUPDATE_TSIG_SECRET", Label: "TSIG secret", Group: "credential", Secret: true},
			{Key: "DNSUPDATE_TSIG_ALGORITHM", Label: "TSIG algorithm", Group: "credential", Optional: true},
			{Key: "DNSUPDATE_KRB5_PRINCIPAL", Label: "Principal", Group: "credential"},
		},
		Methods: []protocol.DNS01ProviderMethod{
			{Name: "TSIG key", Recommended: true, Fields: []string{"DNSUPDATE_TSIG_KEY", "DNSUPDATE_TSIG_SECRET", "DNSUPDATE_TSIG_ALGORITHM"}},
			{Name: "Kerberos password", Fields: []string{"DNSUPDATE_KRB5_PRINCIPAL"}, Values: map[string]string{"DNSUPDATE_TSIG_ALGORITHM": "gss-tsig"}},
			{Name: "Kerberos keytab", Values: map[string]string{"DNSUPDATE_TSIG_ALGORITHM": "gss-tsig"}},
		},
	}
	require.NoError(t, ValidateManifest(m))
}

func TestValidateManifestRequiresDNS01Form(t *testing.T) {
	m := formManifest()
	m.DNS01.Providers[0].Form = nil
	err := ValidateManifest(m)
	require.ErrorIs(t, err, ErrManifestInvalid)
	assert.Contains(t, err.Error(), "form is required")

	// A provider that takes no values declares an empty form.
	m = formManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{Fields: []protocol.DNS01ProviderField{}}
	require.NoError(t, ValidateManifest(m))
}

func TestValidateManifestRejectsBadDNS01Form(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(f *protocol.DNS01ProviderForm)
		want   string
	}{
		{"missing key", func(f *protocol.DNS01ProviderForm) { f.Fields[0].Key = "" }, "key is required"},
		{"duplicate key", func(f *protocol.DNS01ProviderForm) { f.Fields[1].Key = "CF_DNS_API_TOKEN" }, "declared twice"},
		{"unknown group", func(f *protocol.DNS01ProviderForm) { f.Fields[0].Group = "secret" }, "unknown group"},
		{"missing group", func(f *protocol.DNS01ProviderForm) { f.Fields[0].Group = "" }, "unknown group"},
		{"unknown unit", func(f *protocol.DNS01ProviderForm) { f.Fields[4].Unit = "minutes" }, "unknown unit"},
		{"missing label", func(f *protocol.DNS01ProviderForm) { f.Fields[0].Label = "" }, "needs a label"},
		{"single method", func(f *protocol.DNS01ProviderForm) { f.Methods = f.Methods[:1] }, "at least two"},
		{"method field not in fields", func(f *protocol.DNS01ProviderForm) {
			f.Methods[0].Fields = append(f.Methods[0].Fields, "CLOUDFLARE_DNS_API_TOKEN")
		}, "not in fields"},
		{"method field is a setting", func(f *protocol.DNS01ProviderForm) {
			f.Methods[0].Fields = append(f.Methods[0].Fields, "CF_TTL")
		}, "not a credential"},
		{"two recommended methods", func(f *protocol.DNS01ProviderForm) { f.Methods[1].Recommended = true }, "at most one"},
		{"method without name", func(f *protocol.DNS01ProviderForm) { f.Methods[1].Name = "" }, "name is required"},
		{"duplicate method", func(f *protocol.DNS01ProviderForm) { f.Methods[1].Name = "API token" }, "declared twice"},
		{"value key is empty", func(f *protocol.DNS01ProviderForm) {
			f.Methods[1].Values = map[string]string{"": "x"}
		}, "empty key"},
		{"value key is a setting", func(f *protocol.DNS01ProviderForm) {
			f.Methods[1].Values = map[string]string{"CF_TTL": "60"}
		}, "not a credential field"},
		{"value key is a field no method lists", func(f *protocol.DNS01ProviderForm) {
			f.Fields = append(f.Fields, protocol.DNS01ProviderField{Key: "CF_MODE", Label: "Mode", Group: "credential"})
			f.Methods[1].Values = map[string]string{"CF_MODE": "key"}
		}, "no method lists"},
		{"method lists and sets the same key", func(f *protocol.DNS01ProviderForm) {
			f.Methods[1].Values = map[string]string{"CF_API_KEY": "x"}
		}, "both lists and sets"},
		{"same fields and values", func(f *protocol.DNS01ProviderForm) {
			f.Methods[1].Fields = []string{"CF_ZONE_API_TOKEN", "CF_DNS_API_TOKEN"}
		}, "same fields and values"},
		{"two empty methods", func(f *protocol.DNS01ProviderForm) {
			f.Methods[0].Fields = nil
			f.Methods[1].Fields = nil
		}, "same fields and values"},
		{"same values without fields", func(f *protocol.DNS01ProviderForm) {
			f.Methods[0] = protocol.DNS01ProviderMethod{Name: "Role", Values: map[string]string{"CF_MODE": "role"}}
			f.Methods[1] = protocol.DNS01ProviderMethod{Name: "Role again", Values: map[string]string{"CF_MODE": "role"}}
		}, "same fields and values"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := formManifest()
			tc.mutate(m.DNS01.Providers[0].Form)
			err := ValidateManifest(m)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrManifestInvalid)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestLintBadDNS01FormIsError(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{
		Fields: []protocol.DNS01ProviderField{{Key: "MYDNS_API_TOKEN", Label: "API token", Group: "credential"}},
		Methods: []protocol.DNS01ProviderMethod{
			{Name: "Token", Recommended: true, Fields: []string{"MYDNS_API_TOKEN"}},
			{Name: "Key", Recommended: true, Fields: []string{"MYDNS_API_KEY"}},
		},
	}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assert.True(t, report.HasErrors())
	assertHasFinding(t, report, LevelError, "DNS01-18")
}

func TestLintDNS01MethodValueShadowingFieldIsError(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{
		Fields: []protocol.DNS01ProviderField{
			{Key: "MYDNS_API_TOKEN", Label: "API token", Group: "credential"},
			{Key: "MYDNS_TTL", Label: "TTL", Group: "setting"},
		},
		Methods: []protocol.DNS01ProviderMethod{
			{Name: "Token", Recommended: true, Fields: []string{"MYDNS_API_TOKEN"}},
			{Name: "Instance role", Values: map[string]string{"MYDNS_TTL": "60"}},
		},
	}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assertHasFinding(t, report, LevelError, "DNS01-18")
}

func TestLintDNS01MethodWithoutFieldsIsClean(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{
		Fields: []protocol.DNS01ProviderField{{Key: "MYDNS_API_TOKEN", Label: "API token", Group: "credential"}},
		Methods: []protocol.DNS01ProviderMethod{
			{Name: "Token", Recommended: true, Fields: []string{"MYDNS_API_TOKEN"}},
			{Name: "Instance role", Values: map[string]string{"MYDNS_AUTH": "role"}},
		},
	}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	for _, f := range report.Findings {
		assert.NotEqual(t, "DNS01-18", f.Rule, f.Message)
	}
}

func TestLintMissingDNS01FormIsError(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Form = nil
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	assertHasFinding(t, report, LevelError, "DNS01-18")
}

func TestLintGoodDNS01FormHasNoFormFinding(t *testing.T) {
	m := goodManifest()
	m.DNS01.Providers[0].Form = &protocol.DNS01ProviderForm{
		Fields: []protocol.DNS01ProviderField{{Key: "MYDNS_API_TOKEN", Label: "API token", Group: "credential", Secret: true}},
	}
	dir := writeLintFixture(t, lintFixture{manifest: m})
	report, err := Lint(dir)
	require.NoError(t, err)
	for _, finding := range report.Findings {
		assert.NotEqual(t, "DNS01-18", finding.Rule, finding.Message)
	}
}
