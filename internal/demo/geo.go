package demo

import (
	"net/netip"
	"slices"

	"github.com/0xJacky/Nginx-UI/internal/nginx_log/parser"
	"github.com/uozi-tech/cosy/geoip"
)

// geoService fabricates province and city for documentation addresses while
// deferring to the real, embedded country database for everything else.
//
// No city database is shipped with the demo image: GeoLite2-City is 61 MB.
// GeoLite2-Country is already embedded in cosy (geoip.ParseIP), so country
// codes — and therefore the world map — remain genuinely correct.
type geoService struct{}

var _ parser.GeoIPService = (*geoService)(nil)

func (g *geoService) Search(ip string) (*parser.GeoLocation, error) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		// Match the real service: an unparseable address is simply not found.
		return nil, nil
	}

	country := geoip.ParseIP(ip)

	if !isDocumentationIP(addr) {
		// Real address: report the real country and claim nothing more.
		if country == "" || country == "Unknown" {
			return nil, nil
		}
		return &parser.GeoLocation{CountryCode: country, RegionCode: country}, nil
	}

	s := seed("geo", ip)

	// Roughly two thirds of demo traffic is domestic, which is what makes the
	// China map worth rendering at all.
	if s%3 != 0 {
		province := pick(cnProvinces, s)
		city := pick(cnCities[province], s>>8)
		// The real path collapses CN/HK/MO/TW into "CN" (geolite.go:105); the
		// China map keys off that, so mirror it exactly.
		return &parser.GeoLocation{
			CountryCode: "CN",
			RegionCode:  "CN",
			Province:    province,
			City:        city,
		}, nil
	}

	overseas := overseasCountries()
	code := pick(overseas, s>>16)
	return &parser.GeoLocation{
		CountryCode: code,
		RegionCode:  code,
		City:        pick(overseasCities[code], s>>24),
	}, nil
}

// overseasCountries returns the country codes in a stable order. Ranging a map
// directly would make picks depend on Go's randomised map iteration.
func overseasCountries() []string {
	codes := make([]string, 0, len(overseasCities))
	for code := range overseasCities {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}
