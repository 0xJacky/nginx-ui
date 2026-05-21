package setup

import (
	"context"
	"strings"
)

// CommandRunner is the part of the SSH client the verify pipeline uses.
// Depending on the interface rather than the concrete client lets the checks
// be exercised without a live host.
type CommandRunner interface {
	Exec(ctx context.Context, name string, args ...string) (string, error)
}

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
