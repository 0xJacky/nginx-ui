package parser

import "github.com/0xJacky/Nginx-UI/internal/geolite"

// GeoLiteAdapter adapts the geolite.Service to the parser.GeoIPService interface.
type GeoLiteAdapter struct {
	geoService *geolite.Service
}

// NewGeoLiteAdapter creates a new adapter.
func NewGeoLiteAdapter(service *geolite.Service) *GeoLiteAdapter {
	return &GeoLiteAdapter{geoService: service}
}

// Search performs a geo IP lookup and converts the result to the parser's GeoLocation type.
func (a *GeoLiteAdapter) Search(ip string) (*GeoLocation, error) {
	location, err := a.geoService.Search(ip)
	if err != nil || location == nil {
		return nil, err // No error, but no location found, or an actual error occurred.
	}

	return &GeoLocation{
		RegionCode: location.RegionCode,
		Province:   location.Province,
		City:       location.City,
	}, nil
}
