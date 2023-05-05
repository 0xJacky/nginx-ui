package nginx

import "strings"

// refer to https://nginx.org/en/docs/ngx_core_module.html#error_log
// nginx log level: debug, info, notice, warn, error, crit, alert, or emerg

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

func GetLogLevel(output string) int {
	for k, v := range logLevel {
		if strings.Contains(output, v) {
			return k
		}
	}
	return -1
}
