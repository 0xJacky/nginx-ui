package geolite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test addresses are private or reserved ranges, which the country database
// embedded in cosy does not resolve, so the region code comes from the test
// database alone. Each one falls into a different branch of the search tree
// built by writeTestDB.
const (
	ipUnitedStates = "10.0.0.1"    // 0.0.0.0/2
	ipMainland     = "100.64.0.1"  // 64.0.0.0/2
	ipHongKong     = "172.16.0.1"  // 128.0.0.0/2
	ipCorrupt      = "192.168.0.1" // 192.0.0.0/3
	ipTaiwan       = "224.0.0.1"   // 224.0.0.0/4
	ipNoRecord     = "240.0.0.1"   // 240.0.0.0/4
)

func testDBNodes() [][2]mmdbChild {
	unitedStates := map[string]any{
		"country": map[string]any{
			"iso_code": "US",
			"names":    map[string]any{"en": "United States", "zh-CN": "美国"},
		},
		"subdivisions": []any{
			map[string]any{"names": map[string]any{"en": "California", "zh-CN": "加利福尼亚州"}},
		},
		"city": map[string]any{"names": map[string]any{"en": "Mountain View", "zh-CN": "山景城"}},
	}

	// Enterprise-style record: flat province/city names take precedence over
	// the localized maps and subdivisions, and c1-c4 carry ISP details.
	mainland := map[string]any{
		"country": map[string]any{"iso_code": " CN "},
		"province": map[string]any{
			"name":    " Guangdong ",
			"name_zh": "广东",
		},
		"subdivisions": []any{
			map[string]any{"names": map[string]any{"en": "Ignored", "zh-CN": "忽略"}},
		},
		"city": map[string]any{
			"name":  "Shenzhen",
			"names": map[string]any{"zh-CN": "深圳"},
		},
		"c1": " China Telecom ",
		"c2": "AS4134",
		"c3": "idc",
		"c4": "   ",
	}

	// No province and no Chinese city name.
	hongKong := map[string]any{
		"country": map[string]any{"iso_code": "HK"},
		"city":    map[string]any{"names": map[string]any{"en": "Hong Kong"}},
	}

	// A field with the wrong type makes decoding fail.
	corrupt := map[string]any{
		"country": map[string]any{"iso_code": "DE"},
		"c1":      uint32(7),
	}

	// Only the flat subdivision names are present.
	taiwan := map[string]any{
		"country":      map[string]any{"iso_code": "TW"},
		"subdivisions": []any{map[string]any{"name": "Taipei City", "name_zh": "台北市"}},
		"city":         map[string]any{"name": "Taipei", "name_zh": "台北"},
	}

	return [][2]mmdbChild{
		{nodeChild(1), nodeChild(2)},
		{recordChild(unitedStates), recordChild(mainland)},
		{recordChild(hongKong), nodeChild(3)},
		{recordChild(corrupt), nodeChild(4)},
		{recordChild(taiwan), emptyChild()},
	}
}

// writeTestDB writes the test database to path.
func writeTestDB(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, buildMMDB(t, testDBNodes()), 0o644))
}

// newTestService opens the test database through the same loader GetService
// uses, without touching the process-wide singleton.
func newTestService(t *testing.T) *Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "GeoLite2-City.mmdb")
	writeTestDB(t, path)

	s := &Service{}
	require.NoError(t, s.loadFromFile(path))
	t.Cleanup(s.Close)
	return s
}

