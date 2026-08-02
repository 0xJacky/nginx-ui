package settings

import "time"

const (
	defaultSiteCheckConcurrency     = 5
	defaultSiteCheckIntervalSeconds = 300
	maxSiteCheckConcurrency         = 20
	minSiteCheckIntervalSeconds     = 30
)

type SiteCheck struct {
	Enabled         bool `json:"enabled"`
	Concurrency     int  `json:"concurrency" binding:"omitempty,min=1,max=20"`
	IntervalSeconds int  `json:"interval_seconds" binding:"omitempty,min=30"`
}

var SiteCheckSettings = &SiteCheck{
	Enabled:         true,
	Concurrency:     defaultSiteCheckConcurrency,
	IntervalSeconds: defaultSiteCheckIntervalSeconds,
}

// GetConcurrency returns the configured concurrency, clamped to a safe range.
//
// Pointer receiver on purpose. A value receiver copies the whole struct on
// every call, so the periodic check loop was reading Enabled once a tick even
// though it only wants Concurrency — enough for the race detector to flag it
// against any test that toggles Enabled. Reading just the field also avoids a
// struct copy in a hot loop.
func (s *SiteCheck) GetConcurrency() int {
	if s.Concurrency < 1 {
		return defaultSiteCheckConcurrency
	}
	if s.Concurrency > maxSiteCheckConcurrency {
		return maxSiteCheckConcurrency
	}
	return s.Concurrency
}

// GetInterval returns the periodic sweep interval, clamped to a safe minimum.
// Pointer receiver for the same reason as GetConcurrency.
func (s *SiteCheck) GetInterval() time.Duration {
	seconds := s.IntervalSeconds
	if seconds < minSiteCheckIntervalSeconds {
		seconds = defaultSiteCheckIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}
