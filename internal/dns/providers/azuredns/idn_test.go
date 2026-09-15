package azuredns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xJacky/Nginx-UI/internal/dns"
)

// Azure reports a zone by the spelling it was created with, so names only compare
// reliably once both sides are reduced to punycode.
func TestNormalizeZoneNameCanonicalizesIDN(t *testing.T) {
	t.Parallel()

	require.Equal(t, "xn--fsq.example.com", normalizeZoneName("例.example.com"))
	require.Equal(t, "xn--fsq.example.com", normalizeZoneName("xn--fsq.example.com"))
	require.Equal(t, "xn--fsq.example.com", normalizeZoneName(" 例.EXAMPLE.com. "))
	require.Equal(t, "example.com", normalizeZoneName("Example.com."))
	// Unchanged for a label IDNA2008 rejects but this project accepts.
	require.Equal(t, "ab--cd.example.com", normalizeZoneName("ab--cd.example.com"))
}

func TestSelectZoneMatchesAcrossSpellings(t *testing.T) {
	t.Parallel()

	zones := []zoneRef{
		{ResourceGroup: "rg", Name: normalizeZoneName("例.example.com")},
		{ResourceGroup: "rg", Name: "example.com"},
	}

	// A zone stored either way must resolve, and the most specific one still wins.
	for _, domain := range []string{"例.example.com", "xn--fsq.example.com", "www.例.example.com"} {
		zone, ok := selectZone(domain, zones)
		require.True(t, ok, "domain %q should resolve", domain)
		require.Equal(t, "xn--fsq.example.com", zone.Name)
	}

	zone, ok := selectZone("www.example.com", zones)
	require.True(t, ok)
	require.Equal(t, "example.com", zone.Name)
}

func TestRelativeNameStripsPunycodeZone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		record string
		zone   string
		want   string
	}{
		{"unicode record, punycode zone", "www.例.example.com", "xn--fsq.example.com", "www"},
		{"punycode record, unicode zone", "www.xn--fsq.example.com", "例.example.com", "www"},
		{"apex in unicode", "例.example.com", "xn--fsq.example.com", "@"},
		{"unicode label under idn zone", "例.例.example.com", "xn--fsq.example.com", "xn--fsq"},
		{"wildcard survives", "*.例.example.com", "xn--fsq.example.com", "*"},
		{"acme label survives", "_acme-challenge.例.example.com", "xn--fsq.example.com", "_acme-challenge"},
		{"ascii unchanged", "www.example.com", "example.com", "www"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := relativeName(tt.record, tt.zone)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeRelativeNameEncodesIDNAndStillRejectsBadInput(t *testing.T) {
	t.Parallel()

	got, err := normalizeRelativeName("例")
	require.NoError(t, err)
	require.Equal(t, "xn--fsq", got)

	// Characters that would break the record ID encoding stay rejected.
	for _, bad := range []string{"a/b", "a#b", "a?b", "a%b"} {
		_, err := normalizeRelativeName(bad)
		require.Error(t, err, "name %q must stay rejected", bad)
	}
}

func TestMatchesFilterAcceptsUnicodeNameTerm(t *testing.T) {
	t.Parallel()

	record := dns.Record{Type: "A", Name: "xn--fsq"}

	// The UI shows the decoded label, so filtering by it has to match.
	require.True(t, matchesFilter(record, dns.RecordFilter{Name: "例"}))
	require.True(t, matchesFilter(record, dns.RecordFilter{Name: "xn--fsq"}))
	require.True(t, matchesFilter(record, dns.RecordFilter{Name: "例", Type: "A"}))
	require.False(t, matchesFilter(record, dns.RecordFilter{Name: "例", Type: "AAAA"}))
	require.False(t, matchesFilter(record, dns.RecordFilter{Name: "other"}))

	// An ASCII record still filters the way it did before.
	require.True(t, matchesFilter(dns.Record{Type: "A", Name: "www"}, dns.RecordFilter{Name: "ww"}))
}
