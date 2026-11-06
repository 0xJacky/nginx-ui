package capability

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/security/blocklist"
)

const blocklistPluginID = "io.github.example.threatfeed"

func blocklistHost(caller *fakeCaller) *capabilityHost {
	h := newCapabilityHost()
	h.callers[blocklistPluginID] = caller
	h.sources = []plugin.BlocklistSourceEntry{
		{PluginID: blocklistPluginID, Source: protocol.BlocklistSource{
			Code:           "threatfeed",
			Name:           "ThreatFeed",
			RefreshSeconds: 900,
			Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
				{Key: "api_key", DisplayName: "API key", Required: true, Secret: true},
			}},
		}},
		{PluginID: blocklistPluginID, Source: protocol.BlocklistSource{Code: "public-list", Name: "Public list"}},
		// A second plugin declaring the same code does not own it.
		{PluginID: "io.github.other.feed", Source: protocol.BlocklistSource{Code: "threatfeed", Name: "Shadowed"}},
	}
	h.own(protocol.CapabilitySecurityBlocklist, "threatfeed", blocklistPluginID)
	h.own(protocol.CapabilitySecurityBlocklist, "public-list", blocklistPluginID)
	return h
}

func TestBlocklistKindsListTheOwnedCodes(t *testing.T) {
	kinds := NewBlocklistSource(blocklistHost(newFakeCaller())).Kinds()
	if len(kinds) != 2 {
		t.Fatalf("kinds = %+v", kinds)
	}
	feed := kinds[0]
	if feed.Kind != "plugin:threatfeed" || feed.Name != "ThreatFeed" || feed.PluginID != blocklistPluginID || feed.RefreshSeconds != 900 {
		t.Fatalf("kind = %+v", feed)
	}
	if len(feed.Fields) != 1 || !feed.Fields[0].Required || !feed.Fields[0].Secret {
		t.Fatalf("fields = %+v", feed.Fields)
	}
	if kinds[1].RefreshSeconds != protocol.DefaultBlocklistRefreshSeconds {
		t.Fatalf("a kind without an interval gets the default, got %d", kinds[1].RefreshSeconds)
	}
}

func TestBlocklistFetchCallsThePlugin(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodBlocklistFetch] = protocol.BlocklistFetchResult{
		Entries:    []protocol.BlocklistEntry{{CIDR: "198.51.100.23", Reason: "SSH brute force"}, {CIDR: "2001:db8::/32"}},
		TTLSeconds: 120,
	}
	host := blocklistHost(caller)
	source := NewBlocklistSource(host)

	result, err := source.Fetch(t.Context(), "plugin:threatfeed", map[string]string{"api_key": "k"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(result.Entries) != 2 || result.Entries[0].CIDR != "198.51.100.23" || result.Entries[0].Reason != "SSH brute force" || result.TTLSeconds != 120 {
		t.Fatalf("result = %+v", result)
	}

	calls := caller.methodCalls(protocol.MethodBlocklistFetch)
	if len(calls) != 1 {
		t.Fatalf("blocklist.fetch calls = %d", len(calls))
	}
	var params protocol.BlocklistFetchParams
	if err = json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Source != "threatfeed" || params.Config["api_key"] != "k" {
		t.Fatalf("params = %+v", params)
	}
	if host.releaseCount() != 1 {
		t.Fatalf("releases = %d, want 1", host.releaseCount())
	}
}

func TestBlocklistFetchMapsTheErrors(t *testing.T) {
	caller := newFakeCaller()
	host := blocklistHost(caller)
	source := NewBlocklistSource(host)

	caller.errs[protocol.MethodBlocklistFetch] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "api_key was rejected", Data: map[string]any{"field": "api_key"},
	}
	_, err := source.Fetch(t.Context(), "plugin:threatfeed", nil)
	assertCosyCode(t, err, plugin.ErrBlocklistConfigInvalid)

	caller.errs[protocol.MethodBlocklistFetch] = &protocol.Error{Code: protocol.CodeInternalError, Message: "feed is down"}
	_, err = source.Fetch(t.Context(), "plugin:threatfeed", nil)
	if !errors.Is(err, plugin.ErrRPC) {
		t.Fatalf("err = %v, want the plugin rpc error", err)
	}

	for _, kind := range []string{"plugin:unknown", "threatfeed"} {
		_, err = source.Fetch(t.Context(), kind, nil)
		assertCosyCode(t, err, plugin.ErrBlocklistKindUnavailable)
	}

	host.acquireErr = errors.New("cannot start")
	if _, err = source.Fetch(t.Context(), "plugin:threatfeed", nil); err == nil {
		t.Fatal("an unreachable plugin must fail the fetch")
	}
}

func TestBlocklistSourceThroughTheRegistry(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodBlocklistFetch] = protocol.BlocklistFetchResult{Entries: []protocol.BlocklistEntry{{CIDR: "203.0.113.0/24"}}}
	blocklist.RegisterSource(NewBlocklistSource(blocklistHost(caller)))

	kind, ok := blocklist.KindOf("plugin:threatfeed")
	if !ok || kind.RefreshSeconds != 900 {
		t.Fatalf("kind = %+v, %v", kind, ok)
	}

	// Required fields are checked before anything is stored.
	assertCosyCode(t, blocklist.ValidateConfig("plugin:threatfeed", map[string]string{}), plugin.ErrBlocklistConfigInvalid)
	if err := blocklist.ValidateConfig("plugin:threatfeed", map[string]string{"api_key": "k"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	assertCosyCode(t, blocklist.ValidateConfig("plugin:gone", nil), plugin.ErrBlocklistKindUnavailable)

	result, err := blocklist.Fetch(t.Context(), "plugin:threatfeed", map[string]string{"api_key": "k"})
	if err != nil || len(result.Entries) != 1 {
		t.Fatalf("fetch = %+v, %v", result, err)
	}
}
