package demo

import (
	"time"

	"github.com/0xJacky/Nginx-UI/internal/performance"
)

// stubStatus fabricates nginx connection counters.
//
// The demo container serves almost no traffic, so the real stub_status figures
// sit near zero and the dashboard looks broken. Only the counters are
// fabricated: process info and configuration info still describe the nginx that
// is genuinely running in the container.
type stubStatus struct{}

var _ performance.StubStatusProvider = (*stubStatus)(nil)

// Counter baselines. Rates are per time bucket, chosen so that
// requests/handled lands around 2.5, which is a believable keep-alive ratio.
const (
	acceptsBase  = 1_800_000
	acceptsRate  = 640
	requestsBase = 4_600_000
	requestsRate = 1_610
)

func (stubStatus) StubStatus() *performance.StubStatusData {
	bucket := timeBucket(time.Now())
	s := seed("stub_status", "counters") ^ bucket

	active := rangeInt(s, 120, 261)
	reading := rangeInt(s>>8, 2, 7)
	writing := rangeInt(s>>16, 15, 41)

	// Waiting is the remainder by definition; clamp so a wide reading/writing
	// draw can never produce a negative count.
	waiting := active - reading - writing
	if waiting < 0 {
		waiting = 0
	}

	accepts := acceptsBase + int(bucket%100_000)*acceptsRate

	return &performance.StubStatusData{
		Active:  active,
		Accepts: accepts,
		// Equal to accepts on purpose: a gap between them means dropped
		// connections, which would show the demo as unhealthy.
		Handled:  accepts,
		Requests: requestsBase + int(bucket%100_000)*requestsRate,
		Reading:  reading,
		Writing:  writing,
		Waiting:  waiting,
	}
}
