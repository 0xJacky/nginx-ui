package sitecheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/model"
)

// fakeProbeSource answers every check with a fixed outcome.
type fakeProbeSource struct {
	kinds   []ProbeKind
	outcome ProbeOutcome
	err     error

	mu       sync.Mutex
	requests []ProbeRequest
	kindsRun []string
}

func (s *fakeProbeSource) ProbeKinds() []ProbeKind { return s.kinds }

func (s *fakeProbeSource) Probe(_ context.Context, kind string, req ProbeRequest) (ProbeOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kindsRun = append(s.kindsRun, kind)
	s.requests = append(s.requests, req)
	return s.outcome, s.err
}

// withProbeSource registers a source for one test only.
func withProbeSource(t *testing.T, source ProbeSource) {
	t.Helper()
	probeSourcesMutex.Lock()
	previous := probeSources
	probeSources = append(append([]ProbeSource(nil), previous...), source)
	probeSourcesMutex.Unlock()
	t.Cleanup(func() {
		probeSourcesMutex.Lock()
		probeSources = previous
		probeSourcesMutex.Unlock()
	})
}

func TestProbeResultMapping(t *testing.T) {
	tests := []struct {
		name      string
		outcome   ProbeOutcome
		err       error
		status    string
		errorType string
		message   string
	}{
		{"up", ProbeOutcome{Status: ProbeUp, Latency: 42 * time.Millisecond}, nil, StatusOnline, "", ""},
		{"degraded", ProbeOutcome{Status: ProbeDegraded, Message: "slow"}, nil, StatusOnline, ErrorTypeDegraded, "slow"},
		{"down", ProbeOutcome{Status: ProbeDown, Message: "refused"}, nil, StatusOffline, ErrorTypeProbe, "refused"},
		{"down without message", ProbeOutcome{Status: ProbeDown}, nil, StatusOffline, ErrorTypeProbe, "the probe reported the target as down"},
		{"unknown status", ProbeOutcome{Status: "sideways"}, nil, StatusError, ErrorTypeProbe, `the probe answered with the unknown status "sideways"`},
		{"failed call", ProbeOutcome{}, errors.New("plugin is not running"), StatusError, ErrorTypeProbe, "plugin is not running"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := probeResultOf(tc.outcome, tc.err)
			if got.Status != tc.status || got.ErrorType != tc.errorType || got.Error != tc.message {
				t.Fatalf("result = %+v, want status %s, type %q, error %q", got, tc.status, tc.errorType, tc.message)
			}
		})
	}
	if got := probeResultOf(ProbeOutcome{Status: ProbeUp, Latency: 42 * time.Millisecond}, nil); got.ResponseTime != 42 {
		t.Fatalf("response time = %d, want 42", got.ResponseTime)
	}
}

func TestProbeKindsAndRunProbe(t *testing.T) {
	source := &fakeProbeSource{
		kinds: []ProbeKind{
			{Kind: "plugin:tcp", Name: "TCP"},
			{Kind: ProbeKindHTTP, Name: "shadowed"},
			{Kind: "plugin:db", Name: "DB"},
		},
		outcome: ProbeOutcome{Status: ProbeUp},
	}
	withProbeSource(t, source)

	kinds := ProbeKinds()
	if len(kinds) != 2 || kinds[0].Kind != "plugin:db" || kinds[1].Kind != "plugin:tcp" || kinds[0].Fields == nil {
		t.Fatalf("kinds = %+v", kinds)
	}

	if _, err := RunProbe(t.Context(), "plugin:tcp", ProbeRequest{Target: "https://example.com"}); err != nil {
		t.Fatalf("RunProbe: %v", err)
	}
	if _, err := RunProbe(t.Context(), "plugin:gone", ProbeRequest{}); !errors.Is(err, ErrProbeKindUnavailable) {
		t.Fatalf("err = %v, want ErrProbeKindUnavailable", err)
	}
}

