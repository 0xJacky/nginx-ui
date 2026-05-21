package setup

import "strings"

func checkKnownHostsPersistence(path string) StepOutcome {
	if path == "" {
		path = "/etc/nginx-ui/known_hosts"
	}
	if strings.HasPrefix(path, "/etc/nginx-ui/") {
		return StepOutcome{OK: true, Level: "success", Detail: path + " is under the recommended persisted data directory"}
	}
	return StepOutcome{
		OK:          false,
		Level:       "warning",
		Detail:      path + " is outside the recommended /etc/nginx-ui data directory",
		Remediation: "Persist /etc/nginx-ui with a Docker named volume or bind mount so known_hosts survives container rebuilds.",
	}
}
