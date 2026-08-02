package demo

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/upstream"
)

// upstreamProber answers for loopback targets, which is what the demo's own
// site and stream configs point at.
//
// It returns no entry for anything else, so a target the demo did not create
// still gets a real dial. That keeps the override from quietly asserting that
// some unrelated backend is healthy.
type upstreamProber struct{}

var _ upstream.Prober = (*upstreamProber)(nil)

// demoSockets are the targets the bundled demo configs reference. Values are
// the baseline latency in milliseconds; the reported figure drifts around it.
var demoSockets = map[string]float32{
	"127.0.0.1:9000": 1.2,
	"127.0.0.1:9001": 1.4,
	"127.0.0.1:9002": 1.6,
	"127.0.0.1:9003": 3.4,
	"127.0.0.1:9005": 2.1,
	"127.0.0.1:443":  187.6,
	"127.0.0.1:80":   4.8,
}

// offlineSockets always report down, so the UI shows its failure states rather
// than an implausible wall of green.
var offlineSockets = map[string]bool{
	"127.0.0.1:9005": true,
}

func (upstreamProber) Probe(sockets []string) map[string]*upstream.Status {
	if len(sockets) == 0 {
		return nil
	}

	bucket := timeBucket(time.Now())
	result := make(map[string]*upstream.Status, len(sockets))

	for _, socket := range sockets {
		base, known := demoSockets[socket]
		if !known {
			continue // no opinion; the caller dials for real
		}

		if offlineSockets[socket] {
			result[socket] = &upstream.Status{Online: false}
			continue
		}

		// Drift within +/-15% of the baseline, stepping once per time bucket so
		// every instance in the container reports the same figure.
		drift := rangeInt(seed("upstream", socket)^bucket, 85, 116)
		result[socket] = &upstream.Status{
			Online:  true,
			Latency: base * float32(drift) / 100,
		}
	}

	return result
}
