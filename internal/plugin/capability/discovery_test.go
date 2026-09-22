package capability

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/upstream/discovery"
)

const discoveryPluginID = "io.github.example.registry"

func discoveryHost(caller *fakeCaller) *capabilityHost {
	h := newCapabilityHost()
	h.callers[discoveryPluginID] = caller
	h.providers = []plugin.DiscoveryProviderEntry{
		{PluginID: discoveryPluginID, Provider: protocol.DiscoveryProvider{
			Code: "registry",
			Name: "Service registry",
			Configuration: &protocol.ConfigurationSchema{Fields: []protocol.ConfigurationField{
				{Key: "address", DisplayName: "Address", Required: true},
				{Key: "token", DisplayName: "Token", Secret: true},
			}},
		}},
		{PluginID: "io.github.other.registry", Provider: protocol.DiscoveryProvider{Code: "registry", Name: "Shadowed"}},
	}
	h.own(protocol.CapabilityUpstreamDiscovery, "registry", discoveryPluginID)
	return h
}

func TestDiscoveryProvidersListTheOwnedCodes(t *testing.T) {
	providers := NewDiscoverySource(discoveryHost(newFakeCaller())).Providers()
	if len(providers) != 1 {
		t.Fatalf("providers = %+v", providers)
	}
	provider := providers[0]
	if provider.Kind != "plugin:registry" || provider.Name != "Service registry" || provider.PluginID != discoveryPluginID {
		t.Fatalf("provider = %+v", provider)
	}
	if len(provider.Fields) != 2 || !provider.Fields[0].Required || !provider.Fields[1].Secret {
		t.Fatalf("fields = %+v", provider.Fields)
	}
}

func TestDiscoveryResolveCallsThePlugin(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDiscoveryResolve] = protocol.DiscoveryResolveResult{
		Targets: []protocol.DiscoveryTarget{
			{Address: "10.0.1.12", Port: 8080, Weight: 2, Tags: []string{"zone-a"}},
			{Address: "2001:db8::17", Port: 8443},
		},
		TTLSeconds: 30,
	}
	host := discoveryHost(caller)
	source := NewDiscoverySource(host)

	result, err := source.Resolve(t.Context(), "plugin:registry", map[string]string{"address": "https://registry"}, "api")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(result.Targets) != 2 || result.Targets[0].Weight != 2 || result.Targets[0].Tags[0] != "zone-a" ||
		result.Targets[1].Port != 8443 || result.TTLSeconds != 30 {
		t.Fatalf("result = %+v", result)
	}

	calls := caller.methodCalls(protocol.MethodDiscoveryResolve)
	if len(calls) != 1 {
		t.Fatalf("discovery.resolve calls = %d", len(calls))
	}
	var params protocol.DiscoveryResolveParams
	if err = json.Unmarshal(calls[0].Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Provider != "registry" || params.Service != "api" || params.Config["address"] != "https://registry" {
		t.Fatalf("params = %+v", params)
	}
	if host.releaseCount() != 1 {
		t.Fatalf("releases = %d, want 1", host.releaseCount())
	}
}

func TestDiscoveryResolveMapsTheErrors(t *testing.T) {
	caller := newFakeCaller()
	source := NewDiscoverySource(discoveryHost(caller))

	caller.errs[protocol.MethodDiscoveryResolve] = &protocol.Error{
		Code: protocol.CodeInvalidConfig, Message: "unknown service: billing", Data: map[string]any{"field": "service"},
	}
	_, err := source.Resolve(t.Context(), "plugin:registry", nil, "billing")
	assertCosyCode(t, err, plugin.ErrDiscoveryConfigInvalid)

	caller.errs[protocol.MethodDiscoveryResolve] = &protocol.Error{Code: protocol.CodeInternalError, Message: "registry is down"}
	_, err = source.Resolve(t.Context(), "plugin:registry", nil, "api")
	if !errors.Is(err, plugin.ErrRPC) {
		t.Fatalf("err = %v, want the plugin rpc error", err)
	}

	for _, kind := range []string{"plugin:unknown", "registry"} {
		_, err = source.Resolve(t.Context(), kind, nil, "api")
		assertCosyCode(t, err, plugin.ErrDiscoveryKindUnavailable)
	}
}

func TestDiscoverySourceThroughTheRegistry(t *testing.T) {
	caller := newFakeCaller()
	caller.results[protocol.MethodDiscoveryResolve] = protocol.DiscoveryResolveResult{
		Targets: []protocol.DiscoveryTarget{{Address: "10.0.0.5", Port: 80}},
	}
	discovery.RegisterSource(NewDiscoverySource(discoveryHost(caller)))

	assertCosyCode(t, discovery.ValidateConfig("plugin:registry", map[string]string{}), plugin.ErrDiscoveryConfigInvalid)
	if err := discovery.ValidateConfig("plugin:registry", map[string]string{"address": "a"}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	result, err := discovery.Resolve(t.Context(), "plugin:registry", map[string]string{"address": "a"}, "api")
	if err != nil || len(result.Targets) != 1 {
		t.Fatalf("resolve = %+v, %v", result, err)
	}
}
