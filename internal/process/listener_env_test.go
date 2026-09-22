package process

import (
	"os"
	"testing"
)

func TestResolveActiveListener(t *testing.T) {
	t.Setenv(ActiveListenerEnv, "")
	_ = os.Unsetenv(ActiveListenerEnv)
	if n, a := ActiveListener(); n != "" || a != "" {
		t.Fatalf("ActiveListener before resolve = %q %q", n, a)
	}

	network, address, first, changed := ResolveActiveListener("tcp", "0.0.0.0:9000")
	if n, a := ActiveListener(); n != "tcp" || a != "0.0.0.0:9000" {
		t.Fatalf("ActiveListener after resolve = %q %q", n, a)
	}
	if network != "tcp" || address != "0.0.0.0:9000" || !first || changed {
		t.Fatalf("fresh start = %q %q first=%v changed=%v", network, address, first, changed)
	}
	if got := os.Getenv(ActiveListenerEnv); got != "tcp|0.0.0.0:9000" {
		t.Fatalf("env = %q", got)
	}

	// A child with unchanged configuration keeps the parent's listener.
	network, address, first, changed = ResolveActiveListener("tcp", "0.0.0.0:9000")
	if network != "tcp" || address != "0.0.0.0:9000" || first || changed {
		t.Fatalf("same child = %q %q first=%v changed=%v", network, address, first, changed)
	}

	// A child whose configuration switched transports must still follow the
	// parent, because the parent is the process holding the public listener.
	network, address, first, changed = ResolveActiveListener("unix", "/run/nginx-ui.sock")
	if network != "tcp" || address != "0.0.0.0:9000" || first || !changed {
		t.Fatalf("changed child = %q %q first=%v changed=%v", network, address, first, changed)
	}

	// Garbage in the variable is ignored and overwritten.
	t.Setenv(ActiveListenerEnv, "nonsense")
	network, address, first, changed = ResolveActiveListener("unix", "/run/nginx-ui.sock")
	if network != "unix" || address != "/run/nginx-ui.sock" || !first || changed {
		t.Fatalf("garbage env = %q %q first=%v changed=%v", network, address, first, changed)
	}
	if got := os.Getenv(ActiveListenerEnv); got != "unix|/run/nginx-ui.sock" {
		t.Fatalf("env after garbage = %q", got)
	}
}
