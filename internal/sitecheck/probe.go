package sitecheck

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

// ProbeKindHTTP is the built-in HTTP and gRPC check. It is also what an empty
// probe_kind means, so every site configured before probe kinds existed keeps
// its behaviour.
const ProbeKindHTTP = "http"

// Outcomes a probe source reports.
const (
	ProbeUp       = "up"
	ProbeDown     = "down"
	ProbeDegraded = "degraded"
)

// Failure categories of a probe source, reported in SiteInfo.ErrorType next
// to the ones in errorclass.go.
const (
	// ErrorTypeProbe means the probe reported the target down, or could not
	// run at all.
	ErrorTypeProbe = "probe"
	// ErrorTypeDegraded marks a target that answers but reported a problem.
	// The site stays online and it is not an alert failure.
	ErrorTypeDegraded = "degraded"
)

// ErrProbeKindUnavailable is returned for a probe kind no source serves right
// now, for example because its plugin was disabled.
var ErrProbeKindUnavailable = errors.New("probe kind is not available")

// ProbeField is one field of the configuration form of a probe kind. Values
// travel as strings whatever the type.
type ProbeField struct {
	Key string `json:"key"`
	// Type is text (default when empty), textarea, number or bool.
	Type        string `json:"type,omitempty"`
	DisplayName string `json:"display_name"`
	HelpText    string `json:"help_text,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// ProbeKind is a way of checking a site that a source offers next to the
// built-in check.
type ProbeKind struct {
	// Kind is the value stored in model.SiteConfig.ProbeKind.
	Kind string `json:"kind"`
	Name string `json:"name"`
	// PluginID names the plugin behind the kind, when there is one.
	PluginID string       `json:"plugin_id,omitempty"`
	Fields   []ProbeField `json:"fields"`
}

// ProbeRequest is one check a source runs.
type ProbeRequest struct {
	// Target is the site URL, or the custom target URL of the health check.
	Target string
	Config map[string]string
	// Timeout is what the site allows the check to take.
	Timeout time.Duration
}

// ProbeOutcome is the answer of a source.
type ProbeOutcome struct {
	// Status is ProbeUp, ProbeDown or ProbeDegraded.
	Status  string
	Latency time.Duration
	Message string
}

// ProbeSource offers probe kinds that come and go at runtime, such as the
// kinds of probe plugins.
type ProbeSource interface {
	// ProbeKinds lists the kinds the source serves right now.
	ProbeKinds() []ProbeKind
	// Probe checks one target with a kind the source serves. An error means
	// the check could not run, an unhealthy target is a ProbeDown outcome.
	Probe(ctx context.Context, kind string, req ProbeRequest) (ProbeOutcome, error)
}

var (
	probeSources      []ProbeSource
	probeSourcesMutex sync.RWMutex
)

// RegisterProbeSource adds a source of probe kinds.
func RegisterProbeSource(source ProbeSource) {
	probeSourcesMutex.Lock()
	defer probeSourcesMutex.Unlock()
	probeSources = append(probeSources, source)
}

func registeredProbeSources() []ProbeSource {
	probeSourcesMutex.RLock()
	defer probeSourcesMutex.RUnlock()
	return append([]ProbeSource(nil), probeSources...)
}

// probeKindPattern bounds the probe kinds a site may store. Sources name
// their kinds within it, e.g. "plugin:tcp-banner".
var probeKindPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9:_-]{0,63}$`)

// IsBuiltinProbeKind reports whether kind selects the built-in check.
func IsBuiltinProbeKind(kind string) bool {
	return kind == "" || kind == ProbeKindHTTP
}

// IsValidProbeKind reports whether kind is well formed. It does not require a
// source to serve the kind right now: a site keeps its kind while the plugin
// behind it is disabled, and a synchronized node may not run it at all.
func IsValidProbeKind(kind string) bool {
	return IsBuiltinProbeKind(kind) || probeKindPattern.MatchString(kind)
}

// ProbeKinds lists the kinds every source offers, ordered by kind. The
// built-in check is not part of the list.
func ProbeKinds() []ProbeKind {
	kinds := []ProbeKind{}
	seen := map[string]struct{}{}
	for _, source := range registeredProbeSources() {
		for _, kind := range source.ProbeKinds() {
			if IsBuiltinProbeKind(kind.Kind) {
				continue
			}
			if _, dup := seen[kind.Kind]; dup {
				continue
			}
			seen[kind.Kind] = struct{}{}
			if kind.Fields == nil {
				kind.Fields = []ProbeField{}
			}
			kinds = append(kinds, kind)
		}
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i].Kind < kinds[j].Kind })
	return kinds
}

// RunProbe runs one check with the source that serves kind.
func RunProbe(ctx context.Context, kind string, req ProbeRequest) (ProbeOutcome, error) {
	for _, source := range registeredProbeSources() {
		for _, offered := range source.ProbeKinds() {
			if offered.Kind == kind {
				return source.Probe(ctx, kind, req)
			}
		}
	}
	return ProbeOutcome{}, fmt.Errorf("%w: %s", ErrProbeKindUnavailable, kind)
}

// probeResultOf maps what a source answered onto the site status. A check
// that could not run is an error, which the dashboard keeps apart from a
// target that is down.
func probeResultOf(outcome ProbeOutcome, err error) ProbeResult {
	result := ProbeResult{ResponseTime: outcome.Latency.Milliseconds()}
	if err != nil {
		result.Status = StatusError
		result.Error = err.Error()
		result.ErrorType = ErrorTypeProbe
		return result
	}

	message := strings.TrimSpace(outcome.Message)
	switch outcome.Status {
	case ProbeUp:
		result.Status = StatusOnline
	case ProbeDegraded:
		result.Status = StatusOnline
		result.ErrorType = ErrorTypeDegraded
		result.Error = message
		if result.Error == "" {
			result.Error = "the probe reported the target as degraded"
		}
	case ProbeDown:
		result.Status = StatusOffline
		result.ErrorType = ErrorTypeProbe
		result.Error = message
		if result.Error == "" {
			result.Error = "the probe reported the target as down"
		}
	default:
		result.Status = StatusError
		result.ErrorType = ErrorTypeProbe
		result.Error = fmt.Sprintf("the probe answered with the unknown status %q", outcome.Status)
	}
	return result
}

// defaultProbeTimeout applies when a site has no timeout of its own.
const defaultProbeTimeout = 10 * time.Second

// probeWithKind checks a site with a probe kind a source serves.
func probeWithKind(ctx context.Context, siteURL string, config *model.SiteConfig) ProbeResult {
	timeout := defaultProbeTimeout
	if config.Timeout > 0 {
		timeout = time.Duration(config.Timeout) * time.Second
	}
	outcome, err := RunProbe(ctx, config.ProbeKind, ProbeRequest{
		Target:  effectiveHealthCheckURL(siteURL, config.HealthCheckConfig),
		Config:  config.ProbeConfig,
		Timeout: timeout,
	})
	return probeResultOf(outcome, err)
}

// TestProbe runs one check with a probe kind without saving anything, for
// the health check test endpoint.
func TestProbe(ctx context.Context, kind, target string, config map[string]string, timeout time.Duration) ProbeResult {
	if timeout <= 0 {
		timeout = defaultProbeTimeout
	}
	outcome, err := RunProbe(ctx, kind, ProbeRequest{Target: target, Config: config, Timeout: timeout})
	return probeResultOf(outcome, err)
}
