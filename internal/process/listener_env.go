package process

import (
	"os"
	"strings"
)

// ActiveListenerEnv carries the parent's listener choice to the child
// processes risefront spawns for graceful restarts. Children inherit the
// parent's environment, so a fresh start never sees it.
const ActiveListenerEnv = "NGINX_UI_ACTIVE_LISTENER"

// ResolveActiveListener reconciles the listener computed from the current
// configuration with the one the parent process is actually bound to.
//
// After a graceful restart the parent keeps its original listener and only
// forwards connections, so a child must use the parent's transport: the
// PROXY protocol policy depends on it, and a changed socket path would make
// the parent dial a child socket that does not exist. When the configuration
// differs from the active listener the active one wins and changed is true so
// the caller can tell the operator that a full restart is required.
//
// The first process records its choice in the environment and reports
// first=true, which is the only process that may prepare the socket path.
// ActiveListener reports the listener the process tree is bound to, as
// recorded by ResolveActiveListener, or empty strings before it ran.
func ActiveListener() (network, address string) {
	if n, a, ok := strings.Cut(os.Getenv(ActiveListenerEnv), "|"); ok && n != "" && a != "" {
		return n, a
	}
	return "", ""
}

func ResolveActiveListener(network, address string) (activeNetwork, activeAddress string, first, changed bool) {
	if raw := os.Getenv(ActiveListenerEnv); raw != "" {
		if parentNetwork, parentAddress, ok := strings.Cut(raw, "|"); ok && parentNetwork != "" && parentAddress != "" {
			changed = parentNetwork != network || parentAddress != address
			return parentNetwork, parentAddress, false, changed
		}
	}
	_ = os.Setenv(ActiveListenerEnv, network+"|"+address)
	return network, address, true, false
}
