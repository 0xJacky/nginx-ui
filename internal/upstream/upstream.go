package upstream

import (
	"crypto/tls"
	"maps"
	"net"
	"sync"
	"time"
)

const MaxTimeout = 5 * time.Second
const MaxConcurrentWorker = 10

type Status struct {
	Online  bool    `json:"online"`
	Latency float32 `json:"latency"`
}

// Prober overrides how upstream targets are probed.
//
// The slot defaults to nil and only internal/demo fills it, so a production
// binary always dials for real. A demo instance has no reachable backends and
// no business making outbound connections, which on a metered host is also
// billed egress.
type Prober interface {
	// Probe returns a status per socket. Returning no entry for a socket means
	// "no opinion": the caller falls back to a real dial.
	Probe(sockets []string) map[string]*Status
}

var prober Prober

// SetProber installs a probe override. Call once, at boot.
func SetProber(p Prober) {
	prober = p
}

func AvailabilityTest(body []string) (result map[string]*Status) {
	if p := prober; p != nil {
		if fabricated := p.Probe(body); fabricated != nil {
			result = make(map[string]*Status, len(body))
			var remaining []string
			for _, socket := range body {
				if status, ok := fabricated[socket]; ok {
					result[socket] = status
					continue
				}
				remaining = append(remaining, socket)
			}
			// Anything the override declined still gets a real dial, so an
			// override can never silently answer for a target it does not know.
			maps.Copy(result, realAvailabilityTest(remaining))
			return result
		}
	}

	return realAvailabilityTest(body)
}

// AvailabilityTestTargets probes parsed upstream targets using the transport
// implied by their pass directive. HTTPS and gRPCS targets complete a TLS
// handshake so the peer does not report a truncated handshake as an error.
func AvailabilityTestTargets(targets []ProxyTarget) (result map[string]*Status) {
	targetsBySocket := make(map[string]ProxyTarget, len(targets))
	for _, target := range targets {
		socket := formatSocketAddress(target.Host, target.Port)
		if existing, ok := targetsBySocket[socket]; ok {
			existing.Scheme = mergeProbeScheme(existing.Scheme, target.Scheme)
			targetsBySocket[socket] = existing
			continue
		}
		targetsBySocket[socket] = target
	}

	result = make(map[string]*Status, len(targetsBySocket))
	remaining := make(map[string]ProxyTarget, len(targetsBySocket))
	for socket, target := range targetsBySocket {
		remaining[socket] = target
	}

	if p := prober; p != nil {
		sockets := make([]string, 0, len(targetsBySocket))
		for socket := range targetsBySocket {
			sockets = append(sockets, socket)
		}
		for socket, status := range p.Probe(sockets) {
			if _, ok := targetsBySocket[socket]; !ok {
				continue
			}
			result[socket] = status
			delete(remaining, socket)
		}
	}

	plainSockets := make([]string, 0, len(remaining))
	secureTargets := make([]ProxyTarget, 0, len(remaining))
	for socket, target := range remaining {
		if isTLSProbeScheme(target.Scheme) {
			secureTargets = append(secureTargets, target)
			continue
		}
		plainSockets = append(plainSockets, socket)
	}

	maps.Copy(result, realAvailabilityTest(plainSockets))
	maps.Copy(result, realTLSAvailabilityTest(secureTargets))
	return result
}

func realAvailabilityTest(body []string) (result map[string]*Status) {
	result = make(map[string]*Status)

	wg := sync.WaitGroup{}
	wg.Add(len(body))
	c := make(chan struct{}, MaxConcurrentWorker)
	for _, socket := range body {
		c <- struct{}{}
		s := &Status{}
		go testLatency(c, &wg, socket, s)
		result[socket] = s
	}
	wg.Wait()

	return
}

func realTLSAvailabilityTest(targets []ProxyTarget) (result map[string]*Status) {
	result = make(map[string]*Status, len(targets))
	wg := sync.WaitGroup{}
	wg.Add(len(targets))
	workers := make(chan struct{}, MaxConcurrentWorker)

	for _, target := range targets {
		workers <- struct{}{}
		status := &Status{}
		go testTLSLatency(workers, &wg, target, status)
		result[formatSocketAddress(target.Host, target.Port)] = status
	}

	wg.Wait()
	return result
}

func testTLSLatency(workers chan struct{}, wg *sync.WaitGroup, target ProxyTarget, status *Status) {
	defer func() {
		wg.Done()
		<-workers
	}()

	start := time.Now()
	dialer := &net.Dialer{Timeout: MaxTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", formatSocketAddress(target.Host, target.Port), &tls.Config{
		// Availability checks validate that the endpoint can negotiate TLS. They
		// intentionally do not duplicate certificate-policy validation performed
		// by the real proxy request, which may use a private CA or an IP address.
		InsecureSkipVerify: true, //nolint:gosec
		ServerName:         target.Host,
	})
	if err != nil {
		return
	}
	defer conn.Close()

	status.Online = true
	status.Latency = float32(time.Since(start)) / float32(time.Millisecond)
}

func testLatency(c chan struct{}, wg *sync.WaitGroup, socket string, status *Status) {
	defer func() {
		wg.Done()
		<-c
	}()

	scopedWg := sync.WaitGroup{}
	scopedWg.Add(2)
	go testTCPLatency(&scopedWg, socket, status)
	go testUnixSocketLatency(&scopedWg, socket, status)
	scopedWg.Wait()
}

func testTCPLatency(wg *sync.WaitGroup, socket string, status *Status) {
	defer func() {
		wg.Done()
	}()
	start := time.Now()
	conn, err := net.DialTimeout("tcp", socket, MaxTimeout)

	if err != nil {
		return
	}

	defer conn.Close()

	end := time.Now()

	status.Online = true
	status.Latency = float32(end.Sub(start)) / float32(time.Millisecond)
}

func testUnixSocketLatency(wg *sync.WaitGroup, socket string, status *Status) {
	defer func() {
		wg.Done()
	}()
	start := time.Now()
	conn, err := net.DialTimeout("unix", socket, MaxTimeout)

	if err != nil {
		return
	}

	defer conn.Close()

	end := time.Now()

	status.Online = true
	status.Latency = float32(end.Sub(start)) / float32(time.Millisecond)
}
