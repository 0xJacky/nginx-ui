package settings

import "time"

const (
	defaultLogrotateIntervalMinutes = 1440
)

const InvalidLogrotateIntervalMessage = "logrotate interval must be greater than 0"

type Logrotate struct {
	Enabled  bool   `json:"enabled"`
	CMD      string `json:"cmd" protected:"true"`
	Interval int    `json:"interval" binding:"omitempty,min=1"`
}

var LogrotateSettings = &Logrotate{
	Enabled:  false,
	CMD:      "logrotate /etc/logrotate.d/nginx",
	Interval: defaultLogrotateIntervalMinutes, // 24 hours
}

func (l Logrotate) HasValidInterval() bool {
	return l.Interval > 0
}

func (l Logrotate) GetInterval() time.Duration {
	if !l.HasValidInterval() {
		return defaultLogrotateIntervalMinutes * time.Minute
	}

	return time.Duration(l.Interval) * time.Minute
}