func TestCheckSiteUsesThePluginProbeKind(t *testing.T) {
	t.Cleanup(InvalidateSiteConfigCache)
	source := &fakeProbeSource{
		kinds:   []ProbeKind{{Kind: "plugin:tcp", Name: "TCP"}},
		outcome: ProbeOutcome{Status: ProbeDegraded, Latency: 15 * time.Millisecond, Message: "banner is slow"},
	}
	withProbeSource(t, source)

	checker := NewSiteChecker(DefaultCheckOptions())
	checker.client = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		t.Fatalf("unexpected HTTP request to %s while a probe kind is selected", req.URL)
		return nil, nil
	})}
	checker.enhanced.defaultClient = checker.client

	const siteURL = "https://probe.example.com"
	setCachedSiteConfig(canonicalSiteKey("", siteURL), &model.SiteConfig{
		Model:              model.Model{ID: 7},
		Host:               "probe.example.com:443",
		Scheme:             "https",
		DisplayURL:         siteURL,
		HealthCheckEnabled: true,
		Timeout:            3,
		HealthCheckConfig:  &model.HealthCheckConfig{Protocol: "https", TargetURL: "https://10.0.0.1:8443"},
		ProbeKind:          "plugin:tcp",
		ProbeConfig:        map[string]string{"port": "22"},
	})

	info, err := checker.CheckSite(t.Context(), siteURL)
	if err != nil {
		t.Fatalf("CheckSite: %v", err)
	}
	if info.Status != StatusOnline || info.ErrorType != ErrorTypeDegraded || info.Error != "banner is slow" || info.ResponseTime != 15 {
		t.Fatalf("info = %+v", info)
	}
	if info.ProbeKind != "plugin:tcp" {
		t.Fatalf("the site info lost its probe kind: %+v", info.SiteConfig)
	}

	if len(source.requests) != 1 {
		t.Fatalf("probe calls = %d, want 1", len(source.requests))
	}
	req := source.requests[0]
	if req.Target != "https://10.0.0.1:8443" || req.Config["port"] != "22" || req.Timeout != 3*time.Second {
		t.Fatalf("request = %+v", req)
	}
}

func TestDegradedProbeIsNotAnAlertFailure(t *testing.T) {
	policy := &model.SiteHealthAlertConfig{Enabled: true, NetworkErrors: true}
	degraded := &SiteInfo{Status: StatusOnline, Error: "slow", ErrorType: ErrorTypeDegraded}
	if isAlertFailure(policy, degraded) {
		t.Fatal("a degraded probe result counted as a failure")
	}
	down := &SiteInfo{Status: StatusOffline, Error: "refused", ErrorType: ErrorTypeProbe}
	if !isAlertFailure(policy, down) {
		t.Fatal("a probe reporting the target down is a failure")
	}
}

func TestTestProbeReportsAnUnavailableKind(t *testing.T) {
	result := TestProbe(t.Context(), "plugin:gone", "https://example.com", nil, 0)
	if result.Status != StatusError || result.ErrorType != ErrorTypeProbe {
		t.Fatalf("result = %+v", result)
	}
}

func TestSiteInfoJSONLeavesTheProbeValuesOut(t *testing.T) {
	info := &SiteInfo{SiteConfig: model.SiteConfig{
		ProbeKind:   "plugin:db",
		ProbeConfig: map[string]string{"password": "hunter2"},
	}, Status: StatusOnline}

	encoded, err := json.Marshal([]*SiteInfo{info})
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	if strings.Contains(text, "hunter2") || strings.Contains(text, "probe_config") {
		t.Fatalf("site info leaks the probe values: %s", text)
	}
	if !strings.Contains(text, `"probe_kind":"plugin:db"`) || !strings.Contains(text, `"status":"online"`) {
		t.Fatalf("site info lost other fields: %s", text)
	}
	if info.ProbeConfig["password"] != "hunter2" {
		t.Fatal("marshaling changed the site info itself")
	}
}
