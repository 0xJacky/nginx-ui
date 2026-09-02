package geolite

import (
	"os"
	"time"
)

// Availability describes whether the GeoLite2 city database is usable.
type Availability struct {
	Exists       bool
	Path         string
	Size         int64
	LastModified time.Time
}

// AvailabilityProvider overrides how availability is determined.
//
// The slot defaults to nil and only internal/demo ever fills it, so a
// production binary reports the real state of the filesystem and cannot be
// talked out of it. A demo instance ships no city database — it is 61 MB, and
// country codes already come from the database embedded in cosy — so it reports
// the database as present rather than offering a download that would be
// pointless and expensive.
type AvailabilityProvider interface {
	Availability() Availability
}

var availabilityProvider AvailabilityProvider

// SetAvailabilityProvider installs an override. Call once, before boot reaches
// any consumer.
func SetAvailabilityProvider(p AvailabilityProvider) {
	availabilityProvider = p
}

// CurrentAvailability reports the database state, honouring an installed
// override. Prefer this over calling DBExists directly.
func CurrentAvailability() Availability {
	if p := availabilityProvider; p != nil {
		return p.Availability()
	}

	path := GetDBPath()
	availability := Availability{Path: path, Exists: DBExists()}
	if !availability.Exists {
		return availability
	}

	if info, err := os.Stat(path); err == nil {
		availability.Size = info.Size()
		availability.LastModified = info.ModTime()
	}

	return availability
}
