package dns_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

func TestToASCIIName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unicode idn", "例.example.com", "xn--fsq.example.com"},
		{"punycode is idempotent", "xn--fsq.example.com", "xn--fsq.example.com"},
		{"idn tld", "例.中国", "xn--fsq.xn--fiqs8s"},
		{"latin diacritics", "münchen.de", "xn--mnchen-3ya.de"},
		{"lowercased", "EXAMPLE.com", "example.com"},
		{"trailing dot trimmed", "例.example.com.", "xn--fsq.example.com"},
		{"surrounding space trimmed", " 例.example.com ", "xn--fsq.example.com"},
		{"subdomain of idn zone", "www.例.example.com", "www.xn--fsq.example.com"},
		{"ideographic full stop separator", "例。example.com。", "xn--fsq.example.com"},
		{"fullwidth full stop separator", "例．example.com", "xn--fsq.example.com"},
		{"wildcard label preserved", "*.例.example.com", "*.xn--fsq.example.com"},
		{"underscore label preserved", "_acme-challenge.例.example.com", "_acme-challenge.xn--fsq.example.com"},
		{"apex marker preserved", "@", "@"},
		{"double hyphen label preserved", "ab--cd.example.com", "ab--cd.example.com"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, dns.ToASCIIName(tt.input))
		})
	}
}

func TestToUnicodeName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "例.example.com", dns.ToUnicodeName("xn--fsq.example.com"))
	assert.Equal(t, "例.中国", dns.ToUnicodeName("xn--fsq.xn--fiqs8s"))
	assert.Equal(t, "例.example.com", dns.ToUnicodeName("例.example.com"))
	assert.Equal(t, "example.com", dns.ToUnicodeName("example.com"))
	assert.Equal(t, "", dns.ToUnicodeName(""))
}

func TestNormalizeDomainAcceptsInternationalizedDomains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unicode idn is stored as punycode", "例.example.com", "xn--fsq.example.com"},
		{"punycode is accepted unchanged", "xn--fsq.example.com", "xn--fsq.example.com"},
		{"idn tld", "例.中国", "xn--fsq.xn--fiqs8s"},
		{"latin diacritics", "münchen.de", "xn--mnchen-3ya.de"},
		{"idn subdomain", "www.例.example.com", "www.xn--fsq.example.com"},
		{"ascii domain is unchanged", "example.com", "example.com"},
		{"ascii domain is lowercased", "EXAMPLE.COM", "example.com"},
		{"double hyphen label still accepted", "ab--cd.example.com", "ab--cd.example.com"},
		// A CJK input method emits U+3002 / U+FF0E instead of an ASCII dot, which
		// IDNA mapping folds into a label separator.
		{"ideographic full stop separator", "例。Example.com。", "xn--fsq.example.com"},
		{"fullwidth full stop separator", "例．example.com", "xn--fsq.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := dns.NormalizeDomain(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeDomainCollapsesBothSpellingsToOneKey(t *testing.T) {
	t.Parallel()

	// Both spellings of one domain must normalize to the same value, otherwise the
	// per-credential uniqueness index would accept the same zone twice.
	unicodeForm, err := dns.NormalizeDomain("例.example.com")
	require.NoError(t, err)
	punycodeForm, err := dns.NormalizeDomain("xn--fsq.example.com")
	require.NoError(t, err)
	assert.Equal(t, unicodeForm, punycodeForm)
}

func TestNormalizeDomainRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace inside label", "exa mple.com"},
		{"leading hyphen", "-bad.example.com"},
		{"wildcard is not a domain", "*.例.example.com"},
		{"underscore is not a domain", "_acme-challenge.例.example.com"},
		{"no tld", "例"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := dns.NormalizeDomain(tt.input)
			assert.Error(t, err)
		})
	}
}
