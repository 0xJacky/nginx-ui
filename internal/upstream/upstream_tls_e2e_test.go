package upstream

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/settings"
)

func TestHTTPSProxyPassAvailabilityProbeE2E(t *testing.T) {
	t.Run("raw TCP probe reproduces truncated TLS handshake", func(t *testing.T) {
		server, errorLog, _, _ := newTLSProbeServer()
		socket := socketFromURL(t, server.URL)

		status := AvailabilityTest([]string{socket})[socket]
		select {
		case <-errorLog.written:
		case <-time.After(time.Second):
		}
		server.Close()

		if status == nil || !status.Online {
			t.Fatalf("raw TCP probe status = %+v, want online", status)
		}
		if !strings.Contains(errorLog.String(), "TLS handshake error") || !strings.Contains(errorLog.String(), "EOF") {
			t.Fatalf("raw TCP probe error log = %q, want TLS handshake EOF reproduction", errorLog.String())
		}
	})

	t.Run("HTTPS-aware probe completes TLS without an HTTP request", func(t *testing.T) {
		server, errorLog, requestCount, connections := newTLSProbeServer()
		service := GetUpstreamService()
		service.ClearTargets()
		originalEnabled := settings.UpstreamCheckSettings.Enabled
		settings.UpstreamCheckSettings.Enabled = true
		t.Cleanup(func() {
			settings.UpstreamCheckSettings.Enabled = originalEnabled
			service.ClearTargets()
		})

		config := fmt.Sprintf(`server {
    location / {
        proxy_pass %s;
    }
}`, server.URL)
		if err := scanForProxyTargets("tls-probe-e2e.conf", []byte(config)); err != nil {
			server.Close()
			t.Fatalf("scan proxy targets: %v", err)
		}
		targets := service.GetTargets()
		if len(targets) != 1 {
			server.Close()
			t.Fatalf("parsed targets = %+v, want one HTTPS target", targets)
		}
		if targets[0].Scheme != "https" {
			server.Close()
			t.Fatalf("parsed target scheme = %q, want https", targets[0].Scheme)
		}

		socket := formatSocketAddress(targets[0].Host, targets[0].Port)
		service.PerformAvailabilityTest()
		status := service.GetAvailabilityMap()[socket]
		select {
		case <-connections.closed:
		case <-time.After(time.Second):
			server.Close()
			t.Fatal("HTTPS-aware probe connection did not close cleanly")
		}
		server.Close()

		if status == nil || !status.Online {
			t.Fatalf("HTTPS-aware probe status = %+v, want online", status)
		}
		if got := requestCount.Load(); got != 0 {
			t.Fatalf("HTTPS-aware probe sent %d HTTP requests, want zero", got)
		}
		if got := errorLog.String(); strings.Contains(got, "TLS handshake error") {
			t.Fatalf("HTTPS-aware probe emitted server TLS error: %q", got)
		}
	})
}

func TestNamedHTTPSUpstreamPreservesProbeScheme(t *testing.T) {
	config := `upstream secure_backend {
    server 127.0.0.1:9000;
}
server {
    location / {
        proxy_pass https://secure_backend;
    }
}`

	result := ParseProxyTargetsAndUpstreamsFromRawContent(config)
	if len(result.ProxyTargets) != 1 {
		t.Fatalf("proxy targets = %+v, want one upstream server", result.ProxyTargets)
	}
	if got := result.ProxyTargets[0].Scheme; got != "https" {
		t.Fatalf("proxy target scheme = %q, want https", got)
	}
	if got := result.Upstreams["secure_backend"][0].Scheme; got != "https" {
		t.Fatalf("upstream definition scheme = %q, want https", got)
	}
}

type synchronizedLog struct {
	mu      sync.Mutex
	buffer  bytes.Buffer
	written chan struct{}
}

type connectionStateRecorder struct {
	closed chan struct{}
}

func (r *connectionStateRecorder) record(_ net.Conn, state http.ConnState) {
	if state != http.StateClosed {
		return
	}
	select {
	case r.closed <- struct{}{}:
	default:
	}
}

func (l *synchronizedLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n, err := l.buffer.Write(p)
	select {
	case l.written <- struct{}{}:
	default:
	}
	return n, err
}

func (l *synchronizedLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buffer.String()
}

func newTLSProbeServer() (*httptest.Server, *synchronizedLog, *atomic.Int32, *connectionStateRecorder) {
	errorLog := &synchronizedLog{written: make(chan struct{}, 1)}
	requestCount := &atomic.Int32{}
	connections := &connectionStateRecorder{closed: make(chan struct{}, 1)}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount.Add(1)
	}))
	server.Config.ErrorLog = log.New(errorLog, "", 0)
	server.Config.ConnState = connections.record
	server.StartTLS()
	return server, errorLog, requestCount, connections
}

func socketFromURL(t *testing.T, rawURL string) string {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	return parsedURL.Host
}
