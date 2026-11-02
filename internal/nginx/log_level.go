package nginx

import "strings"

// refer to https://nginx.org/en/docs/ngx_core_module.html#error_log
// nginx log level: debug, info, notice, warn, error, crit, alert, or emerg

const Unknown = -1

// The values below must stay aligned with the indexes of logLevel, because
// GetLogLevel returns an index into that slice.
const (
	Debug = iota
	Info
	Notice
	Warn
	Error
	Crit
	Alert
	Emerg
)

var logLevel = [...]string{
	"debug", "info", "notice", "warn", "error", "crit", "alert", "emerg",
}

// GetLogLevel returns the highest level nginx tagged in output. nginx writes the
// level in brackets ("nginx: [warn] ..."), so only a bracketed level counts: a bare
// word such as "alert" in "alertmanager.example.com" or "error" in an error.log path
// is part of the message, not its level.
func GetLogLevel(output string) (level int) {
	level = Unknown
	for k, v := range logLevel {
		if strings.Contains(output, "["+v+"]") {
			// Try to find the highest log level
			level = k
		}
	}
	return
}
