package demo

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/geolite"
)

// demoBuildTime anchors every fabricated timestamp so the demo does not appear
// to have been built moments ago on each cold start.
var demoBuildTime = time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

// geoLiteAvailability reports the city database as installed.
//
// The demo image deliberately does not ship GeoLite2-City (61 MB). Country
// codes come from the database embedded in cosy, and provinces and cities are
// fabricated by geoService, so the feature works — there is simply nothing for
// the operator to download. Reporting it as present keeps the self-check green
// and hides a download button that would only waste bandwidth.
type geoLiteAvailability struct{}

var _ geolite.AvailabilityProvider = (*geoLiteAvailability)(nil)

func (geoLiteAvailability) Availability() geolite.Availability {
	return geolite.Availability{
		Exists:       true,
		Path:         geolite.GetDBPath(),
		Size:         64_212_480,
		LastModified: demoBuildTime,
	}
}
