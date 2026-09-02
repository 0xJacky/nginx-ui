package demo

import (
	"net/url"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
)

// siteProber answers for the hostnames the demo's own site configs declare.
//
// Any other hostname gets no opinion and is probed for real, so this cannot
// make a claim about a site the demo did not create. The demo container also
// has no route to the public internet in the intended deployment, so probing
// real sites would show every card as offline anyway.
type siteProber struct{}

var _ sitecheck.Prober = (*siteProber)(nil)

// demoSiteOutcome is the fabricated result per hostname. The mix is deliberate:
// mostly healthy, one degraded and one down, so the UI shows its warning and
// error states instead of a uniform wall of green.
var demoSiteOutcomes = map[string]sitecheck.ProbeResult{
	"ojbk.me": {
		Status:     sitecheck.StatusOnline,
		StatusCode: 200,
		Title:      "ojbk.me",
	},
	"langgood.com": {
		Status:     sitecheck.StatusError,
		StatusCode: 503,
		Title:      "Prime Sponsor",
		Error:      "Unexpected status code: 503",
		ErrorType:  sitecheck.ErrorTypeStatusCode,
	},
}

// defaultOutcome covers demo sites without an explicit entry.
var defaultOutcome = sitecheck.ProbeResult{
	Status:     sitecheck.StatusOnline,
	StatusCode: 200,
}

func (siteProber) Probe(siteURL string) (sitecheck.ProbeResult, bool) {
	host := hostOf(siteURL)
	if host == "" {
		return sitecheck.ProbeResult{}, false
	}

	outcome, known := demoSiteOutcomes[host]
	if !known {
		if !isDemoHost(host) {
			return sitecheck.ProbeResult{}, false
		}
		outcome = defaultOutcome
		outcome.Title = host
	}

	// Response time drifts per time bucket so the card does not look frozen,
	// while staying identical across the instances in this container.
	s := seed("sitecheck", host) ^ timeBucket(time.Now())
	outcome.ResponseTime = int64(rangeInt(s, 40, 260))

	return outcome, true
}

// isDemoHost reports whether a hostname belongs to the demo's own fixtures.
// Kept explicit rather than answering for everything, so the prober stays a
// partial function.
func isDemoHost(host string) bool {
	for known := range demoSiteOutcomes {
		if host == known || strings.HasSuffix(host, "."+known) {
			return true
		}
	}
	return false
}

func hostOf(siteURL string) string {
	parsed, err := url.Parse(siteURL)
	if err != nil {
		return ""
	}
	if parsed.Host == "" {
		// Bare hostnames arrive without a scheme.
		return strings.TrimSuffix(siteURL, "/")
	}
	return parsed.Hostname()
}
