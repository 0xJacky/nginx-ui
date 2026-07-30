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

func (s UpstreamCheck) GetInterval() time.Duration {
	seconds := s.IntervalSeconds
	if seconds < minUpstreamCheckIntervalSeconds {
		seconds = defaultUpstreamCheckIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}
