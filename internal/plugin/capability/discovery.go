package capability

import (
	"context"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/plugin"
	"github.com/0xJacky/Nginx-UI/internal/plugin/jsonrpc"
	"github.com/0xJacky/Nginx-UI/internal/plugin/protocol"
	"github.com/0xJacky/Nginx-UI/internal/upstream/discovery"
	"github.com/uozi-tech/cosy"
)

// discoveryResolveTimeout bounds one resolution, including starting an
// on_demand plugin.
const discoveryResolveTimeout = 30 * time.Second

// DiscoveryHost is the part of the plugin manager the upstream.discovery
// capability needs.
type DiscoveryHost interface {
	// OwnerOf returns the plugin that serves a provider code.
	OwnerOf(capability, code string) (pluginID string, ok bool)
	// Acquire returns a client for a plugin and a func that releases it,
	// starting an on_demand plugin. The release func is safe to call even
	// when the error is not nil.
	Acquire(ctx context.Context, pluginID string) (jsonrpc.Caller, func(), error)
	// DiscoveryProviders lists what every enabled upstream.discovery plugin
	// offers.
	DiscoveryProviders() []plugin.DiscoveryProviderEntry
}

// RegisterDiscovery offers the providers of every enabled upstream.discovery
// plugin to the upstream discoveries.
func RegisterDiscovery(h DiscoveryHost) {
	discovery.RegisterSource(NewDiscoverySource(h))
}

// NewDiscoverySource exposes the upstream.discovery capability of the
// plugins of h as a provider registry. A provider code is published as
// PluginType(code).
func NewDiscoverySource(h DiscoveryHost) discovery.Source {
	return &discoverySource{host: h}
}

type discoverySource struct {
	host DiscoveryHost
}

// Providers lists every provider once, served by the plugin that owns its
// code.
func (s *discoverySource) Providers() []discovery.Provider {
	entries := s.host.DiscoveryProviders()
	providers := make([]discovery.Provider, 0, len(entries))
	for _, entry := range entries {
		if owner, ok := s.host.OwnerOf(protocol.CapabilityUpstreamDiscovery, entry.Provider.Code); !ok || owner != entry.PluginID {
			continue
		}
		fields := configurationFields(entry.Provider.Configuration)
		provider := discovery.Provider{
			Kind:     PluginType(entry.Provider.Code),
			Name:     entry.Provider.Name,
			PluginID: entry.PluginID,
			Fields:   make([]discovery.ProviderField, 0, len(fields)),
		}
		for _, field := range fields {
			provider.Fields = append(provider.Fields, discovery.ProviderField{
				Key:         field.Key,
				Type:        field.Type,
				DisplayName: field.DisplayName,
				HelpText:    field.HelpText,
				Required:    field.Required,
				Secret:      field.Secret,
			})
		}
		providers = append(providers, provider)
	}
	return providers
}

// Resolve runs discovery.resolve on the plugin that owns the provider right
// now.
func (s *discoverySource) Resolve(ctx context.Context, kind string, config map[string]string, service string) (discovery.Result, error) {
	code, ok := pluginCode(kind)
	if !ok {
		return discovery.Result{}, cosy.WrapErrorWithParams(plugin.ErrDiscoveryKindUnavailable, kind)
	}
	pluginID, ok := s.host.OwnerOf(protocol.CapabilityUpstreamDiscovery, code)
	if !ok {
		return discovery.Result{}, cosy.WrapErrorWithParams(plugin.ErrDiscoveryKindUnavailable, kind)
	}

	callCtx, cancel := context.WithTimeout(ctx, discoveryResolveTimeout)
	defer cancel()

	caller, release, err := s.host.Acquire(callCtx, pluginID)
	if release != nil {
		defer release()
	}
	if err != nil {
		return discovery.Result{}, plugin.WrapRPCError(err)
	}

	var reply protocol.DiscoveryResolveResult
	err = caller.Call(callCtx, protocol.MethodDiscoveryResolve, protocol.DiscoveryResolveParams{
		Provider: code,
		Config:   config,
		Service:  service,
	}, &reply)
	if err != nil {
		if field, message, invalid := invalidConfigField(err); invalid {
			return discovery.Result{}, cosy.WrapErrorWithParams(plugin.ErrDiscoveryConfigInvalid, field, message)
		}
		return discovery.Result{}, plugin.WrapRPCError(err)
	}

	result := discovery.Result{Targets: make([]discovery.Target, 0, len(reply.Targets)), TTLSeconds: reply.TTLSeconds}
	for _, target := range reply.Targets {
		result.Targets = append(result.Targets, discovery.Target{
			Address: target.Address,
			Port:    target.Port,
			Weight:  target.Weight,
			Tags:    target.Tags,
		})
	}
	return result, nil
}
