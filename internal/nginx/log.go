package nginx

import "strings"

// refer to https://nginx.org/en/docs/ngx_core_module.html#error_log
// nginx log level: debug, info, notice, warn, error, crit, alert, or emerg

const (
	Unknown = -1
	Debug   = iota
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

func GetLogLevel(output string) (level int) {
	level = -1
	for k, v := range logLevel {
		if strings.Contains(output, v) {
			// Try to find the highest log level
			level = k
		}
	}
	return
}
