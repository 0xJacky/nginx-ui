package capability

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/sitecheck"
)

const probePluginID = "io.github.example.probe"

func probeHost(caller *fakeCaller) *capabilityHost {
	h := newCapabilityHost()
	h.callers[probePluginID] = caller
	h.kinds = []plugin.ProbeKindEntry{{PluginID: probePluginID, Kind: protocol.ProbeKind{
		Code: "tcp-banner",
		Name: "TCP banner",
		Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
			{Key: "port", Type: protocol.ConfigurationFieldNumber, DisplayName: "Port", Required: true},
		}},
	}}}
	h.own(protocol.CapabilityProbe, "tcp-banner", probePluginID)
	return h
}

func TestProbeKindsListTheOwnedCodes(t *testing.T) {
	kinds := NewProbeSource(probeHost(newFakeCaller())).ProbeKinds()
	if len(kinds) != 1 || kinds[0].Kind != "plugin:tcp-banner" || kinds[0].PluginID != probePluginID {
		t.Fatalf("kinds = %+v", kinds)
	}
	if len(kinds[0].Fields) != 1 || kinds[0].Fields[0].Type != protocol.ConfigurationFieldNumber {
		t.Fatalf("fields = %+v", kinds[0].Fields)
	}
}

func TestProbeCallsThePlugin(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodProbeCheck] = protocol.ProbeCheckResult{
		Status: protocol.ProbeStatusDown, LatencyMS: 1500, Message: "connection refused",
	}
	host := probeHost(caller)

	outcome, err := NewProbeSource(host).Probe(t.Context(), "plugin:tcp-banner", sitecheck.ProbeRequest{
		Target:  "https://example.com",
		Config:  map[string]string{"port": "22"},
		Timeout: 2500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if outcome.Status != sitecheck.ProbeDown || outcome.Latency != 1500*time.Millisecond || outcome.Message != "connection refused" {
		t.Fatalf("outcome = %+v", outcome)
	}

	calls := caller.methodCalls(protocol.MethodProbeCheck)
	if len(calls) != 1 {
		t.Fatalf("probe.check calls = %d", len(calls))
	}
	var params protocol.ProbeCheckParams
	if err = json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Kind != "tcp-banner" || params.Target != "https://example.com" || params.Config["port"] != "22" || params.TimeoutSeconds != 3 {
		t.Fatalf("params = %+v", params)
	}
	if host.releaseCount() != 1 {
		t.Fatalf("releases = %d, want 1", host.releaseCount())
	}
}

func TestProbeFailures(t *testing.T) {
	caller := newFakeCaller()
	source := NewProbeSource(probeHost(caller))

	if _, err := source.Probe(t.Context(), "plugin:unknown", sitecheck.ProbeRequest{}); !errors.Is(err, sitecheck.ErrProbeKindUnavailable) {
		t.Fatalf("err = %v, want ErrProbeKindUnavailable", err)
	}
	if _, err := source.Probe(t.Context(), sitecheck.ProbeKindHTTP, sitecheck.ProbeRequest{}); !errors.Is(err, sitecheck.ErrProbeKindUnavailable) {
		t.Fatalf("err = %v, want ErrProbeKindUnavailable", err)
	}

	caller.errs[protocol.MethodProbeCheck] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "port must be a number", Data: map[string]any{"field": "port"},
	}
	_, err := source.Probe(t.Context(), "plugin:tcp-banner", sitecheck.ProbeRequest{})
	if err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("err = %v, want the invalid field", err)
	}
}

func TestProbeSourceThroughTheRegistry(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodProbeCheck] = protocol.ProbeCheckResult{Status: protocol.ProbeStatusUp, LatencyMS: 8}
	sitecheck.RegisterProbeSource(NewProbeSource(probeHost(caller)))

	result := sitecheck.TestProbe(t.Context(), "plugin:tcp-banner", "https://example.com", nil, time.Second)
	if result.Status != sitecheck.StatusOnline || result.ResponseTime != 8 {
		t.Fatalf("result = %+v", result)
	}
}
