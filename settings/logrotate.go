package settings

type Logrotate struct {
	Enabled  bool   `json:"enabled"`
	CMD      string `json:"cmd" protected:"true"`
	Interval int    `json:"interval"`
}

var LogrotateSettings = &Logrotate{
	Enabled:  false,
	CMD:      "logrotate /etc/logrotate.d/nginx",
	Interval: 1440, // 24 hours
}
