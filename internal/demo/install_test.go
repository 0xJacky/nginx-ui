package demo

import (
	"net/netip"
	"sync"
	"testing"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withDemo flips the shared settings pointer for one test. NodeSettings is
// package-level state, so these tests must not call t.Parallel().
func withDemo(t *testing.T, enabled bool) {
	t.Helper()

	previous := settings.NodeSettings.Demo
	settings.NodeSettings.Demo = enabled
	t.Cleanup(func() { settings.NodeSettings.Demo = previous })
}

func TestInstallIsInertWhenDemoDisabled(t *testing.T) {
	withDemo(t, false)

	assert.Nil(t, Install(), "a normal node must install no overrides at all")
}

func TestInstallReportsEveryOverride(t *testing.T) {
	withDemo(t, true)

	applied := Install()

	require.Len(t, applied, len(overrides()))
	assert.Contains(t, applied, "geoip")
	assert.Contains(t, applied, "geolite-availability")
}

func TestGeoFabricatesOnlyForDocumentationAddresses(t *testing.T) {
	svc := &geoService{}

	// A documentation address gets a full fabricated location.
	got, err := svc.Search("203.0.113.7")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEmpty(t, got.RegionCode)

	// A real routable address must never gain a fabricated province or city.
	// This is the property that keeps the fake harmless if it ever loads on a
	// real installation.
	for _, ip := range []string{"8.8.8.8", "1.1.1.1", "223.5.5.5"} {
		got, err := svc.Search(ip)
		require.NoError(t, err, ip)
		if got == nil {
			continue
		}
		assert.Empty(t, got.Province, "province fabricated for real IP %s", ip)
		assert.Empty(t, got.City, "city fabricated for real IP %s", ip)
	}
}

func TestGeoMirrorsChineseRegionCollapsing(t *testing.T) {
	svc := &geoService{}

	// The real service collapses CN/HK/MO/TW into "CN" and the China map keys
	// off that, so any fabricated domestic location must do the same.
	for _, ip := range documentationSample(t, 200) {
		got, err := svc.Search(ip)
		require.NoError(t, err)
		require.NotNil(t, got)

		if got.Province != "" {
			assert.Equal(t, "CN", got.RegionCode,
				"a fabricated province must always carry region code CN")
			assert.Contains(t, cnProvinces, got.Province)
		}
	}
}

func TestGeoIsDeterministic(t *testing.T) {
	svc := &geoService{}

	for _, ip := range documentationSample(t, 50) {
		first, err := svc.Search(ip)
		require.NoError(t, err)

		for range 20 {
			again, err := svc.Search(ip)
			require.NoError(t, err)
			assert.Equal(t, first, again, "geo lookup for %s is not stable", ip)
		}
	}
}

func TestGeoIsDeterministicUnderConcurrency(t *testing.T) {
	svc := &geoService{}
	ips := documentationSample(t, 32)

	expected := make([]string, len(ips))
	for i, ip := range ips {
		got, err := svc.Search(ip)
		require.NoError(t, err)
		expected[i] = got.Province + "|" + got.City + "|" + got.RegionCode
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, ip := range ips {
				got, err := svc.Search(ip)
				if err != nil || got == nil {
					t.Errorf("lookup failed for %s: %v", ip, err)
					return
				}
				if actual := got.Province + "|" + got.City + "|" + got.RegionCode; actual != expected[i] {
					t.Errorf("%s: got %q, want %q", ip, actual, expected[i])
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestGeoDeclinesUnparseableInput(t *testing.T) {
	svc := &geoService{}

	for _, input := range []string{"", "not-an-ip", "999.999.999.999", "192.0.2"} {
		got, err := svc.Search(input)
		assert.NoError(t, err, input)
		assert.Nil(t, got, input)
	}
}

func TestGeoLiteAvailabilityReportsInstalled(t *testing.T) {
	availability := geoLiteAvailability{}.Availability()

	assert.True(t, availability.Exists)
	assert.NotEmpty(t, availability.Path)
	assert.Positive(t, availability.Size)
	assert.False(t, availability.LastModified.IsZero())
}

func TestSeedIsStableAcrossCalls(t *testing.T) {
	first := seed("geo", "203.0.113.1")
	for range 100 {
		assert.Equal(t, first, seed("geo", "203.0.113.1"))
	}
	assert.NotEqual(t, first, seed("geo", "203.0.113.2"))
	assert.NotEqual(t, first, seed("upstream", "203.0.113.1"))
}

func TestRangeIntStaysInBounds(t *testing.T) {
	for i := range uint64(500) {
		got := rangeInt(i, 10, 20)
		assert.GreaterOrEqual(t, got, 10)
		assert.Less(t, got, 20)
	}

	// Degenerate ranges must not panic.
	assert.Equal(t, 5, rangeInt(123, 5, 5))
	assert.Equal(t, 5, rangeInt(123, 5, 1))
}

func TestPickHandlesEmptyTable(t *testing.T) {
	assert.Empty(t, pick([]string{}, 42))
	assert.Equal(t, "only", pick([]string{"only"}, 42))
}

func TestOverseasCountriesIsStablyOrdered(t *testing.T) {
	first := overseasCountries()
	for range 50 {
		assert.Equal(t, first, overseasCountries(),
			"map iteration order must not leak into fabricated data")
	}
}

// documentationSample returns n addresses drawn from the documentation ranges
// the demo layer claims.
func documentationSample(t *testing.T, n int) []string {
	t.Helper()

	base := netip.MustParseAddr("203.0.113.0")
	out := make([]string, 0, n)
	for i := range n {
		addr := base
		for range i {
			addr = addr.Next()
		}
		require.True(t, isDocumentationIP(addr))
		out = append(out, addr.String())
	}
	return out
}