func TestServiceSearch(t *testing.T) {
	s := newTestService(t)

	tests := []struct {
		name string
		ip   string
		want *IPLocation
	}{
		{
			name: "english names outside china",
			ip:   ipUnitedStates,
			want: &IPLocation{RegionCode: "US", Province: "California", City: "Mountain View"},
		},
		{
			name: "chinese names and trimmed enterprise fields in mainland china",
			ip:   ipMainland,
			want: &IPLocation{
				RegionCode: "CN",
				Province:   "广东",
				City:       "深圳",
				C1:         "China Telecom",
				C2:         "AS4134",
				C3:         "idc",
			},
		},
		{
			name: "hong kong is normalized to CN with a placeholder province",
			ip:   ipHongKong,
			want: &IPLocation{RegionCode: "CN", Province: "其它", City: "Hong Kong"},
		},
		{
			name: "taiwan falls back to subdivision names",
			ip:   ipTaiwan,
			want: &IPLocation{RegionCode: "CN", Province: "台北市", City: "台北"},
		},
		{
			name: "address without a record yields an empty location",
			ip:   ipNoRecord,
			want: &IPLocation{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Search(tt.ip)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServiceSearchFailures(t *testing.T) {
	s := newTestService(t)

	tests := []struct {
		name    string
		ip      string
		wantErr string
	}{
		{name: "invalid address", ip: "not-an-ip", wantErr: "invalid IP address: not-an-ip"},
		{name: "empty address", ip: "", wantErr: "invalid IP address: "},
		{name: "record fails to decode", ip: ipCorrupt, wantErr: "no location data found for IP: " + ipCorrupt},
		// The test database is IPv4-only, so an IPv6 lookup errors out.
		{name: "lookup error without a country code", ip: "2001:db8::1", wantErr: "no location data found for IP: 2001:db8::1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.Search(tt.ip)
			require.Error(t, err)
			assert.Nil(t, got)
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestServiceSearchFallsBackToCountryCode(t *testing.T) {
	s := newTestService(t)

	// The IPv4-only test database cannot answer an IPv6 lookup, but cosy's
	// embedded country database knows Google's public resolver, so Search
	// still returns the country code alone.
	got, err := s.Search("2001:4860:4860::8888")
	require.NoError(t, err)
	assert.Equal(t, &IPLocation{RegionCode: "US"}, got)
}

func TestServiceSearchWithISOMatchesSearch(t *testing.T) {
	s := newTestService(t)

	for _, ip := range []string{ipUnitedStates, ipMainland, ipNoRecord} {
		want, wantErr := s.Search(ip)
		got, gotErr := s.SearchWithISO(ip)
		assert.Equal(t, wantErr, gotErr, ip)
		assert.Equal(t, want, got, ip)
	}
}

func TestServiceSearchWithoutDatabase(t *testing.T) {
	s := &Service{}

	got, err := s.Search(ipUnitedStates)
	assert.Nil(t, got)
	assert.EqualError(t, err, "no databases loaded")
}

func TestServiceCloseIsIdempotent(t *testing.T) {
	s := newTestService(t)

	s.Close()
	assert.NotPanics(t, s.Close)

	got, err := s.Search(ipUnitedStates)
	assert.Nil(t, got)
	assert.EqualError(t, err, "no databases loaded")
}

func TestServiceInit(t *testing.T) {
	t.Run("database missing", func(t *testing.T) {
		dir := useTempConfDir(t)

		err := (&Service{}).init()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "GeoLite2 database not found at "+filepath.Join(dir, "GeoLite2-City.mmdb"))
	})

	t.Run("database unreadable", func(t *testing.T) {
		useTempConfDir(t)
		require.NoError(t, os.WriteFile(GetDBPath(), []byte("not a maxmind database"), 0o644))

		err := (&Service{}).init()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load GeoLite2 database")
	})

	t.Run("database loaded from the resolved path", func(t *testing.T) {
		dir := useTempConfDir(t)
		custom := filepath.Join(dir, "custom", "city.mmdb")
		require.NoError(t, os.MkdirAll(filepath.Dir(custom), 0o755))
		writeTestDB(t, custom)
		setCustomMMDB(t, "custom/city.mmdb")

		s := &Service{}
		require.NoError(t, s.init())
		t.Cleanup(s.Close)

		got, err := s.Search(ipUnitedStates)
		require.NoError(t, err)
		assert.Equal(t, "US", got.RegionCode)
	})
}

func TestIsChineseRegion(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{code: "CN", want: true},
		{code: "HK", want: true},
		{code: "MO", want: true},
		{code: "TW", want: true},
		{code: "US", want: false},
		{code: "cn", want: false},
		{code: " CN", want: false},
		{code: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			assert.Equal(t, tt.want, IsChineseRegion(tt.code))
		})
	}
}

func TestIsChineseIP(t *testing.T) {
	tests := []struct {
		name string
		loc  *IPLocation
		want bool
	}{
		{name: "nil location", loc: nil, want: false},
		{name: "mainland", loc: &IPLocation{RegionCode: "CN"}, want: true},
		{name: "hong kong", loc: &IPLocation{RegionCode: "HK"}, want: true},
		{name: "macau", loc: &IPLocation{RegionCode: "MO"}, want: true},
		{name: "taiwan", loc: &IPLocation{RegionCode: "TW"}, want: true},
		{name: "other country", loc: &IPLocation{RegionCode: "JP"}, want: false},
		{name: "empty region", loc: &IPLocation{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, IsChineseIP(tt.loc))
		})
	}
}
