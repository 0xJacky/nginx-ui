package demo

import (
	"github.com/0xJacky/Nginx-UI/internal/geolite"
	"github.com/0xJacky/Nginx-UI/internal/llm"
	"github.com/0xJacky/Nginx-UI/internal/nginx_log/indexer"
	"github.com/0xJacky/Nginx-UI/internal/performance"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
	"github.com/0xJacky/Nginx-UI/internal/upstream"
	"github.com/0xJacky/Nginx-UI/internal/version"
	"github.com/uozi-tech/cosy/logger"
)

// override pairs a name with the registration it performs, so Install can
// report what it applied without every provider growing its own logging.
type override struct {
	name    string
	install func()
}

// overrides is the complete list of provider slots this package fills. Adding
// a fabricated subsystem means adding a line here and nowhere else.
func overrides() []override {
	return []override{
		{"geoip", func() { indexer.SetGeoIPService(&geoService{}) }},
		{"geolite-availability", func() { geolite.SetAvailabilityProvider(geoLiteAvailability{}) }},
		{"upstream-prober", func() { upstream.SetProber(upstreamProber{}) }},
		{"site-prober", func() { sitecheck.SetProber(siteProber{}) }},
		{"stub-status", func() { performance.SetStubStatusProvider(stubStatus{}) }},
		{"release-check", func() { version.SetReleaseProvider(releaseInfo{}) }},
		{"external-notifiers", installNotifierStubs},
		// Must run after the vendor packages' init(); the registry is
		// last-write-wins and boot happens well after package init.
		{"dns-providers", installDNSProviders},
		{"llm-transport", func() { llm.SetHTTPDoer(llmDoer{}) }},
		// Must precede nginx_log.InitializeServices, which is why Install runs
		// first in the boot sequence: the indexer reads these files once.
		{"log-fixtures", installLogFixtures},
	}
}

// Install fills every provider slot this package owns and returns the names of
// the overrides it applied. It is a no-op returning nil on a normal node.
//
// Must run before any consumer initialises: geo enrichment in particular is
// baked into the indexed document, and the log parser is created behind a
// sync.Once, so an override installed later would silently do nothing.
func Install() []string {
	if !Enabled() {
		return nil
	}

	all := overrides()
	applied := make([]string, 0, len(all))
	for _, o := range all {
		o.install()
		applied = append(applied, o.name)
	}

	logger.Warnf("Demo mode is enabled: %d subsystems now serve fabricated data (%v). "+
		"This node must not be treated as a real installation.", len(applied), applied)

	return applied
}
