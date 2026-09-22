package upstream

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// nginx accepts `server unix:/run/php-fpm.sock;` inside an upstream block, and
// the parser keeps that spelling in the socket address it hands to the probe.
// The probe therefore has to reach a listening socket at that path.
func TestAvailabilityTestTargets_UnixSocketUpstream(t *testing.T) {
	// A unix socket path is limited to about 100 bytes, so keep the temporary
	// directory short instead of using t.TempDir, whose name carries the test name.
	dir, err := os.MkdirTemp("", "ngxui")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	socketPath := filepath.Join(dir, "php-fpm.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on unix socket: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	content := fmt.Sprintf("upstream php {\n    server unix:%s;\n}\n", socketPath)
	targets := ParseProxyTargetsAndUpstreamsFromRawContent(content).ProxyTargets
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	socket := formatSocketAddress(targets[0].Host, targets[0].Port)
	if socket != "unix:"+socketPath {
		t.Fatalf("expected socket address %q, got %q", "unix:"+socketPath, socket)
	}

	status, ok := AvailabilityTestTargets(targets)[socket]
	if !ok {
		t.Fatalf("no status reported for %q", socket)
	}
	if !status.Online {
		t.Errorf("expected %q to be reported online while a listener is bound to it", socket)
	}
}

func TestTestUnixSocketLatency_BarePath(t *testing.T) {
	dir, err := os.MkdirTemp("", "ngxui")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	socketPath := filepath.Join(dir, "bare.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("listen on unix socket: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	var wg sync.WaitGroup
	wg.Add(1)
	status := &Status{}
	testUnixSocketLatency(&wg, socketPath, status)
	wg.Wait()
	if !status.Online {
		t.Errorf("expected a bare socket path to stay probeable")
	}
}
