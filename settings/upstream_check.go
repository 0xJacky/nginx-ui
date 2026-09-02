package settings

import "time"

const (
	defaultUpstreamCheckIntervalSeconds = 30
	minUpstreamCheckIntervalSeconds     = 5
)

type UpstreamCheck struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds" binding:"omitempty,min=5"`
}

var UpstreamCheckSettings = &UpstreamCheck{
	Enabled:         true,
	IntervalSeconds: defaultUpstreamCheckIntervalSeconds,
}

// Pointer receiver so the call reads only the interval field rather than
// copying the whole struct, which would make it race with concurrent writes.
func (s *UpstreamCheck) GetInterval() time.Duration {
	seconds := s.IntervalSeconds
	if seconds < minUpstreamCheckIntervalSeconds {
		seconds = defaultUpstreamCheckIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}
