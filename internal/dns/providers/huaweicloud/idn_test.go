package huaweicloud

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Huawei reports a zone by the spelling it was created with, so names only compare
// reliably once both sides are reduced to punycode.
func TestNormalizeZoneNameCanonicalizesIDN(t *testing.T) {
	t.Parallel()

	require.Equal(t, "xn--fsq.example.com", normalizeZoneName("例.example.com"))
	require.Equal(t, "xn--fsq.example.com", normalizeZoneName("xn--fsq.example.com"))
	require.Equal(t, "xn--fsq.example.com", normalizeZoneName("例.example.com."))
	require.Equal(t, "xn--fsq.example.com", normalizeZoneName(" 例.EXAMPLE.com "))
	require.Equal(t, "example.com", normalizeZoneName("Example.com."))
	// Unchanged for a label IDNA2008 rejects but this project accepts.
	require.Equal(t, "ab--cd.example.com", normalizeZoneName("ab--cd.example.com"))

	require.Equal(t, normalizeZoneName("例.example.com"), normalizeZoneName("xn--fsq.example.com"),
		"both spellings must resolve to the same zone")
}

func TestRecordFQDNBuildsPunycodeName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		domain string
		record string
		want   string
	}{
		{"unicode zone and label", "例.example.com", "例", "xn--fsq.xn--fsq.example.com."},
		{"punycode zone, unicode label", "xn--fsq.example.com", "例", "xn--fsq.xn--fsq.example.com."},
		{"apex", "例.example.com", "@", "xn--fsq.example.com."},
		{"empty name is apex", "例.example.com", "", "xn--fsq.example.com."},
		{"wildcard label survives", "例.example.com", "*", "*.xn--fsq.example.com."},
		{"acme label survives", "例.example.com", "_acme-challenge", "_acme-challenge.xn--fsq.example.com."},
		{"already qualified in unicode", "例.example.com", "www.例.example.com", "www.xn--fsq.example.com."},
		{"ascii is unchanged", "example.com", "www", "www.example.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, recordFQDN(tt.domain, tt.record))
		})
	}
}

func TestRelativeRecordNameStripsPunycodeZone(t *testing.T) {
	t.Parallel()

	// Huawei may answer with either spelling; both strip to the same label.
	require.Equal(t, "www", relativeRecordName("www.例.example.com.", "xn--fsq.example.com"))
	require.Equal(t, "www", relativeRecordName("www.xn--fsq.example.com.", "例.example.com"))
	require.Equal(t, "@", relativeRecordName("例.example.com.", "xn--fsq.example.com"))
	require.Equal(t, "xn--fsq", relativeRecordName("例.例.example.com.", "xn--fsq.example.com"))
	require.Equal(t, "*", relativeRecordName("*.例.example.com.", "xn--fsq.example.com"))
	require.Equal(t, "www", relativeRecordName("www.example.com.", "example.com"))
}
